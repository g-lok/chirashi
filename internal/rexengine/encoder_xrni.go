package rexengine

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
)

func EncodeXRNI(w io.Writer, extraction *SliceExtraction, name string) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode XRNI: extraction data is empty")
	}

	slices := splitExtractionIntoSlices(extraction)
	if len(slices) > 128 {
		slices = slices[:128]
	}

	baseNote := 36

	monoWAV, err := monolithicWAV(extraction)
	if err != nil {
		return err
	}

	pcmBytes := make([]byte, len(extraction.Interleaved)*4)
	for i, v := range extraction.Interleaved {
		binary.LittleEndian.PutUint32(pcmBytes[i*4:], math.Float32bits(v))
	}
	hash := sha256.Sum256(pcmBytes)
	audioName := hex.EncodeToString(hash[:8]) + ".wav"

	instXML := buildXRNIInstrumentXML(name, slices, audioName, baseNote)

	zw := zip.NewWriter(w)

	h := &zip.FileHeader{Name: "Instrument.xml", Method: zip.Deflate}
	fw, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	if _, err := fw.Write([]byte(instXML)); err != nil {
		return err
	}

	h2 := &zip.FileHeader{Name: audioName, Method: zip.Store}
	fw2, err := zw.CreateHeader(h2)
	if err != nil {
		return err
	}
	if _, err := fw2.Write(monoWAV); err != nil {
		return err
	}

	return zw.Close()
}

func monolithicWAV(extraction *SliceExtraction) ([]byte, error) {
	var buf bytes.Buffer
	if err := EncodeWavContainer(&writeSeekBuffer{Buffer: &buf}, extraction, 16); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildXRNIInstrumentXML(name string, slices []SliceExtraction, audioName string, baseNote int) string {
	if name == "" {
		name = "Instrument"
	}

	totalFrames := 0
	for _, s := range slices {
		totalFrames += s.TotalFrames
	}

	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<RenoiseInstrument doc_version="34">
  <Name>%s</Name>
  <SampleGenerator><Samples>
    <Sample>
      <IsAlias>false</IsAlias>
      <FileName>//File:%s</FileName>
      <SliceMarkers>`, escapeXML(name), escapeXML(audioName))

	frameOffset := 0
	for _, s := range slices {
		frameOffset += s.TotalFrames
		xml += fmt.Sprintf(`
        <SliceMarker><SamplePosition>%d</SamplePosition></SliceMarker>`, frameOffset)
	}

	xml += fmt.Sprintf(`
      </SliceMarkers>
      <Mapping><BaseNote>%d</BaseNote></Mapping>
    </Sample>`, baseNote)

	for i, s := range slices {
		note := baseNote + i
		loopEnd := s.TotalFrames
		if loopEnd < 1 {
			loopEnd = 1
		}
		xml += fmt.Sprintf(`
    <Sample>
      <IsAlias>true</IsAlias>
      <Mapping><BaseNote>%d</BaseNote></Mapping>
      <LoopEnd>%d</LoopEnd>
    </Sample>`, note, loopEnd)
	}

	xml += `
  </Samples></SampleGenerator>
</RenoiseInstrument>`

	return xml
}

func escapeXML(s string) string {
	var buf bytes.Buffer
	for _, r := range s {
		switch r {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '"':
			buf.WriteString("&quot;")
		case '\'':
			buf.WriteString("&apos;")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
