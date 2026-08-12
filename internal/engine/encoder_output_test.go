package engine

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/xml"
	"io"
	"math"
	"strings"
	"testing"
)

func testExtraction() *SliceExtraction {
	pcm := make([]float32, 44100*2) // 1 sec stereo @ 44100
	for i := range pcm {
		pcm[i] = 0.5
	}
	return &SliceExtraction{
		Metadata: RexMetadata{
			SampleRate: 44100,
			Channels:   2,
			BitDepth:   16,
		},
		Interleaved: pcm,
		TotalFrames: 44100,
		CuePoints: []WavCueMarker{
			{SliceID: 0, Position: 0, Label: "Slice 01"},
			{SliceID: 1, Position: 22050, Label: "Slice 02"},
		},
	}
}

func TestEncodeAIFF(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeAIFF(&buf, ext); err != nil {
		t.Fatalf("EncodeAIFF: %v", err)
	}
	data := buf.Bytes()
	if string(data[:4]) != "FORM" {
		t.Fatal("bad FORM magic")
	}
	if string(data[8:12]) != "AIFF" {
		t.Fatal("bad AIFF form type")
	}
	if !bytes.Contains(data, []byte("COMM")) {
		t.Fatal("missing COMM chunk")
	}
	if !bytes.Contains(data, []byte("SSND")) {
		t.Fatal("missing SSND chunk")
	}
	if !bytes.Contains(data, []byte("MARK")) {
		t.Fatal("missing MARK chunk (2 cue points)")
	}
}

func TestEncodeAIFF_NoCues(t *testing.T) {
	ext := testExtraction()
	ext.CuePoints = nil
	var buf bytes.Buffer
	if err := EncodeAIFF(&buf, ext); err != nil {
		t.Fatalf("EncodeAIFF: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("MARK")) {
		t.Fatal("MARK chunk present but no cue points")
	}
}

func TestEncodeXRNI(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeXRNI(&buf, ext, "test_instr"); err != nil {
		t.Fatalf("EncodeXRNI: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}
	foundXML := false
	foundWAV := false
	for _, f := range zr.File {
		if f.Name == "Instrument.xml" {
			foundXML = true
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			if !strings.Contains(string(data), "<RenoiseInstrument") {
				t.Fatal("Instrument.xml missing RenoiseInstrument element")
			}
			if !strings.Contains(string(data), "<SliceMarker>") {
				t.Fatal("Instrument.xml missing slice markers")
			}
		}
		if strings.HasSuffix(f.Name, ".wav") {
			foundWAV = true
		}
	}
	if !foundXML {
		t.Fatal("missing Instrument.xml in XRNI")
	}
	if !foundWAV {
		t.Fatal("missing WAV in XRNI")
	}
}

func TestEncodeSimplerADV(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeSimplerADV(&buf, ext, "Samples/Imported/test.wav"); err != nil {
		t.Fatalf("EncodeSimplerADV: %v", err)
	}
	data := buf.Bytes()
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		t.Fatal("not GZip")
	}
	gr, _ := gzip.NewReader(bytes.NewReader(data))
	defer gr.Close()
	xmlData, _ := io.ReadAll(gr)

	var doc struct {
		XMLName xml.Name `xml:"Ableton"`
		Simpler struct{} `xml:"OriginalSimpler"`
	}
	if err := xml.Unmarshal(xmlData, &doc); err != nil {
		t.Fatalf("not valid Ableton XML: %v", err)
	}
	if doc.XMLName.Local != "Ableton" {
		t.Fatal("root element not Ableton")
	}
}

func TestEncodeSimplerALS(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeSimplerALS(&buf, ext, "Samples/Imported/test.wav"); err != nil {
		t.Fatalf("EncodeSimplerALS: %v", err)
	}

	gr, _ := gzip.NewReader(&buf)
	defer gr.Close()
	xmlData, _ := io.ReadAll(gr)

	var doc struct {
		XMLName xml.Name `xml:"Ableton"`
		LiveSet struct{} `xml:"LiveSet"`
	}
	if err := xml.Unmarshal(xmlData, &doc); err != nil {
		t.Fatalf("not valid ALS XML: %v", err)
	}
	if doc.XMLName.Local != "Ableton" {
		t.Fatal("root element not Ableton")
	}
}

func TestEncodeDrumRackADG(t *testing.T) {
	ext := testExtraction()
	slices := splitExtractionIntoSlices(ext)

	var buf bytes.Buffer
	paths := []string{"Samples/Imported/pad_01.wav", "Samples/Imported/pad_02.wav"}
	if err := EncodeDrumRackADG(&buf, slices, paths, 36); err != nil {
		t.Fatalf("EncodeDrumRackADG: %v", err)
	}

	gr, _ := gzip.NewReader(&buf)
	defer gr.Close()
	xmlData, _ := io.ReadAll(gr)

	var doc struct {
		XMLName     xml.Name `xml:"Ableton"`
		GroupDevice struct{} `xml:"GroupDevicePreset"`
	}
	if err := xml.Unmarshal(xmlData, &doc); err != nil {
		t.Fatalf("not valid ADG XML: %v", err)
	}

	if !strings.Contains(string(xmlData), "DrumBranchPreset") {
		t.Fatal("missing DrumBranchPreset")
	}
	if !strings.Contains(string(xmlData), "DrumCell") {
		t.Fatal("missing DrumCell")
	}
	if !strings.Contains(string(xmlData), "ReceivingNote=\"36\"") {
		t.Fatal("missing base MIDI note")
	}
}

func TestEncodeCAF_RoundTrip(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeCAF(&buf, ext); err != nil {
		t.Fatal(err)
	}

	reader := &CAFReader{}
	slices, err := reader.Read(buf.Bytes(), 44100)
	if err != nil {
		t.Fatalf("CAF roundtrip read: %v", err)
	}
	if len(slices) != 2 {
		t.Fatalf("expected 2 slices, got %d", len(slices))
	}
	if slices[0].Metadata.SampleRate != 44100 {
		t.Fatalf("expected 44100 sample rate, got %d", slices[0].Metadata.SampleRate)
	}
	if slices[0].Metadata.Channels != 2 {
		t.Fatalf("expected 2 channels, got %d", slices[0].Metadata.Channels)
	}
	if slices[0].Metadata.BitDepth != 16 {
		t.Fatalf("expected 16 bit depth, got %d", slices[0].Metadata.BitDepth)
	}
	// Verify total frames preserved
	totalFrames := 0
	for _, s := range slices {
		totalFrames += s.TotalFrames
	}
	if totalFrames != ext.TotalFrames {
		t.Fatalf("expected %d total frames, got %d", ext.TotalFrames, totalFrames)
	}
	// Verify PCM data preserved within rounding
	allPCM := make([]float32, 0, ext.TotalFrames*2)
	for _, s := range slices {
		allPCM = append(allPCM, s.Interleaved...)
	}
	for i := range ext.Interleaved {
		orig := int16(ext.Interleaved[i] * 32768)
		got := int16(allPCM[i] * 32768)
		if orig != got {
			t.Fatalf("sample %d: expected %d, got %d", i, orig, got)
		}
	}
}

func TestEncodeAIFF_RoundTrip(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeAIFF(&buf, ext); err != nil {
		t.Fatal(err)
	}

	reader := &AIFFReader{}
	slices, err := reader.Read(buf.Bytes(), 44100)
	if err != nil {
		t.Fatalf("AIFF roundtrip read: %v", err)
	}
	if len(slices) != 2 {
		t.Fatalf("expected 2 slices, got %d", len(slices))
	}
	if slices[0].Metadata.SampleRate != 44100 {
		t.Fatalf("expected 44100 sample rate, got %d", slices[0].Metadata.SampleRate)
	}
	if slices[0].Metadata.Channels != 2 {
		t.Fatalf("expected 2 channels, got %d", slices[0].Metadata.Channels)
	}
	// Verify total frames preserved
	totalFrames := 0
	for _, s := range slices {
		totalFrames += s.TotalFrames
	}
	if totalFrames != ext.TotalFrames {
		t.Fatalf("expected %d total frames, got %d", ext.TotalFrames, totalFrames)
	}
	// Verify PCM data preserved within rounding
	allPCM := make([]float32, 0, ext.TotalFrames*2)
	for _, s := range slices {
		allPCM = append(allPCM, s.Interleaved...)
	}
	for i := range ext.Interleaved {
		orig := int16(ext.Interleaved[i] * 32768)
		got := int16(allPCM[i] * 32768)
		if orig != got {
			t.Fatalf("sample %d: expected %d, got %d", i, orig, got)
		}
	}
}

func TestEncodeXRNI_RoundTrip(t *testing.T) {
	ext := testExtraction()
	var xrniBuf bytes.Buffer
	if err := EncodeXRNI(&xrniBuf, ext, "roundtrip"); err != nil {
		t.Fatal(err)
	}

	reader := &XRNIReader{}
	slices, err := reader.Read(xrniBuf.Bytes(), 44100)
	if err != nil {
		t.Fatalf("XRNI roundtrip read: %v", err)
	}
	if len(slices) != 2 {
		t.Fatalf("expected 2 slices, got %d", len(slices))
	}
}

func TestEncodeCAF(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeCAF(&buf, ext); err != nil {
		t.Fatalf("EncodeCAF: %v", err)
	}
	data := buf.Bytes()
	if string(data[:4]) != "caff" {
		t.Fatal("bad caff magic")
	}
	ver := binary.BigEndian.Uint16(data[4:6])
	if ver != 1 {
		t.Fatalf("expected version 1, got %d", ver)
	}
	if !bytes.Contains(data, []byte("desc")) {
		t.Fatal("missing desc chunk")
	}
	if !bytes.Contains(data, []byte("data")) {
		t.Fatal("missing data chunk")
	}
	if !bytes.Contains(data, []byte("info")) {
		t.Fatal("missing info chunk")
	}
	if !bytes.Contains(data, appleLoopMetaUUID) {
		t.Fatal("missing Apple Loop metadata UUID chunk")
	}
	if !bytes.Contains(data, appleLoopBeatMarkersUUID) {
		t.Fatal("missing Apple Loop beat markers UUID chunk")
	}
}

func TestEncodeCAF_NoCues(t *testing.T) {
	ext := testExtraction()
	ext.CuePoints = nil
	var buf bytes.Buffer
	if err := EncodeCAF(&buf, ext); err != nil {
		t.Fatalf("EncodeCAF: %v", err)
	}
	data := buf.Bytes()
	if !bytes.Contains(data, []byte("caff")) {
		t.Fatal("bad caff magic")
	}
	if !bytes.Contains(data, appleLoopBeatMarkersUUID) {
		t.Fatal("beat markers should still be written with end position")
	}
}

func TestEncodeCAF_16BitPCM(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeCAF(&buf, ext); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()

	descPos := bytes.Index(data, []byte("desc"))
	if descPos < 0 {
		t.Fatal("desc not found")
	}
	descSize := int64(binary.BigEndian.Uint64(data[descPos+4 : descPos+12]))
	if descSize < 32 {
		t.Fatalf("desc size %d", descSize)
	}
	sampleRateBits := binary.BigEndian.Uint64(data[descPos+12 : descPos+20])
	sr := int(math.Float64frombits(sampleRateBits))
	if sr != 44100 {
		t.Fatalf("expected 44100, got %d", sr)
	}
	formatID := string(data[descPos+20 : descPos+24])
	if formatID != "lpcm" {
		t.Fatalf("expected lpcm, got %s", formatID)
	}
	channels := int(binary.BigEndian.Uint32(data[descPos+36 : descPos+40]))
	if channels != 2 {
		t.Fatalf("expected 2 channels, got %d", channels)
	}
	bitDepth := int(binary.BigEndian.Uint32(data[descPos+40 : descPos+44]))
	if bitDepth != 16 {
		t.Fatalf("expected 16 bits, got %d", bitDepth)
	}
}

func TestEncodeAIFF_16BitPCM(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeAIFF(&buf, ext); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()

	commPos := bytes.Index(data, []byte("COMM"))
	if commPos < 0 {
		t.Fatal("COMM not found")
	}
	commSize := int(binary.BigEndian.Uint32(data[commPos+4 : commPos+8]))
	if commSize < 18 {
		t.Fatalf("COMM size %d", commSize)
	}
	channels := int(binary.BigEndian.Uint16(data[commPos+8 : commPos+10]))
	if channels != 2 {
		t.Fatalf("expected 2 channels, got %d", channels)
	}
	frameCount := int(binary.BigEndian.Uint32(data[commPos+10 : commPos+14]))
	if frameCount != 44100 {
		t.Fatalf("expected 44100 frames, got %d", frameCount)
	}
	bits := int(binary.BigEndian.Uint16(data[commPos+14 : commPos+16]))
	if bits != 16 {
		t.Fatalf("expected 16 bits, got %d", bits)
	}
}

func TestFileNameLimit(t *testing.T) {
	tests := []struct {
		format string
		limit  int
	}{
		{"xrni", 31},
		{"aif", 8},
		{"aif-op1", 8},
		{"adv", 255},
		{"als", 255},
		{"adg", 255},
		{"dt2pst", 12},
	}
	for _, tt := range tests {
		got := fileNameLimit(tt.format)
		if got != tt.limit {
			t.Errorf("fileNameLimit(%q) = %d, want %d", tt.format, got, tt.limit)
		}
	}
}

func TestEncodeDT2_RoundTrip(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeDT2Preset(&buf, ext, "test_rt"); err != nil {
		t.Fatal(err)
	}

	reader := &DT2PSTReader{}
	slices, err := reader.Read(buf.Bytes(), 44100)
	if err != nil {
		t.Fatalf("DT2PST roundtrip read: %v", err)
	}
	if len(slices) != 2 {
		t.Fatalf("expected 2 slices, got %d", len(slices))
	}
	if slices[0].Metadata.SampleRate != 44100 {
		t.Fatalf("expected 44100, got %d", slices[0].Metadata.SampleRate)
	}
	if slices[0].Metadata.Channels != 2 {
		t.Fatalf("expected 2 channels, got %d", slices[0].Metadata.Channels)
	}
	totalFrames := 0
	for _, s := range slices {
		totalFrames += s.TotalFrames
	}
	if totalFrames != ext.TotalFrames {
		t.Fatalf("expected %d total frames, got %d", ext.TotalFrames, totalFrames)
	}
}

func TestEncodeXY_RoundTrip(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeXYPreset(&buf, ext); err != nil {
		t.Fatal(err)
	}

	reader := &XYReader{}
	slices, err := reader.Read(buf.Bytes(), 44100)
	if err != nil {
		t.Fatalf("XY roundtrip read: %v", err)
	}
	if len(slices) != 2 {
		t.Fatalf("expected 2 slices, got %d", len(slices))
	}
	if slices[0].Metadata.Channels != 2 {
		t.Fatalf("expected 2 channels, got %d", slices[0].Metadata.Channels)
	}
	// XY stores each slice as individual WAV; total sample count preserved
	totalSamples := 0
	for _, s := range slices {
		totalSamples += len(s.Interleaved)
	}
	if totalSamples != len(ext.Interleaved) {
		t.Fatalf("expected %d total samples, got %d", len(ext.Interleaved), totalSamples)
	}
}

func TestEncodeOP1_Basic(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeOP1AIF(&buf, ext); err != nil {
		t.Fatalf("EncodeOP1AIF: %v", err)
	}
	data := buf.Bytes()
	if string(data[:4]) != "FORM" {
		t.Fatal("bad FORM magic")
	}
	if string(data[8:12]) != "AIFF" {
		t.Fatal("bad AIFF form type")
	}
	if !bytes.Contains(data, []byte("COMM")) {
		t.Fatal("missing COMM chunk")
	}
	if !bytes.Contains(data, []byte("SSND")) {
		t.Fatal("missing SSND chunk")
	}
	if !bytes.Contains(data, []byte("APPL")) {
		t.Fatal("missing APPL chunk")
	}
	if !bytes.Contains(data, []byte("op-1")) {
		t.Fatal("missing op-1 APPL signature")
	}
}

func TestEncodeEL_Basic(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeEL(&buf, ext); err != nil {
		t.Fatalf("EncodeEL: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "ELEKTRON MULTI-SAMPLE") {
		t.Fatal("missing header")
	}
	if !strings.Contains(output, "[[key-zones]]") {
		t.Fatal("missing key-zones")
	}
	if !strings.Contains(output, "slice_01.wav") {
		t.Fatal("missing slice_01.wav")
	}
	if !strings.Contains(output, "slice_02.wav") {
		t.Fatal("missing slice_02.wav")
	}
}

func TestEncodePTI_Basic(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodePTI(&buf, ext); err != nil {
		t.Fatalf("EncodePTI: %v", err)
	}
	data := buf.Bytes()
	if len(data) < 392 {
		t.Fatalf("PTI header too short: %d", len(data))
	}
	if data[0] != 'T' || data[1] != 'I' {
		t.Fatal("bad PTI magic")
	}
	if data[4] != 1 {
		t.Fatal("expected version 1")
	}
	// 392-byte header + PCM samples (44100*2 stereo int16)
	expectedPCM := 44100 * 2 * 2 // 44100 frames * 2 ch * 2 bytes
	if len(data)-392 != expectedPCM {
		t.Fatalf("expected %d PCM bytes, got %d", expectedPCM, len(data)-392)
	}
}

func TestEncodeOT_Basic(t *testing.T) {
	ext := testExtraction()
	var buf bytes.Buffer
	if err := EncodeOT(&buf, ext, 140.0); err != nil {
		t.Fatalf("EncodeOT: %v", err)
	}
	data := buf.Bytes()
	if string(data[:4]) != "FORM" {
		t.Fatal("bad FORM magic")
	}
	if string(data[8:12]) != "DPS1" {
		t.Fatal("bad DPS1 form type")
	}
	if len(data) != 0x340 {
		t.Fatalf("expected 0x340 byte OT file, got %d", len(data))
	}
}

func TestDeviceMaxSlices(t *testing.T) {
	if deviceMaxSlices["xrni"] != 128 {
		t.Fatal("xrni max slices should be 128")
	}
	if deviceMaxSlices["adg"] != 128 {
		t.Fatal("adg max slices should be 128")
	}
	if deviceMaxSlices["aif"] != 0 {
		t.Fatal("aif should have unlimited slices")
	}
	if deviceMaxSlices["adv"] != 0 {
		t.Fatal("adv should have unlimited slices")
	}
	if deviceMaxSlices["pti"] != 48 {
		t.Fatal("pti max slices should be 48")
	}
}
