package engine

import (
	"encoding/binary"
	"fmt"
	"io"
)

func EncodeAIFF(w io.Writer, extraction *SliceExtraction) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode AIFF: extraction data is empty")
	}

	channels := extraction.Metadata.Channels
	sampleRate := extraction.Metadata.SampleRate
	bitDepth := 16
	if extraction.Metadata.BitDepth > 0 {
		bitDepth = extraction.Metadata.BitDepth
	}
	numChannels := uint16(channels)
	bitsPerSample := uint16(bitDepth)
	bytesPerSample := uint16(bitDepth / 8)
	blockAlign := numChannels * bytesPerSample

	totalSamples := len(extraction.Interleaved)
	numFrames := totalSamples / channels
	dataSize := uint32(numFrames * int(blockAlign))

	commSize := uint32(18)
	ssndOffset := uint32(0)
	ssndBlockSize := uint32(0)

	markSize := uint32(0)
	numMarkers := len(extraction.CuePoints)
	if numMarkers > 0 {
		markSize = 2 // numMarks uint16
		for i, cp := range extraction.CuePoints {
			label := cp.Label
			if label == "" {
				label = fmt.Sprintf("S%02d", i+1)
			}
			nameLen := len(label)
			if nameLen > 255 {
				nameLen = 255
			}
			pascalLen := 1 + nameLen
			entrySize := 2 + 4 + pascalLen // ID + Position + Pascal string
			if pascalLen%2 == 1 {
				entrySize++ // pad to even
			}
			markSize += uint32(entrySize)
		}
	}

	formSize := uint32(12) + 8 + commSize + 8 + markSize + 8 + 8 + dataSize

	if _, err := w.Write([]byte("FORM")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, formSize); err != nil {
		return err
	}
	if _, err := w.Write([]byte("AIFF")); err != nil {
		return err
	}

	if _, err := w.Write([]byte("COMM")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, commSize); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, numChannels); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(numFrames)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, bitsPerSample); err != nil {
		return err
	}
	sr := aiffSampleRate(sampleRate)
	if _, err := w.Write(sr[:]); err != nil {
		return err
	}

	if numMarkers > 0 {
		if _, err := w.Write([]byte("MARK")); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, markSize); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, uint16(numMarkers)); err != nil {
			return err
		}
		for i, cp := range extraction.CuePoints {
			if err := binary.Write(w, binary.BigEndian, uint16(i+1)); err != nil {
				return err
			}
			if err := binary.Write(w, binary.BigEndian, cp.Position); err != nil {
				return err
			}
			label := cp.Label
			if label == "" {
				label = fmt.Sprintf("S%02d", i+1)
			}
			labelBytes := []byte(label)
			if len(labelBytes) > 255 {
				labelBytes = labelBytes[:255]
			}
			if _, err := w.Write([]byte{byte(len(labelBytes))}); err != nil {
				return err
			}
			if _, err := w.Write(labelBytes); err != nil {
				return err
			}
			if len(labelBytes)%2 == 0 {
				if _, err := w.Write([]byte{0}); err != nil {
					return err
				}
			}
		}
	}

	if _, err := w.Write([]byte("SSND")); err != nil {
		return err
	}
	ssndChunkSize := uint32(8) + dataSize
	if err := binary.Write(w, binary.BigEndian, ssndChunkSize); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, ssndOffset); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, ssndBlockSize); err != nil {
		return err
	}

	buf := make([]byte, 1024*blockAlign)
	written := 0
	needed := numFrames * int(blockAlign)

	for written < needed {
		remaining := needed - written
		chunk := remaining
		if chunk > len(buf) {
			chunk = len(buf)
		}
		frameStart := written / int(blockAlign)
		switch bitDepth {
		case 16:
			for i := 0; i < chunk/int(bytesPerSample); i++ {
				sampleIdx := frameStart*channels + i
				val := int16(extraction.Interleaved[sampleIdx] * 32768)
				if val > 32767 {
					val = 32767
				} else if val < -32768 {
					val = -32768
				}
				pos := i * 2
				buf[pos] = byte(val >> 8)
				buf[pos+1] = byte(val)
			}
			written += chunk
		default:
			// Fallback: write 16-bit PCM for any unsupported bit depth.
			// Better to produce a valid AIFF than return an error.
			for i := 0; i < chunk/int(bytesPerSample); i++ {
				sampleIdx := frameStart*channels + i
				val := int16(extraction.Interleaved[sampleIdx] * 32768)
				if val > 32767 {
					val = 32767
				} else if val < -32768 {
					val = -32768
				}
				pos := i * 2
				buf[pos] = byte(val >> 8)
				buf[pos+1] = byte(val)
			}
			written += chunk
		}
		if _, err := w.Write(buf[:chunk]); err != nil {
			return err
		}
	}

	return nil
}
