#include <WiFi.h>
#include <WebServer.h>
#include <ESPmDNS.h>
#include <ESP_I2S.h>
#include "secrets.h"

// ---- Pin assignments ----
#define I2S_BCLK 15
#define I2S_WS 16
#define I2S_DOUT 7
#define SD_PIN 6 // amp hardware mute; set to -1 if SD is left unconnected
#define BOOT_PIN 0

#define SAMPLE_RATE 22050
#define BUFFER_SAMPLES 256
#define FADE_MS 300.0f

// Speech audio is streamed into a small, fixed-size ring buffer as it
// arrives over HTTP and drained by audioTask in real time -- it is never
// buffered whole. An earlier malloc-the-whole-clip design ran into this
// chip's real-world heap fragmentation hard: ESP.getMaxAllocHeap() dropped
// to well under 100KB after only a few requests regardless of how it was
// sequenced, making anything beyond a couple of seconds of speech
// unreliable. A small *static* ring (never resized, never fragments)
// sidesteps the problem for clips of any length.
#define SPEAK_RING_BYTES 16384 // ~370ms of buffering at 22050Hz, 16-bit
#define SPEAK_MAX_BYTES 882000 // sanity/DoS cap only (20s of audio) -- not a buffer size

static uint8_t speakRing[SPEAK_RING_BYTES];
volatile size_t speakRingHead = 0; // next index the network (producer) writes
volatile size_t speakRingTail = 0; // next index audioTask (consumer) reads
volatile bool speakStreamAuthorized = false;
volatile size_t speakStreamReceived = 0;

volatile bool isOn = true;
volatile float currentVolume = 0.3f;

static inline size_t speakRingFree() {
  size_t used = (speakRingHead - speakRingTail + SPEAK_RING_BYTES) % SPEAK_RING_BYTES;
  return SPEAK_RING_BYTES - 1 - used; // one slot always kept empty to tell full apart from empty
}

I2SClass i2s;
WebServer server(80);
TaskHandle_t audioTaskHandle = nullptr;

// Pulls the next sample out of speakRing, or silence if nothing's buffered
// (idle between clips, or an underrun).
float nextSample() {
  if (speakRingTail == speakRingHead) return 0.0f;
  uint8_t lo = speakRing[speakRingTail];
  speakRingTail = (speakRingTail + 1) % SPEAK_RING_BYTES;
  if (speakRingTail == speakRingHead) return 0.0f; // odd byte stranded at a chunk boundary
  uint8_t hi = speakRing[speakRingTail];
  speakRingTail = (speakRingTail + 1) % SPEAK_RING_BYTES;
  int16_t sample = (int16_t)((hi << 8) | lo); // little-endian
  return sample / 32768.0f;
}

// ---- Audio task ----

void setOn() {
  isOn = true;
#if SD_PIN >= 0
  digitalWrite(SD_PIN, HIGH);
#endif
}

void setOff() {
  isOn = false;
}

void audioTask(void *param) {
  (void)param;
  static int16_t buf[BUFFER_SAMPLES];
  float envGain = 0.0f;
  const float envStep = 1.0f / (FADE_MS / 1000.0f * SAMPLE_RATE);

  for (;;) {
    float target = currentVolume;

    for (int i = 0; i < BUFFER_SAMPLES; i++) {
      float raw = nextSample();
      if (envGain < target) envGain = min(target, envGain + envStep);
      else if (envGain > target) envGain = max(target, envGain - envStep);

      float s = raw * envGain;
      if (s > 1.0f) s = 1.0f;
      if (s < -1.0f) s = -1.0f;
      buf[i] = (int16_t)(s * 32000.0f);
    }

    i2s.write((uint8_t *)buf, sizeof(buf));

#if SD_PIN >= 0
    if (!isOn && envGain <= 0.0005f) {
      digitalWrite(SD_PIN, LOW);
    }
#endif
  }
}

// ---- HTTP API ----

bool isAuthorized() {
  if (strlen(API_TOKEN) == 0) return true;
  String token = server.hasArg("token") ? server.arg("token") : server.header("X-Token");
  return token == API_TOKEN;
}

bool checkAuth() {
  if (isAuthorized()) return true;
  server.send(401, "application/json", "{\"error\":\"unauthorized\"}");
  return false;
}

String buildStatusJson() {
  char buf[256];
  snprintf(buf, sizeof(buf),
           "{\"on\":%s,\"volume\":%.2f,\"ip\":\"%s\",\"rssi\":%d,\"uptime_s\":%lu,"
           "\"free_heap\":%u,\"max_alloc_heap\":%u}",
           isOn ? "true" : "false",
           currentVolume,
           WiFi.localIP().toString().c_str(),
           (int)WiFi.RSSI(),
           (unsigned long)(millis() / 1000),
           (unsigned)ESP.getFreeHeap(),
           (unsigned)ESP.getMaxAllocHeap());
  return String(buf);
}

void handleStatus() {
  if (!checkAuth()) return;
  server.send(200, "application/json", buildStatusJson());
}

void handleOn() {
  if (!checkAuth()) return;
  setOn();
  server.send(200, "application/json", buildStatusJson());
}

void handleOff() {
  if (!checkAuth()) return;
  setOff();
  server.send(200, "application/json", buildStatusJson());
}

void handleToggle() {
  if (!checkAuth()) return;
  if (isOn) setOff(); else setOn();
  server.send(200, "application/json", buildStatusJson());
}

void handleVolume() {
  if (!checkAuth()) return;
  if (!server.hasArg("level")) {
    server.send(400, "application/json", "{\"error\":\"missing level\"}");
    return;
  }
  float level = server.arg("level").toFloat();
  if (level < 0.0f || level > 1.0f) {
    server.send(400, "application/json", "{\"error\":\"level out of range\"}");
    return;
  }
  currentVolume = level;
  server.send(200, "application/json", buildStatusJson());
}

// gear-box (the TTS service running on the bw-server) synthesizes speech
// itself and pushes the finished audio here directly — raw PCM, mono,
// 16-bit signed little-endian, 22050Hz, no WAV header, no length prefix
// beyond the HTTP Content-Length header itself. This device no longer
// fetches anything; it only ever receives.
//
// WebServer's plain-body path (server.arg("plain")) is not usable here: it
// buffers the whole body through a C-string (String(plainBuf)), which
// truncates at the first zero byte — and raw 16-bit PCM is full of zero
// bytes. The raw-body callback mechanism below (registered as the upload
// function in server.on(), delivered via server.raw()) is what this core's
// WebServer actually uses to hand a non-form POST body to a sketch without
// a String round-trip.
//
// The body is streamed straight into speakRing as it arrives rather than
// buffered whole: an earlier malloc-one-big-buffer design hit this chip's
// real-world heap fragmentation hard (max contiguous allocation fell under
// 100KB after only a handful of requests, however carefully the old and
// new buffers were sequenced), making more than a couple of seconds of
// speech unreliable. Streaming into a small static ring removes that
// class of failure entirely, for a clip of any length.
void handleSpeakRaw() {
  HTTPRaw &raw = server.raw();
  switch (raw.status) {
    case RAW_START: {
      speakStreamReceived = 0;
      // Headers (and any ?token= query arg) are already fully parsed by the
      // time RAW_START fires, so this check still runs before any body
      // bytes are retained or played.
      speakStreamAuthorized = isAuthorized();
      if (!speakStreamAuthorized) break;

      speakRingHead = 0;
      speakRingTail = 0;
      break;
    }

    case RAW_WRITE: {
      if (!speakStreamAuthorized) break;
      if (speakStreamReceived >= SPEAK_MAX_BYTES) break; // sanity cap hit; drop the rest silently

      for (size_t i = 0; i < raw.currentSize; i++) {
        unsigned long waitStart = millis();
        while (speakRingFree() == 0) {
          // audioTask (the other core) is draining in real time, so this
          // only blocks when the network is delivering faster than
          // playback consumes -- expected and brief, not a stall.
          if (millis() - waitStart > 2000) return; // consumer wedged; bail on this chunk
          delay(1);
        }
        speakRing[speakRingHead] = raw.buf[i];
        speakRingHead = (speakRingHead + 1) % SPEAK_RING_BYTES;
      }
      speakStreamReceived += raw.currentSize;
      break;
    }

    case RAW_END:
    case RAW_ABORTED:
    default:
      break; // response sent from handleSpeakPost, which runs right after this
  }
}

void handleSpeakPost() {
  if (!speakStreamAuthorized) {
    server.send(401, "application/json", "{\"error\":\"unauthorized\"}");
    return;
  }
  server.send(200, "application/json", buildStatusJson());
}

void handleSpeakGetNotAllowed() {
  server.send(405, "application/json", "{\"error\":\"POST only\"}");
}

void handleNotFound() {
  server.send(404, "application/json", "{\"error\":\"not found\"}");
}

// ---- Setup / loop ----

void setup() {
  Serial.begin(115200);
  delay(200);

#if SD_PIN >= 0
  pinMode(SD_PIN, OUTPUT);
  digitalWrite(SD_PIN, HIGH);
#endif
  pinMode(BOOT_PIN, INPUT_PULLUP);

  i2s.setPins(I2S_BCLK, I2S_WS, I2S_DOUT, -1, -1);
  if (!i2s.begin(I2S_MODE_STD, SAMPLE_RATE, I2S_DATA_BIT_WIDTH_16BIT, I2S_SLOT_MODE_MONO)) {
    Serial.println("I2S init failed");
  }

  xTaskCreatePinnedToCore(audioTask, "audio", 4096, nullptr, 5, &audioTaskHandle, 0);

  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASS);
  Serial.print("Connecting to WiFi");
  unsigned long start = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - start < 20000) {
    delay(300);
    Serial.print(".");
  }
  Serial.println();
  if (WiFi.status() == WL_CONNECTED) {
    Serial.print("WiFi connected, IP: ");
    Serial.println(WiFi.localIP());
  } else {
    Serial.println("WiFi not connected yet, will keep retrying in loop()");
  }

  if (MDNS.begin("soundmachine")) {
    MDNS.addService("http", "tcp", 80);
    Serial.println("mDNS: soundmachine.local");
  } else {
    Serial.println("mDNS init failed");
  }

  static const char *headerKeys[] = { "X-Token" };
  server.collectHeaders(headerKeys, 1);

  server.on("/status", HTTP_ANY, handleStatus);
  server.on("/on", HTTP_ANY, handleOn);
  server.on("/off", HTTP_ANY, handleOff);
  server.on("/toggle", HTTP_ANY, handleToggle);
  server.on("/volume", HTTP_ANY, handleVolume);
  server.on("/speak", HTTP_POST, handleSpeakPost, handleSpeakRaw);
  server.on("/speak", HTTP_GET, handleSpeakGetNotAllowed);
  server.onNotFound(handleNotFound);
  server.begin();

  Serial.println("HTTP server started");
  Serial.println("Default state: ON");
}

void loop() {
  server.handleClient();

  static unsigned long lastWifiCheck = 0;
  if (millis() - lastWifiCheck > 15000) {
    lastWifiCheck = millis();
    wl_status_t st = WiFi.status();
    if (st == WL_DISCONNECTED || st == WL_CONNECTION_LOST || st == WL_CONNECT_FAILED || st == WL_NO_SSID_AVAIL) {
      Serial.printf("WiFi status %d, reconnecting...\n", (int)st);
      WiFi.reconnect();
    }
  }

  static bool lastReading = HIGH;
  static bool stableState = HIGH;
  static unsigned long lastDebounce = 0;
  bool reading = digitalRead(BOOT_PIN);
  if (reading != lastReading) lastDebounce = millis();
  if (millis() - lastDebounce > 50) {
    if (reading != stableState) {
      stableState = reading;
      if (stableState == LOW) {
        if (isOn) setOff(); else setOn();
        Serial.printf("Power -> %s\n", isOn ? "on" : "off");
      }
    }
  }
  lastReading = reading;

  while (Serial.available()) {
    char c = Serial.read();
    if (c == '+') {
      currentVolume = min(1.0f, currentVolume + 0.05f);
      Serial.printf("Volume: %.2f\n", currentVolume);
    } else if (c == '-') {
      currentVolume = max(0.0f, currentVolume - 0.05f);
      Serial.printf("Volume: %.2f\n", currentVolume);
    }
  }
}
