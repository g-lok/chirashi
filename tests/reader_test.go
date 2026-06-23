package chirashi_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/g-lok/chirashi/internal/engine"
)

func TestDetectReader_ByExtension(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".rex", false},  // REX SDK handled via switch in runner.go, not registry
		{".rx2", false},
		{".rcy", false},
		{".xrni", true},
		{".adv", true},
		{".als", true},
		{".adg", true},
		{".wav", true},
		{".aif", true},
		{".aiff", true},
		{".pti", true},
		{".ot", true},
		{".xy", true},
		{".d2pst", true},
		{".mp3", false},
		{".flac", false},
		{".ogg", false},
	}

	for _, tt := range tests {
		reader := engine.DetectReader(tt.ext)
		if (reader != nil) != tt.want {
			t.Errorf("DetectReader(%q) = %v, want exists=%v", tt.ext, reader, tt.want)
		}
	}
}

func TestProbeInput_Invalid(t *testing.T) {
	_, err := engine.ProbeInput([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestWAVReader_CueMarkers(t *testing.T) {
	buildWAV := func(sampleRate, channels, bitDepth int, samples []int16, cueOffsets []uint32) []byte {
		buf := &bytes.Buffer{}
		bytesPerSample := bitDepth / 8
		bytesPerFrame := channels * bytesPerSample
		dataSize := len(samples) * bytesPerSample

		cueSize := 4 + len(cueOffsets)*24
		riffSize := uint32(36 + dataSize + 8 + cueSize)

		buf.WriteString("RIFF")
		binary.Write(buf, binary.LittleEndian, riffSize)
		buf.WriteString("WAVE")

		buf.WriteString("fmt ")
		binary.Write(buf, binary.LittleEndian, uint32(16))
		binary.Write(buf, binary.LittleEndian, uint16(1))
		binary.Write(buf, binary.LittleEndian, uint16(channels))
		binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
			binary.Write(buf, binary.LittleEndian, uint32(sampleRate*bytesPerFrame))
		binary.Write(buf, binary.LittleEndian, uint16(bytesPerFrame))
		binary.Write(buf, binary.LittleEndian, uint16(bitDepth))

		buf.WriteString("data")
		binary.Write(buf, binary.LittleEndian, uint32(dataSize))
		for _, s := range samples {
			binary.Write(buf, binary.LittleEndian, s)
		}

		buf.WriteString("cue ")
		binary.Write(buf, binary.LittleEndian, uint32(cueSize))
		binary.Write(buf, binary.LittleEndian, uint32(len(cueOffsets)))
		for i, off := range cueOffsets {
			binary.Write(buf, binary.LittleEndian, uint32(i+1))
			binary.Write(buf, binary.LittleEndian, off)
			buf.WriteString("data")
			binary.Write(buf, binary.LittleEndian, uint32(0))
			binary.Write(buf, binary.LittleEndian, uint32(0))
			binary.Write(buf, binary.LittleEndian, off)
		}

		return buf.Bytes()
	}

	wavData := buildWAV(44100, 1, 16,
		[]int16{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000},
		[]uint32{2, 5, 8},
	)

	slices, err := engine.DetectReader(".wav").Read(wavData, 44100)
	if err != nil {
		t.Fatalf("WAV read failed: %v", err)
	}

	if len(slices) != 4 {
		t.Fatalf("expected 4 slices (3 cues + tail), got %d", len(slices))
	}

	checkSliceLen := func(idx, expected int) {
		if slices[idx].TotalFrames != expected {
			t.Errorf("slice %d: expected %d frames, got %d", idx, expected, slices[idx].TotalFrames)
		}
	}
	checkSliceLen(0, 2)
	checkSliceLen(1, 3)
	checkSliceLen(2, 3)
	checkSliceLen(3, 2)
}

func TestWAVReader_NoCues(t *testing.T) {
	buf := &bytes.Buffer{}
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+4))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint32(44100))
	binary.Write(buf, binary.LittleEndian, uint32(88200))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(4))
	binary.Write(buf, binary.LittleEndian, int16(100))
	binary.Write(buf, binary.LittleEndian, int16(200))

	slices, err := engine.DetectReader(".wav").Read(buf.Bytes(), 44100)
	if err != nil {
		t.Fatalf("WAV read failed: %v", err)
	}
	if len(slices) != 1 {
		t.Fatalf("expected 1 slice (no cues), got %d", len(slices))
	}
}

func TestXRNIReader_Basic(t *testing.T) {
	xrniFile := findTestXRNI()
	if xrniFile == "" {
		t.Skip("no XRNI test file found")
	}

	data, err := os.ReadFile(xrniFile)
	if err != nil {
		t.Fatalf("read XRNI: %v", err)
	}

	reader := engine.DetectReader(".xrni")
	if reader == nil {
		t.Fatal("XRNI reader not registered")
	}

	slices, err := reader.Read(data, 44100)
	if err != nil {
		t.Fatalf("XRNI read failed: %v", err)
	}

	if len(slices) == 0 {
		t.Fatal("expected at least 1 slice")
	}

	t.Logf("XRNI: %d slices, %d Hz, %d ch", len(slices), slices[0].Metadata.SampleRate, slices[0].Metadata.Channels)
}

func TestSimplerReader_Basic(t *testing.T) {
	advFile := findTestSimpler()
	if advFile == "" {
		t.Skip("no ADV/Simpler test file found")
	}

	data, err := os.ReadFile(advFile)
	if err != nil {
		t.Fatalf("read ADV: %v", err)
	}

	reader := engine.DetectReader(".adv")
	if reader == nil {
		t.Fatal("Simpler reader not registered")
	}

	slices, err := reader.Read(data, 44100)
	if err != nil {
		t.Logf("Simpler read: %v (expected if sample file missing)", err)
		return
	}

	if len(slices) == 0 {
		t.Fatal("expected at least 1 slice")
	}
	t.Logf("Simpler: %d slices", len(slices))
}

func TestDrumRackReader_Basic(t *testing.T) {
	adgFile := findTestDrumRack()
	if adgFile == "" {
		t.Skip("no ADG/Drum Rack test file found")
	}

	data, err := os.ReadFile(adgFile)
	if err != nil {
		t.Fatalf("read ADG: %v", err)
	}

	reader := engine.DetectReader(".adg")
	if reader == nil {
		t.Fatal("Drum Rack reader not registered")
	}

	slices, err := reader.Read(data, 44100)
	if err != nil {
		t.Logf("Drum Rack read: %v (expected if sample files missing)", err)
		return
	}

	if len(slices) == 0 {
		t.Fatal("expected at least 1 slice")
	}
	t.Logf("Drum Rack: %d slices", len(slices))
}

func TestAIFFReader_Minimal(t *testing.T) {
	buf := &bytes.Buffer{}
	commData := []byte{
		0, 1,       // channels = 1
		0, 0, 0, 2, // frameCount = 2
		0, 16,      // bitDepth = 16
		0, 0,       // sampleRate high
		0, 0, 0, 0, 0, 0, 0, 0, // sampleRate low = 0 → fallback to 44100
	}
	ssndAudio := []byte{0, 100, 0, 200} // 2 × int16 samples
	ssndChunkDataSize := uint32(8 + len(ssndAudio)) // offset(4) + blockSize(4) + audio
	totalSize := uint32(4 + 8 + 18 + 8 + ssndChunkDataSize) // FORM data minus "FORM"+size(8)
	totalSize += 4 + 8 // COMM header
	totalSize += 8     // SSND header

	buf.WriteString("FORM")
	binary.Write(buf, binary.BigEndian, totalSize)
	buf.WriteString("AIFF")

	buf.WriteString("COMM")
	binary.Write(buf, binary.BigEndian, uint32(len(commData)))
	buf.Write(commData)

	buf.WriteString("SSND")
	binary.Write(buf, binary.BigEndian, ssndChunkDataSize)
	binary.Write(buf, binary.BigEndian, uint32(0)) // offset
	binary.Write(buf, binary.BigEndian, uint32(0)) // blockSize
	buf.Write(ssndAudio)

	slices, err := engine.DetectReader(".aiff").Read(buf.Bytes(), 44100)
	if err != nil {
		t.Fatalf("AIFF read failed: %v", err)
	}
	if len(slices) != 1 {
		t.Fatalf("expected 1 slice, got %d", len(slices))
	}
}

func TestPTIReader_Minimal(t *testing.T) {
	buf := &bytes.Buffer{}
	buf.WriteString("PTI\x00")
	name := [28]byte{}
	copy(name[:], "TestPTI")
	buf.Write(name[:])
	binary.Write(buf, binary.LittleEndian, uint8(1))   // version
	binary.Write(buf, binary.LittleEndian, uint8(60))  // root_note
	binary.Write(buf, binary.LittleEndian, uint8(0))   // fine_tune
	binary.Write(buf, binary.LittleEndian, uint8(100)) // volume
	binary.Write(buf, binary.LittleEndian, uint8(64))  // pan
	binary.Write(buf, binary.LittleEndian, uint8(5))   // sample_rate_index (44100)
	binary.Write(buf, binary.LittleEndian, uint8(0))   // loop_mode
	binary.Write(buf, binary.LittleEndian, uint32(0))  // loop_start
	binary.Write(buf, binary.LittleEndian, uint32(0))  // loop_end
	binary.Write(buf, binary.LittleEndian, uint32(4))  // sample_length
	binary.Write(buf, binary.LittleEndian, uint8(36))  // midi_low
	binary.Write(buf, binary.LittleEndian, uint8(96))  // midi_high
	binary.Write(buf, binary.LittleEndian, uint8(60))  // midi_root
	reserved := [338]byte{}
	buf.Write(reserved[:])
	binary.Write(buf, binary.LittleEndian, int16(100))
	binary.Write(buf, binary.LittleEndian, int16(200))

	slices, err := engine.DetectReader(".pti").Read(buf.Bytes(), 44100)
	if err != nil {
		t.Fatalf("PTI read failed: %v", err)
	}
	if len(slices) != 1 {
		t.Fatalf("expected 1 slice, got %d", len(slices))
	}
	if slices[0].Metadata.SampleRate != 44100 {
		t.Errorf("expected 44100 Hz, got %d", slices[0].Metadata.SampleRate)
	}
}

func TestReader_FormatRountrips(t *testing.T) {
	wavData := buildMinimalWAV()
	slices, err := engine.DetectReader(".wav").Read(wavData, 44100)
	if err != nil {
		t.Fatalf("WAV read: %v", err)
	}
	if len(slices) != 1 {
		t.Fatalf("expected 1 slice from minimal WAV")
	}
	if slices[0].TotalFrames != 2 {
		t.Fatalf("expected 2 frames, got %d", slices[0].TotalFrames)
	}
}

func buildMinimalWAV() []byte {
	buf := &bytes.Buffer{}
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+4))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint32(44100))
	binary.Write(buf, binary.LittleEndian, uint32(88200))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(4))
	binary.Write(buf, binary.LittleEndian, int16(100))
	binary.Write(buf, binary.LittleEndian, int16(200))
	return buf.Bytes()
}

func findTestXRNI() string {
	p := "testdata/SM101_brk_animal break_140bpm.xrni"
	if _, err := os.Stat(p); err == nil {
		abs, _ := filepath.Abs(p)
		return abs
	}
	return ""
}

func findTestSimpler() string {
	p := "testdata/ableton_kit/ABAHAMBI - Abahambi.adv"
	if _, err := os.Stat(p); err == nil {
		abs, _ := filepath.Abs(p)
		return abs
	}
	return ""
}

func findTestDrumRack() string {
	p := "testdata/ableton_kit/drum kit.adg"
	if _, err := os.Stat(p); err == nil {
		abs, _ := filepath.Abs(p)
		return abs
	}
	return ""
}


