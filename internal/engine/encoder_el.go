package engine

import (
	"fmt"
	"io"
)

type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) write(s string) {
	if ew.err != nil {
		return
	}
	_, ew.err = io.WriteString(ew.w, s)
}

func (ew *errWriter) writef(format string, args ...interface{}) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintf(ew.w, format, args...)
}

func EncodeEL(w io.Writer, extraction *SliceExtraction) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode EL: extraction data is empty")
	}

	numSlices := len(extraction.CuePoints)
	if numSlices == 0 {
		numSlices = 1
	}
	if numSlices > 64 {
		numSlices = 64
	}

	ew := &errWriter{w: w}
	ew.write("# ELEKTRON MULTI-SAMPLE MAPPING FORMAT\n")
	ew.write("version = 0\n")
	ew.write("name = 'REXConverter'\n\n")

	for i := 0; i < numSlices; i++ {
		pitch := 24 + i

		ew.write("[[key-zones]]\n")
		ew.writef("pitch = %d\n", pitch)
		ew.writef("key-center = %.1f\n\n", float64(pitch))

		ew.write("[[key-zones.velocity-layers]]\n")
		ew.write("velocity = 0.49411765\n")
		ew.write("strategy = 'Forward'\n\n")

		ew.write("[[key-zones.velocity-layers.sample-slots]]\n")
		ew.writef("sample = 'slice_%02d.wav'\n\n", i+1)
	}

	return ew.err
}
