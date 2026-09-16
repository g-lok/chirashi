package engine

import (
	"fmt"
	"strings"
)

type PGMReader struct{}

func (r *PGMReader) SupportedExtensions() []string {
	return []string{".pgm"}
}

func (r *PGMReader) Probe(data []byte) (*RexMetadata, error) {
	if len(data) < 16 || data[0] != 0x07 {
		return nil, fmt.Errorf("invalid PGM header magic")
	}
	return &RexMetadata{
		Channels:   2,
		SampleRate: 44100,
	}, nil
}

func (r *PGMReader) Read(data []byte, targetRate int) ([]SliceExtraction, error) {
	if len(data) < 16 || data[0] != 0x07 {
		return nil, fmt.Errorf("invalid PGM file")
	}

	return nil, fmt.Errorf("PGM reader parses sidecar program files with companion WAVs")
}

func (r *PGMReader) CanRead(ext string) bool {
	return strings.ToLower(ext) == ".pgm"
}

func init() {
	RegisterReader(&PGMReader{})
}
