#include "mic.h"
#include "mic_config.h"

static I2SClass i2sMic;
static TaskHandle_t micTaskHandle = nullptr;
static volatile bool micRunning = false;

// Ring buffer of converted 16-bit samples, written by micTask (producer)
// and drained by micRead() (consumer) -- the same fixed-size,
// never-resized pattern sound_machine.ino uses for speakRing, and for the
// same reason: a static ring can't fragment the heap the way repeated
// malloc/free of audio buffers does.
static int16_t micRing[MIC_RING_FRAMES];
static volatile size_t micRingHead = 0; // next slot micTask writes
static volatile size_t micRingTail = 0; // next slot micRead() reads

static inline size_t micRingFree() {
  size_t used = (micRingHead - micRingTail + MIC_RING_FRAMES) % MIC_RING_FRAMES;
  return MIC_RING_FRAMES - 1 - used; // one slot always kept empty to tell full apart from empty
}

static void micTask(void *param) {
  (void)param;
  static int32_t raw[MIC_READ_FRAMES];

  for (;;) {
    if (!micRunning) {
      vTaskDelay(pdMS_TO_TICKS(10));
      continue;
    }

    size_t bytesRead = i2sMic.readBytes((char *)raw, sizeof(raw));
    size_t framesRead = bytesRead / sizeof(int32_t);

    for (size_t i = 0; i < framesRead; i++) {
      if (micRingFree() == 0) break; // consumer behind; drop the rest of this batch
      micRing[micRingHead] = (int16_t)(raw[i] >> MIC_SAMPLE_SHIFT);
      micRingHead = (micRingHead + 1) % MIC_RING_FRAMES;
    }
  }
}

bool micInit() {
  i2sMic.setPins(MIC_BCLK_PIN, MIC_WS_PIN, -1, MIC_DIN_PIN);
  if (!i2sMic.begin(I2S_MODE_STD, MIC_SAMPLE_RATE, MIC_BIT_WIDTH, I2S_SLOT_MODE_MONO, I2S_STD_SLOT_LEFT)) {
    return false;
  }

  if (micTaskHandle == nullptr) {
    xTaskCreatePinnedToCore(micTask, "mic", MIC_TASK_STACK, nullptr, MIC_TASK_PRIORITY, &micTaskHandle, MIC_TASK_CORE);
  }
  return true;
}

bool micStart() {
  if (micTaskHandle == nullptr) return false;
  micRingHead = 0;
  micRingTail = 0;
  micRunning = true;
  return true;
}

void micStop() {
  micRunning = false;
}

size_t micRead(int16_t *out, size_t maxSamples) {
  size_t n = 0;
  while (n < maxSamples && micRingTail != micRingHead) {
    out[n++] = micRing[micRingTail];
    micRingTail = (micRingTail + 1) % MIC_RING_FRAMES;
  }
  return n;
}
