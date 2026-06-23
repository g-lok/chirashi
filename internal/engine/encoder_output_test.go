package engine

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/xml"
	"io"
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
		{"d2pst", 12},
	}
	for _, tt := range tests {
		got := fileNameLimit(tt.format)
		if got != tt.limit {
			t.Errorf("fileNameLimit(%q) = %d, want %d", tt.format, got, tt.limit)
		}
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
}
