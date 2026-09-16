package engine

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

type bitwigMultisampleXML struct {
	XMLName     xml.Name        `xml:"multisample"`
	Name        string          `xml:"name,attr"`
	Generator   string          `xml:"generator"`
	Category    string          `xml:"category,omitempty"`
	Creator     string          `xml:"creator,omitempty"`
	Description string          `xml:"description,omitempty"`
	Samples     []bitwigSample  `xml:"sample"`
}

type bitwigSample struct {
	File        string         `xml:"file,attr"`
	Gain        string         `xml:"gain,attr"`
	SampleStart int            `xml:"sample-start,attr"`
	SampleStop  int            `xml:"sample-stop,attr"`
	Key         bitwigKey      `xml:"key"`
	Velocity    bitwigVelocity `xml:"velocity"`
	Loop        bitwigLoop     `xml:"loop"`
}

type bitwigKey struct {
	Root     int    `xml:"root,attr"`
	Tune     string `xml:"tune,attr"`
	Track    string `xml:"track,attr"`
	Low      int    `xml:"low,attr"`
	High     int    `xml:"high,attr"`
	LowFade  string `xml:"low-fade,attr"`
	HighFade string `xml:"high-fade,attr"`
}

type bitwigVelocity struct {
	Low      int    `xml:"low,attr"`
	High     int    `xml:"high,attr"`
	LowFade  string `xml:"low-fade,attr"`
	HighFade string `xml:"high-fade,attr"`
}

type bitwigLoop struct {
	Mode  string `xml:"mode,attr"`
	Start int    `xml:"start,attr"`
	Stop  int    `xml:"stop,attr"`
}

func EncodeMultisample(w io.Writer, extraction *SliceExtraction, name, category string, targetBitDepth int) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode multisample: extraction data is empty")
	}

	if name == "" {
		name = "chirashi"
	}
	if category == "" {
		category = "chirashi"
	}

	slices := splitExtractionIntoSlices(extraction)
	if len(slices) == 0 {
		slices = []SliceExtraction{*extraction}
	}

	zw := zip.NewWriter(w)

	msXML := bitwigMultisampleXML{
		Name:      name,
		Generator: "chirashi",
		Category:  category,
		Creator:   "chirashi",
		Samples:   make([]bitwigSample, len(slices)),
	}

	baseNote := 24 // Start at C1

	for i, s := range slices {
		sliceName := fmt.Sprintf("slice_%02d.wav", i+1)
		midiNote := baseNote + i
		if midiNote > 127 {
			midiNote = 127
		}

		totalFrames := s.TotalFrames
		if totalFrames == 0 && s.Metadata.SampleRate > 0 && s.Metadata.Channels > 0 {
			totalFrames = len(s.Interleaved) / s.Metadata.Channels
		}

		msXML.Samples[i] = bitwigSample{
			File:        sliceName,
			Gain:        "0.000",
			SampleStart: 0,
			SampleStop:  totalFrames,
			Key: bitwigKey{
				Root:     midiNote,
				Tune:     "0.00",
				Track:    "1",
				Low:      midiNote,
				High:     midiNote,
				LowFade:  "0",
				HighFade: "0",
			},
			Velocity: bitwigVelocity{
				Low:      1,
				High:     127,
				LowFade:  "0",
				HighFade: "0",
			},
			Loop: bitwigLoop{
				Mode:  "off",
				Start: 0,
				Stop:  totalFrames,
			},
		}

		var wavBuf bytes.Buffer
		if err := EncodeWavContainer(&writeSeekBuffer{Buffer: &wavBuf}, &s, targetBitDepth); err != nil {
			return fmt.Errorf("failed encoding WAV slice %d for multisample: %w", i+1, err)
		}

		h := &zip.FileHeader{
			Name:   sliceName,
			Method: zip.Store, // STORED (uncompressed) for direct audio streaming
		}
		fw, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err := fw.Write(wavBuf.Bytes()); err != nil {
			return err
		}
	}

	xmlData, err := xml.MarshalIndent(msXML, "", "  ")
	if err != nil {
		return fmt.Errorf("failed marshaling multisample.xml: %w", err)
	}

	xmlHeader := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fullXML := append(xmlHeader, xmlData...)

	xmlZipHeader := &zip.FileHeader{
		Name:   "multisample.xml",
		Method: zip.Deflate,
	}
	fx, err := zw.CreateHeader(xmlZipHeader)
	if err != nil {
		return err
	}
	if _, err := fx.Write(fullXML); err != nil {
		return err
	}

	return zw.Close()
}
