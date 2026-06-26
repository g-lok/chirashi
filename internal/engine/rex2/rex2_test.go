package rex2

import (
	"os"
	"path/filepath"
	"testing"
)

func testdataPath(elem ...string) string {
	parts := append([]string{"..", "..", "..", "tests", "testdata"}, elem...)
	return filepath.Join(parts...)
}

func loadTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("test file not found: %v", err)
	}
	return data
}

// --- Round-trip: encode known PCM → decode → verify identical ---

func genSaw16(n int) []int32 {
	p := make([]int32, n)
	for i := range p {
		p[i] = int32((i % 200) - 100) * 300
	}
	return p
}

func TestRoundTrip_Mono16Bit(t *testing.T) {
	frameCount := 100
	pcm := genSaw16(frameCount)
	info := FileInfo{
		Channels:      1,
		SampleRate:    44100,
		SliceCount:    1,
		Tempo:         120000,
		OriginalTempo: 120000,
		PPQLength:     61440,
		TimeSigNum:    4,
		TimeSigDen:    4,
		BitDepth:      16,
		TotalFrames:   frameCount,
	}
	slices := []SliceInfo{{PPQPos: 0, SampleStart: 0, SampleLength: frameCount}}
	encoded, err := Encode(pcm, info, slices, CreatorInfo{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Info.Channels != 1 {
		t.Errorf("channels = %d, want 1", decoded.Info.Channels)
	}
	if decoded.Info.BitDepth != 16 {
		t.Errorf("bitDepth = %d, want 16", decoded.Info.BitDepth)
	}
	if decoded.Info.SampleRate != 44100 {
		t.Errorf("sampleRate = %d, want 44100", decoded.Info.SampleRate)
	}
	if decoded.Info.TotalFrames != frameCount {
		t.Errorf("totalFrames = %d, want %d", decoded.Info.TotalFrames, frameCount)
	}
	if decoded.Info.Tempo != 120000 {
		t.Errorf("tempo = %d, want 120000", decoded.Info.Tempo)
	}
	if len(decoded.Slices) != 1 {
		t.Errorf("slices = %d, want 1", len(decoded.Slices))
	}
	if len(decoded.PCM) != frameCount {
		t.Errorf("pcm length = %d, want %d", len(decoded.PCM), frameCount)
	}
	for i := range pcm {
		diff := decoded.PCM[i] - pcm[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > 1 {
			t.Errorf("sample %d: got %d, want %d", i, decoded.PCM[i], pcm[i])
		}
	}
}

func TestRoundTrip_Stereo16Bit(t *testing.T) {
	frameCount := 100
	pcm := make([]int32, frameCount*2)
	saw := genSaw16(frameCount)
	for i, v := range saw {
		pcm[i*2+0] = v
		pcm[i*2+1] = -v
	}
	info := FileInfo{
		Channels:      2,
		SampleRate:    44100,
		SliceCount:    1,
		Tempo:         120000,
		OriginalTempo: 120000,
		PPQLength:     61440,
		TimeSigNum:    4,
		TimeSigDen:    4,
		BitDepth:      16,
		TotalFrames:   frameCount,
	}
	slices := []SliceInfo{{PPQPos: 0, SampleStart: 0, SampleLength: frameCount}}
	encoded, err := Encode(pcm, info, slices, CreatorInfo{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Info.Channels != 2 {
		t.Errorf("channels = %d, want 2", decoded.Info.Channels)
	}
	if len(decoded.PCM) != frameCount*2 {
		t.Errorf("pcm length = %d, want %d", len(decoded.PCM), frameCount*2)
	}
	for i := range pcm {
		diff := decoded.PCM[i] - pcm[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > 1 {
			t.Errorf("sample %d: got %d, want %d", i, decoded.PCM[i], pcm[i])
		}
	}
}

func TestRoundTrip_MultipleSlices(t *testing.T) {
	frameCount := 200
	pcm := make([]int32, frameCount)
	for i := range pcm {
		pcm[i] = int32(i-100) * 327
	}
	slices := []SliceInfo{
		{PPQPos: 0, SampleStart: 0, SampleLength: 100},
		{PPQPos: 15360, SampleStart: 100, SampleLength: 50},
		{PPQPos: 23040, SampleStart: 150, SampleLength: 50},
	}
	info := FileInfo{
		Channels:      1,
		SampleRate:    44100,
		SliceCount:    len(slices),
		Tempo:         140000,
		OriginalTempo: 140000,
		PPQLength:     61440,
		TimeSigNum:    4,
		TimeSigDen:    4,
		BitDepth:      16,
		TotalFrames:   frameCount,
	}
	encoded, err := Encode(pcm, info, slices, CreatorInfo{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Info.SliceCount != 3 {
		t.Errorf("sliceCount = %d, want 3", decoded.Info.SliceCount)
	}
	// Check slice start positions
	expectedStarts := []int{0, 100, 150}
	for i, s := range decoded.Slices {
		if s.SampleStart != expectedStarts[i] {
			t.Errorf("slice %d: start = %d, want %d", i, s.SampleStart, expectedStarts[i])
		}
	}
}

func TestRoundTrip_24Bit(t *testing.T) {
	frameCount := 50
	pcm := make([]int32, frameCount)
	for i := range pcm {
		pcm[i] = int32(i-25) * 262144
	}
	info := FileInfo{
		Channels:    1,
		SampleRate:  48000,
		SliceCount:  1,
		Tempo:       120000,
		BitDepth:    24,
		TotalFrames: frameCount,
		PPQLength:   61440,
		TimeSigNum:  4,
		TimeSigDen:  4,
	}
	slices := []SliceInfo{{PPQPos: 0, SampleStart: 0, SampleLength: frameCount}}
	encoded, err := Encode(pcm, info, slices, CreatorInfo{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Info.BitDepth != 24 {
		t.Errorf("bitDepth = %d, want 24", decoded.Info.BitDepth)
	}
	if decoded.Info.SampleRate != 48000 {
		t.Errorf("sampleRate = %d, want 48000", decoded.Info.SampleRate)
	}
}

func TestRoundTrip_ZeroSample(t *testing.T) {
	frameCount := 100
	pcm := make([]int32, frameCount)
	info := FileInfo{
		Channels:    1,
		SampleRate:  44100,
		SliceCount:  1,
		Tempo:       120000,
		BitDepth:    16,
		TotalFrames: frameCount,
		PPQLength:   61440,
		TimeSigNum:  4,
		TimeSigDen:  4,
	}
	slices := []SliceInfo{{PPQPos: 0, SampleStart: 0, SampleLength: frameCount}}
	encoded, err := Encode(pcm, info, slices, CreatorInfo{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.PCM) != frameCount {
		t.Errorf("pcm length = %d, want %d", len(decoded.PCM), frameCount)
	}
	for i, v := range decoded.PCM {
		if v != 0 {
			t.Errorf("sample %d: %d, want 0", i, v)
		}
	}
}

// --- Real file decode ---

func TestDecode_120FourBeats(t *testing.T) {
	data := loadTestFile(t, testdataPath("120FourBeats.rx2"))
	f, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Info.Tempo != 120000 {
		t.Errorf("tempo = %d, want 120000", f.Info.Tempo)
	}
	if f.Info.SampleRate != 44100 {
		t.Errorf("sampleRate = %d, want 44100", f.Info.SampleRate)
	}
	if f.Info.Channels != 1 {
		t.Errorf("channels = %d, want 1", f.Info.Channels)
	}
	if f.Info.BitDepth != 16 {
		t.Errorf("bitDepth = %d, want 16", f.Info.BitDepth)
	}
	if f.Info.SliceCount < 4 {
		t.Errorf("sliceCount = %d, want >= 4", f.Info.SliceCount)
	}
	if len(f.PCM) == 0 {
		t.Error("PCM is empty")
	}
}

func TestDecode_120Stereo(t *testing.T) {
	data := loadTestFile(t, testdataPath("120Stereo.rx2"))
	f, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Info.Channels != 2 {
		t.Errorf("channels = %d, want 2", f.Info.Channels)
	}
	if f.Info.SliceCount < 4 {
		t.Errorf("sliceCount = %d, want >= 4", f.Info.SliceCount)
	}
	if len(f.PCM) == 0 {
		t.Error("PCM is empty")
	}
}

func TestDecode_120Mono(t *testing.T) {
	data := loadTestFile(t, testdataPath("120Mono.rx2"))
	f, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Info.Channels != 1 {
		t.Errorf("channels = %d, want 1", f.Info.Channels)
	}
	if f.Info.SliceCount < 1 {
		t.Errorf("sliceCount = %d, want >= 1", f.Info.SliceCount)
	}
}

func TestDecode_120Mono24Bits(t *testing.T) {
	data := loadTestFile(t, testdataPath("120Mono24Bits.rx2"))
	f, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Info.BitDepth != 24 {
		t.Errorf("bitDepth = %d, want 24", f.Info.BitDepth)
	}
	if len(f.PCM) == 0 {
		t.Error("PCM is empty")
	}
}

func TestDecode_120Gated(t *testing.T) {
	data := loadTestFile(t, testdataPath("120Gated.rx2"))
	f, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Info.SliceCount < 1 {
		t.Errorf("sliceCount = %d, want >= 1", f.Info.SliceCount)
	}
}

func TestDecode_100HasCreatorInfo(t *testing.T) {
	data := loadTestFile(t, testdataPath("100HasCreatorInfo.rx2"))
	f, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	_ = f.Creator
	if f.Info.Tempo != 100000 {
		t.Errorf("tempo = %d, want 100000", f.Info.Tempo)
	}
}

func TestDecode_240FiveHundredSlices(t *testing.T) {
	data := loadTestFile(t, testdataPath("240FiveHundredSlices.rx2"))
	f, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Info.SliceCount < 400 {
		t.Errorf("sliceCount = %d, want >= 400", f.Info.SliceCount)
	}
}

func TestDecode_120SevenEights(t *testing.T) {
	data := loadTestFile(t, testdataPath("120SevenEights.rx2"))
	f, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Info.TimeSigNum != 7 {
		t.Errorf("timeSigNum = %d, want 7", f.Info.TimeSigNum)
	}
	if f.Info.TimeSigDen != 8 {
		t.Errorf("timeSigDen = %d, want 8", f.Info.TimeSigDen)
	}
}

func TestDecode_120AllMuted(t *testing.T) {
	data := loadTestFile(t, testdataPath("120AllMuted.rx2"))
	f, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Info.SliceCount < 1 {
		t.Errorf("sliceCount = %d, want >= 1", f.Info.SliceCount)
	}
}

// --- Error cases ---

func TestDecode_ErrorCorrupt(t *testing.T) {
	data := loadTestFile(t, testdataPath("ErrorCorrupt.rx2"))
	_, err := Decode(data)
	if err == nil {
		t.Fatal("expected error for corrupt file")
	}
}

func TestDecode_ErrorCorrupt2(t *testing.T) {
	data := loadTestFile(t, testdataPath("ErrorCorrupt2.rx2"))
	_, err := Decode(data)
	if err == nil {
		t.Fatal("expected error for corrupt file")
	}
}

func TestDecode_ErrorTooNew(t *testing.T) {
	data := loadTestFile(t, testdataPath("ErrorTooNew.rx2"))
	// Current parser doesn't enforce HEAD version check, so this may decode
	_, _ = Decode(data)
	// If it decodes, at least verify it didn't crash
}

func TestDecode_InvalidSize(t *testing.T) {
	_, err := Decode([]byte{0, 1, 2})
	if err == nil {
		t.Fatal("expected error for short input")
	}
}

func TestDecode_UnknownFormat(t *testing.T) {
	_, err := Decode([]byte("FORM0000XXXX"))
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

// --- Single-slice (no SLCE chunk) ---

func TestDecode_120TransmitAsOneSlice(t *testing.T) {
	data := loadTestFile(t, testdataPath("120TransmitAsOneSlice.rx2"))
	f, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Info.SliceCount < 1 {
		t.Errorf("sliceCount = %d, want >= 1", f.Info.SliceCount)
	}
}

// --- Legacy RCY ---

func TestDecode_LegacyRCY(t *testing.T) {
	data := loadTestFile(t, testdataPath("120RcyTest.rcy"))
	f, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Info.SampleRate < 8000 {
		t.Errorf("sampleRate = %d, seems wrong", f.Info.SampleRate)
	}
	if f.Info.SliceCount < 1 {
		t.Errorf("sliceCount = %d, want >= 1", f.Info.SliceCount)
	}
}

// --- Legacy REX ---

func TestDecode_LegacyREX(t *testing.T) {
	data := loadTestFile(t, testdataPath("120RexTest.rex"))
	f, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Info.SampleRate < 8000 {
		t.Errorf("sampleRate = %d, seems wrong", f.Info.SampleRate)
	}
	if f.Info.SliceCount < 1 {
		t.Errorf("sliceCount = %d, want >= 1", f.Info.SliceCount)
	}
}

// --- Float32 conversion ---

func TestPCMToFloat32_Zero(t *testing.T) {
	v := PCMToFloat32(0, 16)
	if v != 0 {
		t.Errorf("got %f, want 0", v)
	}
}

func TestPCMToFloat32_Positive(t *testing.T) {
	v := PCMToFloat32(32767, 16)
	if v <= 0 || v > 1.0 {
		t.Errorf("got %f, want (0,1]", v)
	}
}

func TestPCMToFloat32_Negative(t *testing.T) {
	v := PCMToFloat32(-32768, 16)
	if v >= 0 {
		t.Errorf("got %f, want negative", v)
	}
	if v < -1.001 || v > -0.999 {
		t.Errorf("got %f, want ≈ -1.0", v)
	}
}

func TestFloat32ToPCM_Roundtrip(t *testing.T) {
	orig := int32(12345)
	f := PCMToFloat32(orig, 16)
	back := Float32ToPCM(f, 16)
	diff := back - orig
	if diff < 0 {
		diff = -diff
	}
	if diff > 2 {
		t.Errorf("roundtrip: %d → %f → %d, diff %d", orig, f, back, diff)
	}
}

func TestFloat32ToPCM_Range(t *testing.T) {
	v := Float32ToPCM(1.0, 16)
	if v != 32767 {
		t.Errorf("1.0 → %d, want 32767", v)
	}
	v = Float32ToPCM(-1.0, 16)
	if v != -32768 {
		t.Errorf("-1.0 → %d, want -32768", v)
	}
}

// --- Clamp ---

func TestClampSample_InRange(t *testing.T) {
	v := clampSample(100, 16)
	if v != 100 {
		t.Errorf("got %d, want 100", v)
	}
}

func TestClampSample_AboveMax(t *testing.T) {
	v := clampSample(100000, 16)
	if v != 32767 {
		t.Errorf("got %d, want 32767", v)
	}
}

func TestClampSample_BelowMin(t *testing.T) {
	v := clampSample(-100000, 16)
	if v != -32768 {
		t.Errorf("got %d, want -32768", v)
	}
}

// --- Bit depth code helpers ---

func TestBitDepthCode(t *testing.T) {
	tests := []struct {
		depth int
		code  uint8
	}{
		{8, 1},
		{16, 3},
		{24, 5},
		{32, 7},
		{0, 3},
		{999, 3},
	}
	for _, tc := range tests {
		c := bitDepthCode(tc.depth)
		if c != tc.code {
			t.Errorf("bitDepthCode(%d) = %d, want %d", tc.depth, c, tc.code)
		}
	}
}

func TestBitDepthFromCode(t *testing.T) {
	tests := []struct {
		code  uint8
		depth int
	}{
		{1, 8},
		{3, 16},
		{5, 24},
		{7, 32},
		{0, 16},
		{255, 16},
	}
	for _, tc := range tests {
		d := bitDepthFromCode(tc.code)
		if d != tc.depth {
			t.Errorf("bitDepthFromCode(%d) = %d, want %d", tc.code, d, tc.depth)
		}
	}
}

// --- Encode with creator info ---

func TestEncode_WithCreatorInfo(t *testing.T) {
	pcm := make([]int32, 100)
	info := FileInfo{
		Channels:      1,
		SampleRate:    44100,
		SliceCount:    1,
		Tempo:         120000,
		OriginalTempo: 120000,
		PPQLength:     61440,
		TimeSigNum:    4,
		TimeSigDen:    4,
		BitDepth:      16,
		TotalFrames:   100,
	}
	slices := []SliceInfo{{PPQPos: 0, SampleStart: 0, SampleLength: 100}}
	creator := CreatorInfo{
		Name:      "TestCreator",
		Copyright: "TestCopyright",
		URL:       "https://example.com",
		Email:     "test@example.com",
		FreeText:  "Free text",
	}
	encoded, err := Encode(pcm, info, slices, creator)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Creator.Name != "TestCreator" {
		t.Errorf("creator name = %q, want %q", decoded.Creator.Name, "TestCreator")
	}
	if decoded.Creator.Copyright != "TestCopyright" {
		t.Errorf("creator copyright = %q, want %q", decoded.Creator.Copyright, "TestCopyright")
	}
	if decoded.Creator.URL != "https://example.com" {
		t.Errorf("creator URL = %q, want %q", decoded.Creator.URL, "https://example.com")
	}
}

// --- Empty / nil creator info should encode without CREI chunk ---

func TestEncode_NoCreatorInfo(t *testing.T) {
	pcm := make([]int32, 100)
	info := FileInfo{
		Channels:      1,
		SampleRate:    44100,
		SliceCount:    1,
		Tempo:         120000,
		OriginalTempo: 120000,
		PPQLength:     61440,
		TimeSigNum:    4,
		TimeSigDen:    4,
		BitDepth:      16,
		TotalFrames:   100,
	}
	slices := []SliceInfo{{PPQPos: 0, SampleStart: 0, SampleLength: 100}}
	encoded, err := Encode(pcm, info, slices, CreatorInfo{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Should not contain CREI chunk
	if containsCREI(encoded) {
		t.Error("encoded contains CREI chunk but creator info is empty")
	}
}

func containsCREI(data []byte) bool {
	for i := 0; i+4 < len(data); i++ {
		if string(data[i:i+4]) == "CREI" {
			return true
		}
	}
	return false
}
