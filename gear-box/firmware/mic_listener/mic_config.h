#pragma once

#include <ESP_I2S.h>

// ---- I2S microphone wiring (ESP32-S3-WROOM-2, octal flash/PSRAM) ----
// VDD -> 3V3 (never 5V), GND -> GND, L/R/SEL -> GND (left channel).
// Avoid GPIO 26-37 (octal flash/PSRAM), strapping pins 0/3/45/46, and the
// USB pins 19/20 -- the defaults below steer clear of all of them.
#ifndef MIC_BCLK_PIN
#define MIC_BCLK_PIN 5 // SCK / BCLK
#endif
#ifndef MIC_WS_PIN
#define MIC_WS_PIN 4 // WS / LRCL
#endif
#ifndef MIC_DIN_PIN
#define MIC_DIN_PIN 6 // SD / DOUT (mic's output, this chip's input)
#endif

// 22050Hz matches sound_machine's fixed I2S playback rate (and
// piper.SampleRate on the server) exactly, so a captured utterance can be
// forwarded to gear-box and on to the speaker with no resampling step.
#ifndef MIC_SAMPLE_RATE
#define MIC_SAMPLE_RATE 22050
#endif

// Most small round I2S MEMS breakouts (SPH0645/ICS-43434/INMP441-style)
// ship 24-bit data left-justified in a 32-bit I2S slot, not real 16-bit
// audio, so the driver is configured for the full 32-bit slot and mic.cpp
// shifts the result down to 16 bits afterward (see MIC_SAMPLE_SHIFT).
// Asking the driver for a 16-bit slot directly would read the wrong 16
// bits of that frame.
#ifndef MIC_BIT_WIDTH
#define MIC_BIT_WIDTH I2S_DATA_BIT_WIDTH_32BIT
#endif

// Shift applied to each 32-bit I2S slot to produce a 16-bit sample. 16 is
// right for standard MSB-justified 24-in-32 output (ICS-43434,
// INMP441-style parts). SPH0645 has a well-known one-bit timing offset in
// silicon -- if the signal comes through at roughly half (or double) the
// expected level, try 14 (or 18) instead.
#ifndef MIC_SAMPLE_SHIFT
#define MIC_SAMPLE_SHIFT 14
#endif

#ifndef MIC_RING_FRAMES
#define MIC_RING_FRAMES 4096 // 16-bit samples held in mic.cpp's ring buffer
#endif

#ifndef MIC_READ_FRAMES
#define MIC_READ_FRAMES 512 // frames pulled from the I2S driver per read() call
#endif

#ifndef MIC_TASK_STACK
#define MIC_TASK_STACK 4096
#endif
#ifndef MIC_TASK_PRIORITY
#define MIC_TASK_PRIORITY 5
#endif
#ifndef MIC_TASK_CORE
#define MIC_TASK_CORE 1
#endif

// ---- Utterance capture (simple RMS-threshold VAD) ----
// This is a deliberately crude voice-activity gate, not real VAD: a window
// of audio counts as "speech" once its RMS crosses MIC_VAD_RMS_THRESHOLD,
// and the utterance ends once RMS has stayed below that for
// MIC_VAD_SILENCE_MS straight. Tune the threshold against your actual
// room/mic -- there's no calibration step.
#ifndef MIC_VAD_RMS_THRESHOLD
#define MIC_VAD_RMS_THRESHOLD 600
#endif
#ifndef MIC_VAD_SILENCE_MS
#define MIC_VAD_SILENCE_MS 800
#endif
#ifndef MIC_VAD_MIN_UTTERANCE_MS
#define MIC_VAD_MIN_UTTERANCE_MS 300 // shorter blips are dropped, not sent
#endif

// Sanity cap on a single utterance, well under both gear-box's own upload
// cap and the speaker firmware's SPEAK_MAX_BYTES, since this buffer gets
// forwarded almost as-is from mic to speaker.
#ifndef MIC_VAD_MAX_SECONDS
#define MIC_VAD_MAX_SECONDS 8
#endif
#define MIC_VAD_MAX_BYTES ((size_t)MIC_VAD_MAX_SECONDS * (size_t)MIC_SAMPLE_RATE * sizeof(int16_t))
