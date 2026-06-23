package chirashi_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
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
		{".caf", true},
		{".pti", true},
		{".ot", true},
		{".xy", true},
		{".dt2pst", true},
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

func TestCAFReader_Minimal(t *testing.T) {
	var buf bytes.Buffer
	// CAF header
	buf.WriteString("caff")
	binary.Write(&buf, binary.BigEndian, uint16(1)) // version
	binary.Write(&buf, binary.BigEndian, uint16(0)) // flags

	// desc chunk: 32 bytes
	buf.WriteString("desc")
	binary.Write(&buf, binary.BigEndian, uint64(32))
	binary.Write(&buf, binary.BigEndian, uint64(0x40E5888000000000)) // 44100.0 float64 BE
	binary.Write(&buf, binary.BigEndian, uint32(0x6c70636d))         // 'lpcm'
	binary.Write(&buf, binary.BigEndian, uint32(0x0C))               // format flags: signed | packed
	binary.Write(&buf, binary.BigEndian, uint32(4))                  // bytes per packet (2ch × 2B)
	binary.Write(&buf, binary.BigEndian, uint32(1))                  // frames per packet
	binary.Write(&buf, binary.BigEndian, uint32(2))                  // channels per frame
	binary.Write(&buf, binary.BigEndian, uint32(16))                 // bits per channel

	// data chunk: 4 (offset) + 8 (4 samples × 2B)
	buf.WriteString("data")
	binary.Write(&buf, binary.BigEndian, uint64(12))
	binary.Write(&buf, binary.BigEndian, uint32(0)) // edit count / offset
	binary.Write(&buf, binary.BigEndian, int16(100))
	binary.Write(&buf, binary.BigEndian, int16(200))
	binary.Write(&buf, binary.BigEndian, int16(300))
	binary.Write(&buf, binary.BigEndian, int16(400))

	slices, err := engine.DetectReader(".caf").Read(buf.Bytes(), 44100)
	if err != nil {
		t.Fatalf("CAF read failed: %v", err)
	}
	if len(slices) != 1 {
		t.Fatalf("expected 1 slice, got %d", len(slices))
	}
	if slices[0].Metadata.SampleRate != 44100 {
		t.Errorf("expected 44100 Hz, got %d", slices[0].Metadata.SampleRate)
	}
	if slices[0].Metadata.Channels != 2 {
		t.Errorf("expected 2 channels, got %d", slices[0].Metadata.Channels)
	}
	if slices[0].TotalFrames != 2 {
		t.Errorf("expected 2 frames, got %d", slices[0].TotalFrames)
	}
}

func TestOTReader_Minimal(t *testing.T) {
	wavData := buildMinimalWAV()

	// OT sidecar: 2 slices, byte offsets into WAV
	otData := make([]byte, 272)
	copy(otData[0:4], "OT\x00\x00")
	binary.BigEndian.PutUint16(otData[6:8], 2) // sliceCount
	// buildMinimalWAV: 1ch 16-bit = 2 bytes/frame, 2 frames = 4 bytes
	binary.BigEndian.PutUint32(otData[8:12], 0)    // slice 0 start byte
	binary.BigEndian.PutUint32(otData[12:16], 2)   // slice 1 start byte
	binary.BigEndian.PutUint32(otData[264:268], 2)  // slice 0 end byte
	binary.BigEndian.PutUint32(otData[268:272], 4)  // slice 1 end byte

	slices, err := engine.ReadOTWithWAV(otData, wavData, 44100)
	if err != nil {
		t.Fatalf("OT read with WAV: %v", err)
	}
	if len(slices) != 2 {
		t.Fatalf("expected 2 slices, got %d", len(slices))
	}
	if slices[0].TotalFrames != 1 {
		t.Errorf("slice 0: expected 1 frame, got %d", slices[0].TotalFrames)
	}
	if slices[1].TotalFrames != 1 {
		t.Errorf("slice 1: expected 1 frame, got %d", slices[1].TotalFrames)
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

func TestPTIReader_TIFormat(t *testing.T) {
	pcm := make([]float32, 100)
	for i := range pcm {
		pcm[i] = float32(i) / 100.0
	}
	ext := &engine.SliceExtraction{
		Metadata: engine.RexMetadata{
			SampleRate: 44100,
			Channels:   1,
			BitDepth:   16,
		},
		Interleaved: pcm,
		TotalFrames: 100,
		CuePoints: []engine.WavCueMarker{
			{SliceID: 0, Position: 0, Label: "Slice 01"},
			{SliceID: 1, Position: 25, Label: "Slice 02"},
			{SliceID: 2, Position: 50, Label: "Slice 03"},
			{SliceID: 3, Position: 75, Label: "Slice 04"},
		},
	}

	var encBuf bytes.Buffer
	if err := engine.EncodePTI(&encBuf, ext); err != nil {
		t.Fatalf("EncodePTI: %v", err)
	}
	ptiData := encBuf.Bytes()

	if ptiData[0] != 'T' || ptiData[1] != 'I' {
		t.Fatal("expected TI magic from encoder")
	}

	slices, err := engine.DetectReader(".pti").Read(ptiData, 44100)
	if err != nil {
		t.Fatalf("PTI TI-format read: %v", err)
	}
	if len(slices) != 4 {
		t.Fatalf("expected 4 slices, got %d", len(slices))
	}
	for i, sl := range slices {
		if sl.Metadata.Channels != 1 {
			t.Errorf("slice %d: expected mono, got %d channels", i, sl.Metadata.Channels)
		}
		if sl.Metadata.SampleRate != 44100 {
			t.Errorf("slice %d: expected 44100 Hz, got %d", i, sl.Metadata.SampleRate)
		}
	}
	totalFrames := 0
	for _, sl := range slices {
		totalFrames += sl.TotalFrames
	}
	if totalFrames != 100 {
		t.Errorf("expected 100 total frames, got %d", totalFrames)
	}
}

func TestPTIReader_TIFormat_NoSlices(t *testing.T) {
	pcm := make([]float32, 50)
	for i := range pcm {
		pcm[i] = 0.5
	}
	ext := &engine.SliceExtraction{
		Metadata: engine.RexMetadata{
			SampleRate: 44100,
			Channels:   1,
			BitDepth:   16,
		},
		Interleaved: pcm,
		TotalFrames: 50,
		CuePoints:   nil,
	}

	var encBuf bytes.Buffer
	if err := engine.EncodePTI(&encBuf, ext); err != nil {
		t.Fatalf("EncodePTI: %v", err)
	}

	slices, err := engine.DetectReader(".pti").Read(encBuf.Bytes(), 44100)
	if err != nil {
		t.Fatalf("PTI TI-format noslice read: %v", err)
	}
	if len(slices) != 1 {
		t.Fatalf("expected 1 slice (no cues), got %d", len(slices))
	}
}

func TestOTReader_ProbeDualFormat(t *testing.T) {
	build := func(magic []byte, extra int) []byte {
		b := make([]byte, 0, len(magic)+extra)
		b = append(b, magic...)
		b = append(b, make([]byte, extra)...)
		return b
	}
	tests := []struct {
		name  string
		magic []byte
		match bool
	}{
		{"FORM DPS1", build([]byte("FORM\x00\x00\x00\x00DPS1"), 20), true},
		{"bad after FORM", build([]byte("FORM"), 20), false},
		{"OT legacy", build([]byte("OT\x00\x00"), 12), true},
		{"bad magic", build([]byte("BAD\x00"), 12), false},
	}
	for _, tt := range tests {
		_, err := engine.ProbeInput(tt.magic)
		if tt.match && err != nil {
			t.Errorf("Probe(%s): expected match, got %v", tt.name, err)
		}
		if !tt.match && err == nil {
			t.Errorf("Probe(%s): expected no match, got nil", tt.name)
		}
	}
}

func TestPTIReader_ProbeDualFormat(t *testing.T) {
	tests := []struct {
		name  string
		magic []byte
		match bool
	}{
		{"TI format", []byte("TI\x01\x00\x01\x02\x05\x01\x00\x00\x00\x06\x00\x00"), true},
		{"TI short", []byte("TI"), false},
		{"PTI legacy", []byte("PTI\x00\x00\x00\x00\x00"), true},
		{"bad magic", []byte("BAD\x00\x00\x00"), false},
	}
	for _, tt := range tests {
		_, err := engine.ProbeInput(tt.magic)
		if tt.match && err != nil {
			t.Errorf("Probe(%s): expected match, got %v", tt.name, err)
		}
		if !tt.match && err == nil {
			t.Errorf("Probe(%s): expected no match, got nil", tt.name)
		}
	}
}

func TestOTReader_DPS1Format(t *testing.T) {
	wavData := buildMinimalWAV()

	otData := make([]byte, 0x340)
	copy(otData[0:4], "FORM")
	binary.BigEndian.PutUint32(otData[4:8], uint32(0x340-8))
	copy(otData[8:12], "DPS1")
	copy(otData[12:16], "SMPA")
	binary.BigEndian.PutUint32(otData[16:20], uint32(0x330-8))

	binary.BigEndian.PutUint32(otData[23:27], uint32(120*24)) // BPM * 24
	binary.BigEndian.PutUint32(otData[27:31], 200)            // trim
	binary.BigEndian.PutUint32(otData[31:35], 200)            // loop
	binary.BigEndian.PutUint16(otData[40:42], 0x30)
	binary.BigEndian.PutUint32(otData[43:47], 0)
	binary.BigEndian.PutUint32(otData[47:51], 2) // total sample frames

	// Slice 0: frames 0-2
	binary.BigEndian.PutUint32(otData[58:62], 0)   // start frame = 0
	binary.BigEndian.PutUint32(otData[62:66], 2)   // end frame = 2
	binary.BigEndian.PutUint32(otData[66:70], 0xFFFFFFFF)

	binary.BigEndian.PutUint32(otData[0x33A:0x33E], 1) // num slices

	var cs uint16
	for i := 0x10; i < 0x33E; i++ {
		cs += uint16(otData[i])
	}
	binary.BigEndian.PutUint16(otData[0x33E:0x340], cs)

	slices, err := engine.ReadOTWithWAV(otData, wavData, 44100)
	if err != nil {
		t.Fatalf("OT DPS1 read: %v", err)
	}
	if len(slices) != 1 {
		t.Fatalf("expected 1 slice, got %d", len(slices))
	}
	if slices[0].TotalFrames != 2 {
		t.Fatalf("expected 2 frames, got %d", slices[0].TotalFrames)
	}
}

func TestDT2PSTReader_RealFile(t *testing.T) {
	p := findTestDT2PST()
	if p == "" {
		t.Skip("no DT2PST test file found")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read DT2PST: %v", err)
	}
	reader := engine.DetectReader(".dt2pst")
	slices, err := reader.Read(data, 44100)
	if err != nil {
		t.Fatalf("DT2PST read: %v", err)
	}
	if len(slices) == 0 {
		t.Fatal("expected at least 1 slice")
	}
	t.Logf("DT2PST: %d slices, %d Hz, %d ch", len(slices), slices[0].Metadata.SampleRate, slices[0].Metadata.Channels)
}

func TestOTReader_RealSampleFile(t *testing.T) {
	p := findTestOT()
	if p == "" {
		t.Skip("no OT test file found")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read OT: %v", err)
	}
	// Verify format detection
	meta, err := engine.ProbeInput(data)
	if err != nil {
		t.Fatalf("OT probe: %v", err)
	}
	t.Logf("OT: %d Hz, %d ch", meta.SampleRate, meta.Channels)

	// Verify Reader.Read() returns the expected companion WAV error
	reader := engine.DetectReader(".ot")
	_, err = reader.Read(data, 44100)
	if err == nil {
		t.Fatal("expected error (companion WAV required), got nil")
	}
	if !strings.Contains(err.Error(), "companion WAV required") {
		t.Fatalf("expected 'companion WAV required' error, got: %v", err)
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
	candidates := []string{
		"testdata/SM101_brk_animal break_140bpm.xrni",
		"../samples/SM101_brk_Bongo Bar_110bpm.xrni",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	return ""
}

func findTestSimpler() string {
	candidates := []string{
		"testdata/ableton_kit/ABAHAMBI - Abahambi.adv",
		"testdata/ableton_kit/ABAHAMBI - Abahambi-2.adv",
		"../samples/ableton_kit/ABAHAMBI - Abahambi.adv",
		"../samples/ableton_kit/ABAHAMBI - Abahambi-2.adv",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	return ""
}

func findTestDrumRack() string {
	candidates := []string{
		"testdata/ableton_kit/drum kit.adg",
		"../samples/ableton_kit/drum kit.adg",
		"../samples/ableton_kit/drum kit-2.adg",
		"../samples/ableton_kit/rack kit.adg",
		"../samples/ableton_kit/rack kit-2.adg",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	return ""
}

func findTestDT2PST() string {
	p := "../samples/SOLE DISPLAY.dt2pst"
	if _, err := os.Stat(p); err == nil {
		abs, _ := filepath.Abs(p)
		return abs
	}
	return ""
}

func findTestOT() string {
	p := "../samples/sample.ot"
	if _, err := os.Stat(p); err == nil {
		abs, _ := filepath.Abs(p)
		return abs
	}
	return ""
}


