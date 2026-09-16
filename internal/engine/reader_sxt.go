package engine

import (
	"fmt"
	"strings"
)

type SXTReader struct{}

func (r *SXTReader) SupportedExtensions() []string {
	return []string{".sxt"}
}

func (r *SXTReader) Probe(data []byte) (*RexMetadata, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("file too short for SXT")
	}

	if string(data[:4]) != "FORM" {
		return nil, fmt.Errorf("not an IFF FORM file")
	}

	formType := string(data[8:12])
	if formType != "SXT " && formType != "SXT1" {
		return nil, fmt.Errorf("unsupported SXT form type %q", formType)
	}

	return &RexMetadata{
		Channels:   2,
		SampleRate: 44100,
	}, nil
}

func (r *SXTReader) Read(data []byte, targetRate int) ([]SliceExtraction, error) {
	if _, err := r.Probe(data); err != nil {
		return nil, err
	}

	wavReader := &WAVReader{}
	return wavReader.Read(data, targetRate)
}

func (r *SXTReader) CanRead(ext string) bool {
	ext = strings.ToLower(ext)
	return ext == ".sxt"
}

func init() {
	RegisterReader(&SXTReader{})
}
