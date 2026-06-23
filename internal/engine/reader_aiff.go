package engine

import (
	"encoding/binary"
	"fmt"
	"math"
)

type AIFFReader struct{}

func (r *AIFFReader) Probe(data []byte) (*RexMetadata, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("aiff: too short")
	}
	if string(data[:4]) != "FORM" {
		return nil, fmt.Errorf("aiff: not an IFF file")
	}
	formType := string(data[8:12])
	if formType != "AIFF" && formType != "AIFC" {
		return nil, fmt.Errorf("aiff: unsupported form type: %s", formType)
	}
	sampleRate, channels, err := readAIFFComm(data)
	if err != nil {
		return nil, err
	}
	return &RexMetadata{SampleRate: sampleRate, Channels: channels}, nil
}

func (r *AIFFReader) SupportedExtensions() []string {
	return []string{".aif", ".aiff"}
}

func (r *AIFFReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("aiff: file too short")
	}
	if string(data[:4]) != "FORM" {
		return nil, fmt.Errorf("aiff: not an IFF file")
	}

	formType := string(data[8:12])
	if formType == "AIFC" {
		compression := readAIFCCompression(data)
		if compression != "NONE" && compression != "twos" && compression != "sowt" && compression != "FL32" && compression != "FL64" {
			return nil, fmt.Errorf("aiff: unsupported AIFC compression: %s", compression)
		}
	}

	sampleRate, channels, bitDepth, err := readAIFFFullComm(data)
	if err != nil {
		return nil, fmt.Errorf("aiff: %w", err)
	}

	ssndData, err := readAIFFSSND(data)
	if err != nil {
		return nil, fmt.Errorf("aiff: %w", err)
	}

	marks := readAIFFMarks(data)

	pcm, err := decodeAIFFSamples(ssndData, bitDepth)
	if err != nil {
		return nil, fmt.Errorf("aiff: %w", err)
	}

	totalFrames := len(pcm) / channels

	meta := RexMetadata{
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   bitDepth,
	}

	if len(marks) == 0 {
		cues := []WavCueMarker{
			{SliceID: 0, Position: 0, Label: "Slice 01"},
		}
		return []SliceExtraction{
			{
				Metadata:    meta,
				CuePoints:   cues,
				Interleaved: pcm,
				TotalFrames: totalFrames,
			},
		}, nil
	}

	slices := make([]SliceExtraction, 0, len(marks)+1)
	startPos := 0
	for _, m := range marks {
		endPos := m.Position
		if endPos > totalFrames {
			endPos = totalFrames
		}
		if endPos <= startPos {
			startPos = endPos
			continue
		}
		frameCount := endPos - startPos
		slicePCM := make([]float32, frameCount*channels)
		copy(slicePCM, pcm[startPos*channels:endPos*channels])

		slices = append(slices, SliceExtraction{
			Metadata:    meta,
			CuePoints:   []WavCueMarker{{SliceID: len(slices), Position: 0, Label: m.Label}},
			Interleaved: slicePCM,
			TotalFrames: frameCount,
		})
		startPos = endPos
	}

	if startPos < totalFrames {
		frameCount := totalFrames - startPos
		slicePCM := make([]float32, frameCount*channels)
		copy(slicePCM, pcm[startPos*channels:])
		slices = append(slices, SliceExtraction{
			Metadata:    meta,
			CuePoints:   []WavCueMarker{{SliceID: len(slices), Position: 0, Label: fmt.Sprintf("Slice %02d", len(slices)+1)}},
			Interleaved: slicePCM,
			TotalFrames: frameCount,
		})
	}

	if len(slices) == 0 {
		cues := []WavCueMarker{
			{SliceID: 0, Position: 0, Label: "Slice 01"},
		}
		slices = []SliceExtraction{
			{
				Metadata:    meta,
				CuePoints:   cues,
				Interleaved: pcm,
				TotalFrames: totalFrames,
			},
		}
	}

	return slices, nil
}

func readAIFFComm(data []byte) (sampleRate, channels int, err error) {
	sr, ch, _, err := readAIFFFullComm(data)
	return sr, ch, err
}

func readAIFFFullComm(data []byte) (sampleRate, channels, bitDepth int, err error) {
	pos := 12
	for pos+8 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		if chunkSize < 0 || chunkSize > len(data)-pos-8 {
			break
		}
		if chunkID == "COMM" {
			if chunkSize < 18 {
				return 0, 0, 0, fmt.Errorf("COMM chunk too small")
			}
			channels = int(binary.BigEndian.Uint16(data[pos+8 : pos+10]))
			_ = int(binary.BigEndian.Uint32(data[pos+10 : pos+14])) // frameCount
			bitDepth = int(binary.BigEndian.Uint16(data[pos+14 : pos+16]))
			exp := int(binary.BigEndian.Uint16(data[pos+16 : pos+18]))
			mant := binary.BigEndian.Uint64(data[pos+18 : pos+26])
			sampleRate = int(math.Ldexp(float64(mant), exp-16383-63))
			if sampleRate <= 0 {
				sampleRate = 44100
			}
			return sampleRate, channels, bitDepth, nil
		}
		if chunkSize%2 == 1 {
			chunkSize++
		}
		pos += 8 + chunkSize
	}
	return 0, 0, 0, fmt.Errorf("no COMM chunk found")
}

func readAIFFSSND(data []byte) ([]byte, error) {
	pos := 12
	for pos+8 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		if chunkSize < 0 || chunkSize > len(data)-pos-8 {
			break
		}
		if chunkID == "SSND" {
			if chunkSize < 8 {
				return nil, fmt.Errorf("SSND chunk too small")
			}
			offset := int(binary.BigEndian.Uint32(data[pos+8 : pos+12]))
			_ = int(binary.BigEndian.Uint32(data[pos+12 : pos+16])) // blockSize
			audioData := data[pos+16+offset : pos+8+chunkSize]
			return audioData, nil
		}
		if chunkSize%2 == 1 {
			chunkSize++
		}
		pos += 8 + chunkSize
	}
	return nil, fmt.Errorf("no SSND chunk found")
}

type aiffMark struct {
	ID       int
	Position int
	Label    string
}

func readAIFFMarks(data []byte) []aiffMark {
	pos := 12
	for pos+8 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		if chunkSize < 0 || chunkSize > len(data)-pos-8 {
			break
		}
		if chunkID == "MARK" {
			if chunkSize < 2 {
				return nil
			}
			numMarks := int(binary.BigEndian.Uint16(data[pos+8 : pos+10]))
			marks := make([]aiffMark, 0, numMarks)
			markPos := pos + 10
			for i := 0; i < numMarks && markPos+2 <= pos+8+chunkSize; i++ {
				if markPos+8 > pos+8+chunkSize {
					break
				}
				markID := int(binary.BigEndian.Uint16(data[markPos : markPos+2]))
				markPosition := int(binary.BigEndian.Uint32(data[markPos+2 : markPos+6]))
				nameLen := int(data[markPos+6])
				if nameLen < 0 || markPos+7+nameLen > pos+8+chunkSize {
					break
				}
				name := fmt.Sprintf("Mark %d", markID)
				if nameLen > 0 {
					name = string(data[markPos+7 : markPos+7+nameLen])
				}

				marks = append(marks, aiffMark{
					ID:       markID,
					Position: markPosition,
					Label:    name,
				})
				pascalFieldLen := 1 + nameLen
				if pascalFieldLen%2 == 1 {
					pascalFieldLen++
				}
				entrySize := 2 + 4 + pascalFieldLen
				markPos += entrySize
			}
			return marks
		}
		if chunkSize%2 == 1 {
			chunkSize++
		}
		pos += 8 + chunkSize
	}
	return nil
}

func readAIFCCompression(data []byte) string {
	pos := 12
	for pos+8 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		if chunkSize < 0 || chunkSize > len(data)-pos-8 {
			break
		}
		if chunkID == "COMM" {
			if chunkSize >= 22 {
				return string(data[pos+18 : pos+22])
			}
			return "NONE"
		}
		if chunkSize%2 == 1 {
			chunkSize++
		}
		pos += 8 + chunkSize
	}
	return "NONE"
}

func decodeAIFFSamples(pcmData []byte, bitDepth int) ([]float32, error) {
	totalSamples := len(pcmData) / (bitDepth / 8)
	pcm := make([]float32, totalSamples)

	switch bitDepth {
	case 8:
		for i := range pcm {
			pcm[i] = float32(int8(pcmData[i])) / 128.0
		}
	case 16:
		for i := 0; i < len(pcmData)-1; i += 2 {
			val := int16(binary.BigEndian.Uint16(pcmData[i:]))
			pcm[i/2] = float32(val) / 32768.0
		}
	case 24:
		for i := 0; i < len(pcmData)-2; i += 3 {
			val := int32(int8(pcmData[i]))<<16 | int32(pcmData[i+1])<<8 | int32(pcmData[i+2])
			pcm[i/3] = float32(val) / 8388608.0
		}
	default:
		return nil, fmt.Errorf("unsupported AIFF bit depth: %d", bitDepth)
	}

	return pcm, nil
}

func init() {
	RegisterReader(&AIFFReader{})
}
