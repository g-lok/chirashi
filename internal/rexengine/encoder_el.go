package rexengine

import (
	"fmt"
	"io"
)

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

	fmt.Fprintf(w, "# ELEKTRON MULTI-SAMPLE MAPPING FORMAT\n")
	fmt.Fprintf(w, "version = 0\n")
	fmt.Fprintf(w, "name = 'REXConverter'\n\n")

	for i := 0; i < numSlices; i++ {
		pitch := 24 + i

		fmt.Fprintf(w, "[[key-zones]]\n")
		fmt.Fprintf(w, "pitch = %d\n", pitch)
		fmt.Fprintf(w, "key-center = %.1f\n\n", float64(pitch))

		fmt.Fprintf(w, "[[key-zones.velocity-layers]]\n")
		fmt.Fprintf(w, "velocity = 0.49411765\n")
		fmt.Fprintf(w, "strategy = 'Forward'\n\n")

		fmt.Fprintf(w, "[[key-zones.velocity-layers.sample-slots]]\n")
		fmt.Fprintf(w, "sample = 'slice_%02d.wav'\n\n", i+1)
	}

	return nil
}
