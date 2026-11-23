package whisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const (
	// InputSampleRate documents the rate mic_listener/piper actually
	// produce (see piper.SampleRate) -- the rate Transcribe converts from.
	InputSampleRate = 22050

	// ModelSampleRate is what whisper.cpp's stock English models expect.
	// whisper.cpp's own audio reader does not resample; feeding it audio
	// at the wrong rate silently degrades or breaks transcription, so
	// Transcribe always resamples before invoking the binary.
	ModelSampleRate = 16000

	// MaxTranscriptChars is a sanity cap on returned text, mirroring
	// piper.MaxTextChars's role in the opposite direction. A single VAD
	// utterance (at most 8s) should never produce anywhere near this much
	// text; it exists to bound a pathological/garbage transcription.
	MaxTranscriptChars = 2000
)

// Transcriber runs the whisper.cpp CLI (whisper-cli) as a subprocess to
// turn raw PCM into text.
type Transcriber struct {
	Binary    string
	ModelPath string
}

func New(binary, modelPath string) *Transcriber {
	return &Transcriber{Binary: binary, ModelPath: modelPath}
}

// whisperJSON mirrors whisper-cli's --output-json shape closely enough to
// pull out what this package needs; unrecognized fields are ignored by
// encoding/json.
type whisperJSON struct {
	Transcription []struct {
		Text string `json:"text"`
	} `json:"transcription"`
}

// Transcribe takes raw PCM (mono, 16-bit signed little-endian,
// InputSampleRate Hz -- the shape mic_listener uploads to /hear) and
// returns the transcribed text. It resamples to ModelSampleRate and wraps
// the result in a WAV header before handing it to whisper-cli: the binary
// reads audio containers (wav/mp3/flac/ogg) from stdin via "-f -", not
// headerless raw PCM.
func (t *Transcriber) Transcribe(ctx context.Context, pcm []byte) (string, error) {
	resampled := Resample(pcm, InputSampleRate, ModelSampleRate)
	wav := wrapWAV(resampled, ModelSampleRate)

	cmd := exec.CommandContext(ctx, t.Binary,
		"-m", t.ModelPath,
		"-f", "-",
		"--output-json", "-of", "-",
		"-nt", "-np",
		"-l", "en",
	)
	cmd.Stdin = bytes.NewReader(wav)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("whisper: %w: %s", err, stderr.String())
	}

	var parsed whisperJSON
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return "", fmt.Errorf("whisper: parse output: %w", err)
	}

	var text strings.Builder
	for _, seg := range parsed.Transcription {
		trimmed := strings.TrimSpace(seg.Text)
		if trimmed == "" {
			continue
		}
		if text.Len() > 0 {
			text.WriteByte(' ')
		}
		text.WriteString(trimmed)
	}

	result := text.String()
	if len(result) > MaxTranscriptChars {
		result = result[:MaxTranscriptChars]
	}
	return result, nil
}
