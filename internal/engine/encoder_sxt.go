package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

func EncodeSXT(w io.Writer, extraction *SliceExtraction, name string) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode SXT: extraction data is empty")
	}

	slices := splitExtractionIntoSlices(extraction)
	if len(slices) == 0 {
		slices = []SliceExtraction{*extraction}
	}

	if len(slices) > 128 {
		slices = slices[:128] // Max 128 keyboard notes
	}

	if name == "" {
		name = "chirashi"
	}
	name = sanitizeName(name, 32)

	var bodyBuf bytes.Buffer

	baseNote := 24 // Start at C1

	for i := range slices {
		sliceName := fmt.Sprintf("slice_%02d.wav", i+1)
		if len(sliceName) > 32 {
			sliceName = sliceName[:32]
		}
		formattedName := fmt.Sprintf("%-32s", sliceName)

		midiNote := baseNote + i
		if midiNote > 127 {
			midiNote = 127
		}

		// 32B filename + 1B root + 1B lowKey + 1B highKey + 1B lowVel + 1B highVel + 1B tune + 1B pan + 1B vol + 9B reserved = 48B record
		var rec [48]byte
		copy(rec[0:32], formattedName)
		rec[32] = byte(midiNote) // rootNote
		rec[33] = byte(midiNote) // lowKey
		rec[34] = byte(midiNote) // highKey
		rec[35] = 1              // lowVel
		rec[36] = 127            // highVel
		rec[37] = 0              // tune
		rec[38] = 0              // pan
		rec[39] = 100            // volume

		bodyBuf.Write(rec[:])
	}

	// Build FORM / SXT container
	var formBuf bytes.Buffer
	formBuf.WriteString("SXT ")

	// NAME chunk
	formBuf.WriteString("NAME")
	nameBytes := []byte(fmt.Sprintf("%-32s", strings.TrimSpace(name)))
	binary.Write(&formBuf, binary.BigEndian, uint32(len(nameBytes)))
	formBuf.Write(nameBytes)

	// BODY chunk
	formBuf.WriteString("BODY")
	binary.Write(&formBuf, binary.BigEndian, uint32(bodyBuf.Len()))
	formBuf.Write(bodyBuf.Bytes())

	// Write FORM header
	fullData := formBuf.Bytes()
	if _, err := w.Write([]byte("FORM")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(fullData))); err != nil {
		return err
	}
	_, err := w.Write(fullData)
	return err
}
