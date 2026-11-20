#include <math.h>
#include "mic.h"
#include "mic_config.h"

// Minimal demo: start the mic and log RMS/peak every ~200ms so you can
// verify it reacts to sound before wiring it into anything bigger.

void setup() {
  Serial.begin(115200);
  delay(200);

  if (!micInit()) {
    Serial.println("mic init failed");
    while (true) delay(1000);
  }
  micStart();
  Serial.println("mic capture started");
}

void loop() {
  static int16_t buf[MIC_READ_FRAMES];
  static double sumSquares = 0;
  static int peak = 0;
  static size_t windowCount = 0;
  static unsigned long lastReport = 0;

  size_t n = micRead(buf, MIC_READ_FRAMES);
  for (size_t i = 0; i < n; i++) {
    sumSquares += (double)buf[i] * (double)buf[i];
    int mag = abs((int)buf[i]);
    if (mag > peak) peak = mag;
    windowCount++;
  }

  if (millis() - lastReport >= 200) {
    lastReport = millis();
    double rms = windowCount > 0 ? sqrt(sumSquares / windowCount) : 0.0;
    Serial.printf("rms=%.1f peak=%d samples=%u\n", rms, peak, (unsigned)windowCount);
    sumSquares = 0;
    peak = 0;
    windowCount = 0;
  }

  delay(5);
}
