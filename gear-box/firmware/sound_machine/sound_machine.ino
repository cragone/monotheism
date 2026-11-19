#include <WiFi.h>
#include <WebServer.h>
#include <ESPmDNS.h>
#include <ESP_I2S.h>
#include <esp_system.h>
#include <math.h>
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

enum Mode { MODE_WHITE = 0, MODE_PINK, MODE_BROWN, MODE_RAIN, MODE_OCEAN, MODE_FAN, MODE_TONE, MODE_SPEAK, MODE_COUNT };
static const char *MODE_NAMES[MODE_COUNT] = { "white", "pink", "brown", "rain", "ocean", "fan", "tone", "speak" };

#define SPEAK_MAX_BYTES 260000 // cap to stay within available heap

volatile bool isOn = true;
volatile int currentMode = MODE_WHITE;
volatile float currentVolume = 0.3f;

int16_t *speakBuf = nullptr;
size_t speakLen = 0;

I2SClass i2s;
WebServer server(80);
TaskHandle_t audioTaskHandle = nullptr;

// ---- Noise generators ----

static inline float randFloat() {
  return ((float)esp_random() / (float)UINT32_MAX) * 2.0f - 1.0f;
}

struct Droplet {
  bool active;
  float env;
  float decay;
};

struct GenState {
  float pb0, pb1, pb2, pb3, pb4, pb5, pb6; // pink
  float brown;
  float hpPrevIn, hpPrevOut; // rain hiss highpass
  Droplet droplets[8];
  float oceanBrown, oceanPhase;
  float fanLP, fanHumPhase;
  float tonePhase;
  size_t speakIndex;
};
static GenState gs;

void resetGenState() {
  memset(&gs, 0, sizeof(gs));
}

float generateSample(Mode mode) {
  float white = randFloat();

  switch (mode) {
    case MODE_WHITE:
      return white;

    case MODE_PINK: {
      gs.pb0 = 0.99886f * gs.pb0 + white * 0.0555179f;
      gs.pb1 = 0.99332f * gs.pb1 + white * 0.0750759f;
      gs.pb2 = 0.96900f * gs.pb2 + white * 0.1538520f;
      gs.pb3 = 0.86650f * gs.pb3 + white * 0.3104856f;
      gs.pb4 = 0.55000f * gs.pb4 + white * 0.5329522f;
      gs.pb5 = -0.7616f * gs.pb5 - white * 0.0168980f;
      float pink = gs.pb0 + gs.pb1 + gs.pb2 + gs.pb3 + gs.pb4 + gs.pb5 + gs.pb6 + white * 0.5362f;
      gs.pb6 = white * 0.115926f;
      return pink * 0.11f;
    }

    case MODE_BROWN: {
      gs.brown = (gs.brown + 0.02f * white) / 1.02f;
      return gs.brown * 3.5f;
    }

    case MODE_RAIN: {
      float hp = white - gs.hpPrevIn + 0.95f * gs.hpPrevOut;
      gs.hpPrevIn = white;
      gs.hpPrevOut = hp;
      float out = hp * 0.35f;

      if (randFloat() > 0.9995f) {
        for (auto &d : gs.droplets) {
          if (!d.active) {
            d.active = true;
            d.env = 1.0f;
            d.decay = 0.985f + (randFloat() * 0.5f + 0.5f) * 0.01f;
            break;
          }
        }
      }
      for (auto &d : gs.droplets) {
        if (d.active) {
          out += d.env * randFloat() * 0.6f;
          d.env *= d.decay;
          if (d.env < 0.01f) d.active = false;
        }
      }
      return out;
    }

    case MODE_OCEAN: {
      gs.oceanBrown = (gs.oceanBrown + 0.02f * white) / 1.02f;
      float brownOut = gs.oceanBrown * 3.5f;
      gs.oceanPhase += 2.0f * PI * (1.0f / 9.0f) / SAMPLE_RATE;
      if (gs.oceanPhase > 2.0f * PI) gs.oceanPhase -= 2.0f * PI;
      float swell = 0.5f + 0.5f * sinf(gs.oceanPhase);
      return brownOut * (0.3f + 0.7f * swell);
    }

    case MODE_FAN: {
      gs.fanLP += 0.08f * (white - gs.fanLP);
      gs.fanHumPhase += 2.0f * PI * 110.0f / SAMPLE_RATE;
      if (gs.fanHumPhase > 2.0f * PI) gs.fanHumPhase -= 2.0f * PI;
      float hum = sinf(gs.fanHumPhase) * 0.05f;
      return gs.fanLP * 0.9f + hum;
    }

    case MODE_TONE: {
      gs.tonePhase += 2.0f * PI * 440.0f / SAMPLE_RATE;
      if (gs.tonePhase > 2.0f * PI) gs.tonePhase -= 2.0f * PI;
      return sinf(gs.tonePhase) * 0.5f;
    }

    case MODE_SPEAK: {
      if (!speakBuf || gs.speakIndex >= speakLen) return 0.0f;
      float sample = speakBuf[gs.speakIndex] / 32768.0f;
      gs.speakIndex++;
      return sample;
    }

    default:
      return 0.0f;
  }
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
  Mode lastMode = (Mode)-1;
  float envGain = 0.0f;
  const float envStep = 1.0f / (FADE_MS / 1000.0f * SAMPLE_RATE);

  for (;;) {
    Mode mode = (Mode)currentMode;
    if (mode != lastMode) {
      resetGenState();
      lastMode = mode;
    }
    float target = currentVolume;

    for (int i = 0; i < BUFFER_SAMPLES; i++) {
      float raw = generateSample(mode);
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

int parseModeName(const String &name) {
  for (int i = 0; i < MODE_COUNT; i++) {
    if (name.equalsIgnoreCase(MODE_NAMES[i])) return i;
  }
  return -1;
}

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
           "{\"on\":%s,\"mode\":\"%s\",\"volume\":%.2f,\"ip\":\"%s\",\"rssi\":%d,\"uptime_s\":%lu}",
           isOn ? "true" : "false",
           MODE_NAMES[currentMode],
           currentVolume,
           WiFi.localIP().toString().c_str(),
           (int)WiFi.RSSI(),
           (unsigned long)(millis() / 1000));
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

void handleMode() {
  if (!checkAuth()) return;
  if (!server.hasArg("name")) {
    server.send(400, "application/json", "{\"error\":\"missing name\"}");
    return;
  }
  int m = parseModeName(server.arg("name"));
  if (m < 0) {
    server.send(400, "application/json", "{\"error\":\"unknown mode\"}");
    return;
  }
  currentMode = m;
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
int16_t *pendingSpeakBuf = nullptr;
size_t pendingSpeakCap = 0;
size_t pendingSpeakOffset = 0;
bool pendingSpeakAuthorized = false;
bool pendingSpeakOk = false;

void handleSpeakRaw() {
  HTTPRaw &raw = server.raw();
  switch (raw.status) {
    case RAW_START: {
      pendingSpeakOk = false;
      pendingSpeakOffset = 0;
      if (pendingSpeakBuf) {
        free(pendingSpeakBuf);
        pendingSpeakBuf = nullptr;
      }
      // Headers (and any ?token= query arg) are already fully parsed by the
      // time RAW_START fires, so this check still runs before any body
      // bytes are retained.
      pendingSpeakAuthorized = isAuthorized();
      if (!pendingSpeakAuthorized) break;

      int contentLen = server.clientContentLength();
      if (contentLen <= 0 || (size_t)contentLen > SPEAK_MAX_BYTES) break;

      pendingSpeakBuf = (int16_t *)malloc((size_t)contentLen);
      if (!pendingSpeakBuf) break;
      pendingSpeakCap = (size_t)contentLen;
      pendingSpeakOk = true;
      break;
    }

    case RAW_WRITE: {
      if (!pendingSpeakOk) break;
      if (pendingSpeakOffset + raw.currentSize > pendingSpeakCap) {
        pendingSpeakOk = false;
        break;
      }
      memcpy((uint8_t *)pendingSpeakBuf + pendingSpeakOffset, raw.buf, raw.currentSize);
      pendingSpeakOffset += raw.currentSize;
      break;
    }

    case RAW_END:
    case RAW_ABORTED:
    default:
      break; // finalized in handleSpeakPost, which runs right after this
  }
}

void handleSpeakPost() {
  if (!pendingSpeakAuthorized) {
    server.send(401, "application/json", "{\"error\":\"unauthorized\"}");
    if (pendingSpeakBuf) {
      free(pendingSpeakBuf);
      pendingSpeakBuf = nullptr;
    }
    return;
  }
  if (!pendingSpeakOk || !pendingSpeakBuf) {
    server.send(400, "application/json", "{\"error\":\"bad or missing body\"}");
    if (pendingSpeakBuf) {
      free(pendingSpeakBuf);
      pendingSpeakBuf = nullptr;
    }
    return;
  }

  if (speakBuf) free(speakBuf);
  speakBuf = pendingSpeakBuf;
  speakLen = pendingSpeakOffset / 2;
  pendingSpeakBuf = nullptr;
  currentMode = MODE_SPEAK;
  server.send(200, "application/json", buildStatusJson());
}

void handleSpeakGetNotAllowed() {
  server.send(405, "application/json", "{\"error\":\"POST only\"}");
}

void handleModes() {
  if (!checkAuth()) return;
  String json = "{\"modes\":[";
  for (int i = 0; i < MODE_COUNT; i++) {
    if (i) json += ",";
    json += "\"";
    json += MODE_NAMES[i];
    json += "\"";
  }
  json += "]}";
  server.send(200, "application/json", json);
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
  server.on("/mode", HTTP_ANY, handleMode);
  server.on("/volume", HTTP_ANY, handleVolume);
  server.on("/modes", HTTP_ANY, handleModes);
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
        currentMode = (currentMode + 1) % MODE_COUNT;
        Serial.printf("Mode -> %s\n", MODE_NAMES[currentMode]);
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
