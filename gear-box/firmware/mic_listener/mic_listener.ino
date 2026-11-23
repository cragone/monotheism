#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <HTTPClient.h>
#include <math.h>
#include "mic.h"
#include "mic_config.h"
#include "secrets.h"

// Captures an utterance (RMS-threshold VAD, see mic_config.h) and POSTs it
// to gear-box's /hear endpoint as raw PCM -- mono, 16-bit signed
// little-endian, MIC_SAMPLE_RATE Hz. gear-box currently just echoes that
// straight back out through sound_machine's speaker, unchanged: this is
// step one (mic -> gear-box -> speaker) ahead of gear-box running an LLM
// over what it heard instead of just replaying it.
//
// The utterance buffer is allocated once from PSRAM at startup and reused
// for every utterance -- same reasoning as speakRing/micRing elsewhere in
// this codebase: a buffer this size (MIC_VAD_MAX_SECONDS worth of audio)
// shouldn't come from a repeated malloc/free cycle, and it's too big for
// this chip's internal SRAM to hold as a plain static array.
static uint8_t *utteranceBuf = nullptr;
static size_t utteranceLen = 0;
static bool speaking = false;
static unsigned long speechStartedAt = 0;
static unsigned long lastLoudAt = 0;

static void connectWiFi() {
  WiFi.mode(WIFI_STA);
  // Default modem-sleep power save only wakes the radio at the AP's
  // beacon/DTIM interval, which measured as 26-230ms of jitter and
  // occasional dropped connects to gear-box on this specific network --
  // made worse by the I2S capture task on the other core competing for
  // scheduling. Trading a bit more power draw for a radio that's always
  // listening fixes both.
  WiFi.setSleep(false);
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
}

// GEARBOX_HOST is gear-box's public hostname (reached through the same
// Cloudflare Tunnel + Traefik path-scoped carve-out as a browser hitting
// gear-box.charlesragone.com/hear directly, see gear-box-ingress.yaml) --
// not the LAN NodePort this used to hit. The mic's direct LAN connects to
// that NodePort were reliably refused for reasons that looked like
// router-level device isolation (ICMP to it was clean, other LAN clients
// reached it fine, only this device's outbound TCP connects there failed),
// so this routes around the LAN entirely instead of fighting that.
// setInsecure() skips certificate validation -- pragmatic for a
// constrained device reaching through Cloudflare's edge rather than
// pinning/rotating a CA cert, at the cost of not verifying who's on the
// other end of the TLS handshake. MIC_API_TOKEN is what actually gates
// this now that it's reachable from the public internet, not the TLS
// session itself.
//
// This house's WiFi is documented elsewhere in this codebase as middling
// and occasionally flaky (see esp32.Client.Speak's own retry-once, on the
// other leg of this same round trip) -- one retry on a dropped connection
// shouldn't fail the whole utterance.
static int sendUtteranceOnce(const uint8_t *pcm, size_t len) {
  WiFiClientSecure client;
  client.setInsecure();

  HTTPClient http;
  String url = String("https://") + GEARBOX_HOST + "/hear";
  http.begin(client, url);
  http.setTimeout(15000);
  http.addHeader("Content-Type", "application/octet-stream");
  if (strlen(GEARBOX_API_TOKEN) > 0) {
    http.addHeader("X-Token", GEARBOX_API_TOKEN);
  }

  int status = http.POST((uint8_t *)pcm, len);
  http.end();
  return status;
}

static void sendUtterance(const uint8_t *pcm, size_t len) {
  int status = sendUtteranceOnce(pcm, len);
  if (status != HTTP_CODE_OK) {
    Serial.printf("send failed (status=%d, %s), rssi=%d, retrying once\n", status,
                  HTTPClient::errorToString(status).c_str(), (int)WiFi.RSSI());
    delay(1000);
    status = sendUtteranceOnce(pcm, len);
  }
  Serial.printf("sent %u bytes to gear-box, status=%d\n", (unsigned)len, status);
}

void setup() {
  Serial.begin(115200);
  delay(200);

  utteranceBuf = (uint8_t *)heap_caps_malloc(MIC_VAD_MAX_BYTES, MALLOC_CAP_SPIRAM);
  if (!utteranceBuf) {
    Serial.println("failed to allocate utterance buffer from PSRAM");
    while (true) delay(1000);
  }

  if (!micInit()) {
    Serial.println("mic init failed");
    while (true) delay(1000);
  }
  micStart();

  connectWiFi();
  Serial.println("listening");
}

void loop() {
  static int16_t buf[MIC_READ_FRAMES];

  static unsigned long lastWifiCheck = 0;
  if (millis() - lastWifiCheck > 15000) {
    lastWifiCheck = millis();
    wl_status_t st = WiFi.status();
    if (st == WL_DISCONNECTED || st == WL_CONNECTION_LOST || st == WL_CONNECT_FAILED || st == WL_NO_SSID_AVAIL) {
      Serial.printf("WiFi status %d, reconnecting...\n", (int)st);
      WiFi.reconnect();
    }
  }

  size_t n = micRead(buf, MIC_READ_FRAMES);
  if (n == 0) {
    delay(2);
    return;
  }

  double sumSquares = 0;
  for (size_t i = 0; i < n; i++) {
    sumSquares += (double)buf[i] * (double)buf[i];
  }
  double rms = sqrt(sumSquares / n);
  bool loud = rms > MIC_VAD_RMS_THRESHOLD;

  if (loud) {
    if (!speaking) {
      speaking = true;
      utteranceLen = 0;
      speechStartedAt = millis();
      Serial.println("speech start");
    }
    lastLoudAt = millis();
  }

  if (speaking) {
    size_t bytesIn = n * sizeof(int16_t);
    size_t room = MIC_VAD_MAX_BYTES - utteranceLen;
    size_t toCopy = bytesIn < room ? bytesIn : room;
    memcpy(utteranceBuf + utteranceLen, (const uint8_t *)buf, toCopy);
    utteranceLen += toCopy;

    bool silenceTimedOut = millis() - lastLoudAt > MIC_VAD_SILENCE_MS;
    bool full = utteranceLen >= MIC_VAD_MAX_BYTES;

    if (silenceTimedOut || full) {
      speaking = false;
      unsigned long durationMs = millis() - speechStartedAt;

      if (durationMs >= MIC_VAD_MIN_UTTERANCE_MS && utteranceLen > 0) {
        Serial.printf("speech end: %lums, %u bytes\n", durationMs, (unsigned)utteranceLen);
        if (WiFi.status() == WL_CONNECTED) {
          sendUtterance(utteranceBuf, utteranceLen);
        } else {
          Serial.println("WiFi down, dropping utterance");
        }
      } else {
        Serial.println("speech too short, dropped");
      }
      utteranceLen = 0;
    }
  }
}
