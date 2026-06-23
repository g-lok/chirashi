package engine

import (
	"fmt"
	"strings"
)

type InputReader interface {
	Probe(data []byte) (*RexMetadata, error)
	Read(data []byte, targetSampleRate int) ([]SliceExtraction, error)
	SupportedExtensions() []string
}

var readers []InputReader

func RegisterReader(r InputReader) {
	readers = append(readers, r)
}

func DetectReader(ext string) InputReader {
	ext = strings.ToLower(ext)
	for _, r := range readers {
		for _, e := range r.SupportedExtensions() {
			if ext == e {
				return r
			}
		}
	}
	return nil
}

func ProbeInput(data []byte) (*RexMetadata, error) {
	for _, r := range readers {
		meta, err := r.Probe(data)
		if err == nil && meta != nil {
			return meta, nil
		}
	}
	return nil, fmt.Errorf("unsupported format: could not probe input")
}

var sampleLibraryPath string

func SetLibraryPath(path string) {
	sampleLibraryPath = path
}
