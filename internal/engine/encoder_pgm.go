package engine

import (
	"fmt"
	"io"
	"strings"
)

func EncodePGM(w io.Writer, extraction *SliceExtraction, name string) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode PGM: extraction data is empty")
	}

	slices := splitExtractionIntoSlices(extraction)
	if len(slices) == 0 {
		slices = []SliceExtraction{*extraction}
	}

	if len(slices) > 64 {
		slices = slices[:64] // MPC 1000/500/2500 limit: 64 pads (Banks A-D)
	}

	if name == "" {
		name = "chirashi"
	}
	name = sanitizeName(name, 16)

	buf := make([]byte, 16+64*128)
	buf[0] = 0x07 // MPC PGM magic tag
	copy(buf[1:17], fmt.Sprintf("%-16s", name))

	for i := range slices {
		padOffset := 16 + i*128

		sampleName := fmt.Sprintf("slice_%02d.wav", i+1)
		if len(sampleName) > 16 {
			sampleName = sampleName[:16]
		}
		formattedName := fmt.Sprintf("%-16s", strings.ToUpper(sampleName))

		// Offset 0: Sample 1 filename (16 bytes)
		copy(buf[padOffset:padOffset+16], formattedName)

		// Offset 16: Volume / Level (0..100) -> 100
		buf[padOffset+16] = 100

		// Offset 17: Tune / Pitch
		buf[padOffset+17] = 0

		// Offset 18: Velocity Low
		buf[padOffset+18] = 1

		// Offset 19: Velocity High
		buf[padOffset+19] = 127

		// Offset 20: Voice Overlap / Play Mode
		buf[padOffset+20] = 0

		// Offset 21: MIDI Note Assignment (C2 = Note 36 + i)
		midiNote := 36 + i
		if midiNote > 127 {
			midiNote = 127
		}
		buf[padOffset+21] = byte(midiNote)
	}

	_, err := w.Write(buf)
	return err
}
