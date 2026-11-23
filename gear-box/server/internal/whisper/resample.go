package whisper

import "encoding/binary"

// Resample linearly interpolates mono 16-bit signed little-endian PCM from
// fromHz to toHz. This is a deliberately simple resampler (no
// windowed-sinc/anti-aliasing filter) -- acceptable for the mild 22050->16000
// downsample this package actually needs, feeding a model that already
// tolerates noisy input, not a mastering-quality requirement.
func Resample(pcm []byte, fromHz, toHz int) []byte {
	if fromHz == toHz || len(pcm) < 4 {
		return pcm
	}

	n := len(pcm) / 2
	samples := make([]int16, n)
	for i := range n {
		samples[i] = int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
	}

	outLen := max(n*toHz/fromHz, 1)
	ratio := float64(fromHz) / float64(toHz)

	out := make([]byte, outLen*2)
	for i := range outLen {
		srcPos := float64(i) * ratio
		i0 := int(srcPos)
		if i0 >= n-1 {
			binary.LittleEndian.PutUint16(out[i*2:i*2+2], uint16(samples[n-1]))
			continue
		}

		frac := srcPos - float64(i0)
		s0, s1 := float64(samples[i0]), float64(samples[i0+1])
		interpolated := int16(s0 + (s1-s0)*frac)
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], uint16(interpolated))
	}

	return out
}
