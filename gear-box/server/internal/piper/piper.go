package piper

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

const (
	// SampleRate must match the ESP32 firmware's fixed I2S playback rate
	// (see gear-box/firmware/sound_machine/sound_machine.ino, SAMPLE_RATE).
	SampleRate = 22050

	// MaxTextChars mirrors the firmware's own text-length cap from its old
	// handleSpeak handler, enforced here too so a long request fails fast
	// instead of after an expensive synthesis call.
	MaxTextChars = 200

	// MaxOutputBytes mirrors the firmware's SPEAK_MAX_BYTES buffer ceiling.
	MaxOutputBytes = 260000
)

// Synthesizer runs the Piper CLI as a subprocess to turn text into raw PCM.
type Synthesizer struct {
	Binary    string
	ModelPath string
}

func New(binary, modelPath string) *Synthesizer {
	return &Synthesizer{Binary: binary, ModelPath: modelPath}
}

// Synthesize returns raw PCM: mono, 16-bit signed little-endian, SampleRate Hz.
func (s *Synthesizer) Synthesize(ctx context.Context, text string) ([]byte, error) {
	if len(text) == 0 || len(text) > MaxTextChars {
		return nil, fmt.Errorf("text must be 1-%d chars", MaxTextChars)
	}

	cmd := exec.CommandContext(ctx, s.Binary,
		"--model", s.ModelPath,
		"--output-raw",
		"--sample_rate", fmt.Sprintf("%d", SampleRate),
	)
	cmd.Stdin = bytes.NewReader([]byte(text))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("piper: %w: %s", err, stderr.String())
	}

	pcm := stdout.Bytes()
	if len(pcm) == 0 {
		return nil, fmt.Errorf("piper produced no audio")
	}
	if len(pcm) > MaxOutputBytes {
		return nil, fmt.Errorf("synthesized audio (%d bytes) exceeds device buffer cap (%d bytes)", len(pcm), MaxOutputBytes)
	}

	return pcm, nil
}
