package engine

import (
	"strings"
)

type SP404Reader struct{}

func (r *SP404Reader) SupportedExtensions() []string {
	return []string{".sp404"}
}

func (r *SP404Reader) Probe(data []byte) (*RexMetadata, error) {
	wavReader := &WAVReader{}
	return wavReader.Probe(data)
}

func (r *SP404Reader) Read(data []byte, targetRate int) ([]SliceExtraction, error) {
	wavReader := &WAVReader{}
	return wavReader.Read(data, targetRate)
}

func (r *SP404Reader) CanRead(ext string) bool {
	return strings.ToLower(ext) == ".sp404"
}

func init() {
	RegisterReader(&SP404Reader{})
}
