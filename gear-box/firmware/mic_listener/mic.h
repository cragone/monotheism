#pragma once

#include <stddef.h>
#include <stdint.h>

// I2S microphone capture (RX, mono, left slot). See mic_config.h for
// pins, sample rate, bit width, and the 32-bit -> 16-bit shift.
//
// micInit() configures the I2S peripheral and starts a FreeRTOS task that
// continuously drains it into a small ring buffer; micStart()/micStop()
// toggle whether that task is actively capturing. micRead() drains the
// ring into the caller's buffer.

bool micInit();
bool micStart();
void micStop();

// Copies up to maxSamples 16-bit samples out of the ring into out, oldest
// first. Returns the number actually copied (0 if nothing is buffered).
size_t micRead(int16_t *out, size_t maxSamples);
