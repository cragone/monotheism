package whisper

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func sineWavePCM(numSamples int, freqHz, sampleRate float64) []byte {
	pcm := make([]byte, numSamples*2)
	for i := range numSamples {
		v := math.Sin(2 * math.Pi * freqHz * float64(i) / sampleRate)
		binary.LittleEndian.PutUint16(pcm[i*2:i*2+2], uint16(int16(v*10000)))
	}
	return pcm
}

func TestResampleLength(t *testing.T) {
	pcm := sineWavePCM(22050, 440, 22050)

	out := Resample(pcm, 22050, 16000)

	gotSamples := len(out) / 2
	wantSamples := 16000
	if gotSamples != wantSamples {
		t.Fatalf("expected %d samples after resampling, got %d", wantSamples, gotSamples)
	}
}

func TestResampleNoOpWhenRatesMatch(t *testing.T) {
	pcm := sineWavePCM(100, 440, 22050)

	out := Resample(pcm, 22050, 22050)

	if len(out) != len(pcm) {
		t.Fatalf("expected no-op resample to preserve length, got %d want %d", len(out), len(pcm))
	}
}

func TestWrapWAVHeader(t *testing.T) {
	pcm := []byte{0x01, 0x02, 0x03, 0x04}
	wav := wrapWAV(pcm, 16000)

	if len(wav) != 44+len(pcm) {
		t.Fatalf("expected 44-byte header + %d bytes of pcm, got %d bytes total", len(pcm), len(wav))
	}
	if string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		t.Fatalf("missing RIFF/WAVE markers: %q", wav[0:12])
	}
	if string(wav[12:16]) != "fmt " || string(wav[36:40]) != "data" {
		t.Fatalf("missing fmt /data chunk markers: %q %q", wav[12:16], wav[36:40])
	}

	gotRate := binary.LittleEndian.Uint32(wav[24:28])
	if gotRate != 16000 {
		t.Fatalf("expected sample rate 16000 in header, got %d", gotRate)
	}
	gotDataLen := binary.LittleEndian.Uint32(wav[40:44])
	if int(gotDataLen) != len(pcm) {
		t.Fatalf("expected data chunk size %d, got %d", len(pcm), gotDataLen)
	}
	if string(wav[44:]) != string(pcm) {
		t.Fatalf("pcm bytes not preserved after header")
	}
}

// fakeWhisperBinary writes a tiny shell script that ignores its arguments
// and prints canned whisper-cli --output-json-shaped JSON to stdout,
// exercising Transcribe's parsing path without needing the real binary.
func fakeWhisperBinary(t *testing.T, json string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-whisper-cli")
	script := "#!/bin/sh\ncat <<'EOF'\n" + json + "\nEOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return path
}

func TestTranscribeJoinsSegments(t *testing.T) {
	binPath := fakeWhisperBinary(t, `{"transcription":[{"text":" hello "},{"text":" world"}]}`)
	tr := New(binPath, "unused-model-path")

	got, err := tr.Transcribe(context.Background(), sineWavePCM(2205, 440, 22050))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("expected %q, got %q", "hello world", got)
	}
}

func TestTranscribeSkipsBlankSegments(t *testing.T) {
	binPath := fakeWhisperBinary(t, `{"transcription":[{"text":"  "},{"text":"ok"}]}`)
	tr := New(binPath, "unused-model-path")

	got, err := tr.Transcribe(context.Background(), sineWavePCM(2205, 440, 22050))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Fatalf("expected %q, got %q", "ok", got)
	}
}

func TestTranscribeErrorsOnBinaryFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "failing-whisper-cli")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho boom >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	tr := New(path, "unused-model-path")

	_, err := tr.Transcribe(context.Background(), sineWavePCM(2205, 440, 22050))
	if err == nil {
		t.Fatal("expected an error when the whisper binary exits non-zero")
	}
}
