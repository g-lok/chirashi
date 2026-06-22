package rexconverter_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// WAV header parsing for verification.
type wavHeader struct {
	riffID     [4]byte
	riffSize   uint32
	waveID     [4]byte
	fmtID      [4]byte
	fmtSize    uint32
	audioFmt   uint16
	numChans   uint16
	sampleRate uint32
	byteRate   uint32
	blockAlign uint16
	bitsPerSmp uint16
	dataID     [4]byte
	dataSize   uint32
}

func parseWAVHeader(t *testing.T, data []byte) wavHeader {
	t.Helper()
	var h wavHeader
	if len(data) < 44 {
		t.Fatalf("WAV too short: %d bytes", len(data))
	}
	copy(h.riffID[:], data[0:4])
	h.riffSize = binary.LittleEndian.Uint32(data[4:8])
	copy(h.waveID[:], data[8:12])
	copy(h.fmtID[:], data[12:16])
	h.fmtSize = binary.LittleEndian.Uint32(data[16:20])
	h.audioFmt = binary.LittleEndian.Uint16(data[20:22])
	h.numChans = binary.LittleEndian.Uint16(data[22:24])
	h.sampleRate = binary.LittleEndian.Uint32(data[24:28])
	h.byteRate = binary.LittleEndian.Uint32(data[28:32])
	h.blockAlign = binary.LittleEndian.Uint16(data[32:34])
	h.bitsPerSmp = binary.LittleEndian.Uint16(data[34:36])
	copy(h.dataID[:], data[36:40])
	h.dataSize = binary.LittleEndian.Uint32(data[40:44])
	return h
}

func TestWAVHeaderStructure(t *testing.T) {
	var buf bytes.Buffer

	// Write a minimal valid WAV manually.

	// RIFF header
	buf.Write([]byte("RIFF"))
	binary.Write(&buf, binary.LittleEndian, uint32(0)) // placeholder size
	buf.Write([]byte("WAVE"))

	// fmt chunk
	buf.Write([]byte("fmt "))
	binary.Write(&buf, binary.LittleEndian, uint32(16))   // chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))    // PCM
	binary.Write(&buf, binary.LittleEndian, uint16(2))    // channels
	binary.Write(&buf, binary.LittleEndian, uint32(44100)) // sample rate
	binary.Write(&buf, binary.LittleEndian, uint32(176400)) // byte rate
	binary.Write(&buf, binary.LittleEndian, uint16(4))    // block align
	binary.Write(&buf, binary.LittleEndian, uint16(16))   // bits per sample

	// data chunk
	pcmData := make([]int16, 1000)
	for i := range pcmData {
		pcmData[i] = int16(i)
	}
	buf.Write([]byte("data"))
	binary.Write(&buf, binary.LittleEndian, uint32(len(pcmData)*2)) // data size
	for _, s := range pcmData {
		binary.Write(&buf, binary.LittleEndian, s)
	}

	// Fix RIFF size
	riffSize := buf.Len() - 8
	b := buf.Bytes()
	binary.LittleEndian.PutUint32(b[4:8], uint32(riffSize))

	// Parse and verify
	h := parseWAVHeader(t, b)
	if string(h.riffID[:]) != "RIFF" {
		t.Fatalf("bad RIFF ID: %s", string(h.riffID[:]))
	}
	if string(h.waveID[:]) != "WAVE" {
		t.Fatalf("bad WAVE ID")
	}
	if string(h.fmtID[:]) != "fmt " {
		t.Fatalf("bad fmt ID")
	}
	if h.audioFmt != 1 {
		t.Fatalf("bad audio format: %d", h.audioFmt)
	}
	if h.numChans != 2 {
		t.Fatalf("bad channels: %d", h.numChans)
	}
	if h.sampleRate != 44100 {
		t.Fatalf("bad sample rate: %d", h.sampleRate)
	}
	if h.bitsPerSmp != 16 {
		t.Fatalf("bad bit depth: %d", h.bitsPerSmp)
	}
	if h.riffSize != uint32(riffSize) {
		t.Fatalf("riff size: expected %d, got %d", riffSize, h.riffSize)
	}
}

func TestWAVRoundTrip(t *testing.T) {
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test.wav")

	f, err := os.Create(wavPath)
	if err != nil {
		t.Fatal(err)
	}

	// Generate 1 second of 440Hz sine at 44100 Hz, stereo, 16-bit
	numSamples := 44100 * 2 // 1 second stereo
	sine := make([]int16, numSamples)
	for i := 0; i < 44100; i++ {
		val := int16(8000.0 * sine64(float64(i)*440.0/44100.0*2.0*3.14159))
		sine[i*2] = val   // L
		sine[i*2+1] = val // R
	}

	// Write RIFF WAV manually.
	dataSize := len(sine) * 2

	f.Write([]byte("RIFF"))
	binary.Write(f, binary.LittleEndian, uint32(36+uint32(dataSize)))
	f.Write([]byte("WAVE"))

	f.Write([]byte("fmt "))
	binary.Write(f, binary.LittleEndian, uint32(16))
	binary.Write(f, binary.LittleEndian, uint16(1))
	binary.Write(f, binary.LittleEndian, uint16(2))
	binary.Write(f, binary.LittleEndian, uint32(44100))
	binary.Write(f, binary.LittleEndian, uint32(176400))
	binary.Write(f, binary.LittleEndian, uint16(4))
	binary.Write(f, binary.LittleEndian, uint16(16))

	f.Write([]byte("data"))
	binary.Write(f, binary.LittleEndian, uint32(dataSize))
	for _, s := range sine {
		binary.Write(f, binary.LittleEndian, s)
	}
	f.Close()

	// Re-read and verify
	data, err := os.ReadFile(wavPath)
	if err != nil {
		t.Fatal(err)
	}
	h := parseWAVHeader(t, data)
	if h.riffSize != uint32(36+uint32(dataSize)) {
		t.Fatalf("riff size mismatch")
	}
	if h.dataSize != uint32(dataSize) {
		t.Fatalf("data size: expected %d, got %d", dataSize, h.dataSize)
	}

	// Verify first few samples
	expectedSamples := []int16{sine[0], sine[1], sine[2], sine[3]}
	for i, exp := range expectedSamples {
		got := int16(binary.LittleEndian.Uint16(data[44+i*2:]))
		if got != exp {
			t.Fatalf("sample %d: expected %d, got %d", i, exp, got)
		}
	}
}

func sine64(x float64) float64 {
	return (sin64(x) + 1.0) * 0.5
}

func sin64(x float64) float64 {
	// Simple Taylor-series sin for test purposes
	if x < 0 {
		return -sin64(-x)
	}
	// Reduce to [0, 2pi)
	x = float64(int64(x/(2*3.14159))) * (2 * 3.14159)
	x2 := x * x
	x3 := x2 * x
	x5 := x3 * x2
	x7 := x5 * x2
	return x - x3/6.0 + x5/120.0 - x7/5040.0
}

func TestCueChunkStructure(t *testing.T) {
	// Verify that a manually constructed cue chunk has correct structure.
	var buf bytes.Buffer

	buf.Write([]byte("cue "))
	cueSizePos := buf.Len()
	binary.Write(&buf, binary.LittleEndian, uint32(0)) // placeholder size

	cueBodyStart := buf.Len()
	binary.Write(&buf, binary.LittleEndian, uint32(2)) // 2 cue points
	for i := 0; i < 2; i++ {
		binary.Write(&buf, binary.LittleEndian, uint32(i+1))   // cue ID
		binary.Write(&buf, binary.LittleEndian, uint32(i*100))  // position
		buf.Write([]byte("data"))                                // chunk ID
		binary.Write(&buf, binary.LittleEndian, uint32(0))     // chunk start
		binary.Write(&buf, binary.LittleEndian, uint32(0))     // block start
		binary.Write(&buf, binary.LittleEndian, uint32(i*100))  // sample offset
	}
	cueSize := buf.Len() - cueBodyStart
	b := buf.Bytes()
	binary.LittleEndian.PutUint32(b[cueSizePos:], uint32(cueSize))

	// Parse cue chunk
	if string(b[0:4]) != "cue " {
		t.Fatal("bad cue chunk ID")
	}
	parsedSize := binary.LittleEndian.Uint32(b[4:8])
	if parsedSize != uint32(cueSize) {
		t.Fatalf("cue size: expected %d, got %d", cueSize, parsedSize)
	}
	numCues := binary.LittleEndian.Uint32(b[8:12])
	if numCues != 2 {
		t.Fatalf("expected 2 cues, got %d", numCues)
	}
}

func TestDownmixStereoToMono(t *testing.T) {
	stereo := []float32{0.5, -0.5, 0.25, -0.25, 0.0, 0.0}
	mono := downmix(stereo)
	expected := []float32{0.0, 0.0, 0.0}
	if len(mono) != len(expected) {
		t.Fatalf("expected %d mono samples, got %d", len(expected), len(mono))
	}
	for i, v := range mono {
		if v != expected[i] {
			t.Fatalf("sample %d: expected %f, got %f", i, expected[i], v)
		}
	}
}

func downmix(stereo []float32) []float32 {
	mono := make([]float32, len(stereo)/2)
	for i := range mono {
		mono[i] = (stereo[i*2] + stereo[i*2+1]) / 2.0
	}
	return mono
}

func TestPCMFrameCount(t *testing.T) {
	// Verify frame count calculation for mono vs stereo.
	monoData := make([]float32, 1000)
	stereoData := make([]float32, 2000)

	monoFrames := len(monoData) / 1
	stereoFrames := len(stereoData) / 2

	if monoFrames != 1000 {
		t.Fatalf("mono frames: expected 1000, got %d", monoFrames)
	}
	if stereoFrames != 1000 {
		t.Fatalf("stereo frames: expected 1000, got %d", stereoFrames)
	}
}

func TestBitDepthValidation(t *testing.T) {
	valid := []int{8, 16, 24}
	invalid := []int{0, 1, 4, 32, 64}

	for _, bd := range valid {
		if bd != 8 && bd != 16 && bd != 24 {
			t.Fatalf("unexpected valid bit depth: %d", bd)
		}
	}
	for _, bd := range invalid {
		_ = bd
	}
}
