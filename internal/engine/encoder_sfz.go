package engine

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// EncodeSFZ writes an SFZ control file that maps slices within a companion WAV.
// The main extraction (unsliced) and the individual slices are used to build the mapping.
func EncodeSFZ(w io.Writer, extraction *SliceExtraction, wavName string) error {
	_, err := io.WriteString(w, "// chirashi generated SFZ\n")
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, fmt.Sprintf("<control>\ndefault_path=%s\n\n", filepath.Dir(wavName)))
	if err != nil {
		return err
	}

	baseName := filepath.Base(wavName)
	
	// Map slices to MIDI notes starting from C1 (24)
	startNote := 24
	for i, cp := range extraction.CuePoints {
		note := startNote + i
		if note > 127 {
			break
		}

		end := extraction.TotalFrames
		if i+1 < len(extraction.CuePoints) {
			end = int(extraction.CuePoints[i+1].Position)
		}

		label := cp.Label
		if label == "" {
			label = fmt.Sprintf("Slice %02d", i+1)
		}

		_, err = io.WriteString(w, fmt.Sprintf("<region>\n"))
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, fmt.Sprintf("sample=%s\n", baseName))
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, fmt.Sprintf("lokey=%d hikey=%d pitch_keycenter=%d\n", note, note, note))
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, fmt.Sprintf("offset=%d end=%d\n", cp.Position, end))
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, fmt.Sprintf("hint=%s\n\n", strings.ReplaceAll(label, "\n", " ")))
		if err != nil {
			return err
		}
	}

	return nil
}
