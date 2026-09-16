package engine

import (
	"fmt"
	"os"
	"path/filepath"
)

var sp404BankNames = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}

func SP404PadName(index int) string {
	bankIdx := index / 16
	padIdx := (index % 16) + 1
	if bankIdx >= len(sp404BankNames) {
		bankIdx = len(sp404BankNames) - 1
		padIdx = 16
	}
	return fmt.Sprintf("%s%02d", sp404BankNames[bankIdx], padIdx)
}

func EncodeSP404(baseDir string, extraction *SliceExtraction, name string, targetBitRate int) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode SP-404: extraction data is empty")
	}

	slices := splitExtractionIntoSlices(extraction)
	if len(slices) == 0 {
		slices = []SliceExtraction{*extraction}
	}

	if len(slices) > 160 {
		slices = slices[:160] // SP-404 MKII project limit: 10 banks * 16 pads = 160
	}

	importDir := filepath.Join(baseDir, "ROLAND", "SP-404MK2", "IMPORT")
	if err := os.MkdirAll(importDir, 0755); err != nil {
		return fmt.Errorf("failed creating SP-404 import directory: %w", err)
	}

	if name == "" {
		name = "slice"
	}
	name = sanitizeName(name, 20)

	for i, s := range slices {
		padCode := SP404PadName(i)
		fileName := fmt.Sprintf("%s_%s_%02d.wav", padCode, name, i+1)
		filePath := filepath.Join(importDir, fileName)

		f, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed creating SP-404 pad file %s: %w", filePath, err)
		}

		sliceToEncode := s
		targetSR := 48000
		if sliceToEncode.Metadata.SampleRate != targetSR && sliceToEncode.Metadata.SampleRate > 0 {
			if err := ForceSampleRate(&sliceToEncode, targetSR); err != nil {
				f.Close()
				return fmt.Errorf("failed resampling SP-404 slice %d: %w", i+1, err)
			}
		}

		if err := EncodeWavContainer(f, &sliceToEncode, targetBitRate); err != nil {
			f.Close()
			return fmt.Errorf("failed writing SP-404 WAV %s: %w", filePath, err)
		}
		f.Close()
	}

	return nil
}
