package rexengine

import (
	"encoding/binary"
	"fmt"
)

type WAVReader struct{}

func (r *WAVReader) Probe(data []byte) (*RexMetadata, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("wav: too short")
	}
	if string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, fmt.Errorf("wav: invalid header")
	}
	sampleRate, channels, err := readWAVFmt(data)
	if err != nil {
		return nil, err
	}
	return &RexMetadata{SampleRate: sampleRate, Channels: channels}, nil
}

func (r *WAVReader) SupportedExtensions() []string {
	return []string{".wav"}
}

func (r *WAVReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("wav: file too short")
	}
	if string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, fmt.Errorf("wav: invalid RIFF/WAVE header")
	}

	sampleRate, channels, bitDepth, err := readWAVFullFmt(data)
	if err != nil {
		return nil, fmt.Errorf("wav: %w", err)
	}

	pcmData, err := readWAVData(data)
	if err != nil {
		return nil, fmt.Errorf("wav: %w", err)
	}

	cuePoints := readWAVCues(data)

	pcm, err := decodeWAVSamples(pcmData, bitDepth)
	if err != nil {
		return nil, fmt.Errorf("wav: %w", err)
	}

	totalFrames := len(pcm) / channels

	meta := RexMetadata{
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   bitDepth,
	}

	if len(cuePoints) == 0 {
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

	slices := make([]SliceExtraction, 0, len(cuePoints)+1)
	startPos := 0
	for idx, cp := range cuePoints {
		endPos := int(cp.Position)
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
			CuePoints:   []WavCueMarker{{SliceID: len(slices), Position: 0, Label: fmt.Sprintf("Slice %02d", idx+1)}},
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

func readWAVFmt(data []byte) (sampleRate, channels int, err error) {
	sr, ch, _, err := readWAVFullFmt(data)
	return sr, ch, err
}

func readWAVFullFmt(data []byte) (sampleRate, channels, bitDepth int, err error) {
	pos := 12
	for pos+8 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		if chunkSize < 0 || chunkSize > len(data)-pos-8 {
			break
		}
		if chunkID == "fmt " {
			if chunkSize < 16 {
				return 0, 0, 0, fmt.Errorf("fmt chunk too small")
			}
			audioFormat := binary.LittleEndian.Uint16(data[pos+8 : pos+10])
			if audioFormat != 1 && audioFormat != 3 {
				return 0, 0, 0, fmt.Errorf("unsupported audio format %d (only PCM=1, IEEE float=3)", audioFormat)
			}
			channels = int(binary.LittleEndian.Uint16(data[pos+10 : pos+12]))
			sampleRate = int(binary.LittleEndian.Uint32(data[pos+12 : pos+16]))
			bitDepth = int(binary.LittleEndian.Uint16(data[pos+22 : pos+24]))
			return sampleRate, channels, bitDepth, nil
		}
		if chunkSize%2 == 1 {
			chunkSize++
		}
		pos += 8 + chunkSize
	}
	return 0, 0, 0, fmt.Errorf("no fmt chunk found")
}

func readWAVData(data []byte) ([]byte, error) {
	pos := 12
	for pos+8 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		if chunkSize < 0 || chunkSize > len(data)-pos-8 {
			break
		}
		if chunkID == "data" {
			return data[pos+8 : pos+8+chunkSize], nil
		}
		if chunkSize%2 == 1 {
			chunkSize++
		}
		pos += 8 + chunkSize
	}
	return nil, fmt.Errorf("no data chunk found")
}

type wavCuePointRaw struct {
	DWName         uint32
	DWPosition     uint32
	FccChunk       [4]byte
	DWChunkStart   uint32
	DWBlockStart   uint32
	DWSampleOffset uint32
}

func readWAVCues(data []byte) []WavCueMarker {
	pos := 12
	for pos+8 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		if chunkSize < 0 || chunkSize > len(data)-pos-8 {
			break
		}
		if chunkID == "cue " {
			if chunkSize < 4 {
				return nil
			}
			numCues := int(binary.LittleEndian.Uint32(data[pos+8 : pos+12]))
			if chunkSize < 4+numCues*24 {
				return nil
			}
			cues := make([]WavCueMarker, numCues)
			for i := 0; i < numCues; i++ {
				offset := pos + 12 + i*24
				raw := wavCuePointRaw{
					DWName:         binary.LittleEndian.Uint32(data[offset : offset+4]),
					DWPosition:     binary.LittleEndian.Uint32(data[offset+4 : offset+8]),
					FccChunk:       [4]byte{data[offset+8], data[offset+9], data[offset+10], data[offset+11]},
					DWChunkStart:   binary.LittleEndian.Uint32(data[offset+12 : offset+16]),
					DWBlockStart:   binary.LittleEndian.Uint32(data[offset+16 : offset+20]),
					DWSampleOffset: binary.LittleEndian.Uint32(data[offset+20 : offset+24]),
				}
				cues[i] = WavCueMarker{
					SliceID:  int(raw.DWName) - 1,
					Position: raw.DWSampleOffset,
					Label:    fmt.Sprintf("Cue %d", raw.DWName),
				}
			}
			return cues
		}
		if chunkSize%2 == 1 {
			chunkSize++
		}
		pos += 8 + chunkSize
	}
	return nil
}

func decodeWAVSamples(pcmData []byte, bitDepth int) ([]float32, error) {
	totalSamples := len(pcmData) / (bitDepth / 8)
	pcm := make([]float32, totalSamples)

	switch bitDepth {
	case 8:
		for i := range pcm {
			pcm[i] = float32(int(pcmData[i])-128) / 128.0
		}
	case 16:
		for i := 0; i < len(pcmData)-1; i += 2 {
			val := int16(binary.LittleEndian.Uint16(pcmData[i:]))
			pcm[i/2] = float32(val) / 32768.0
		}
	case 24:
		for i := 0; i < len(pcmData)-2; i += 3 {
			val := int32(int8(pcmData[i+2]))<<16 | int32(pcmData[i+1])<<8 | int32(pcmData[i])
			pcm[i/3] = float32(val) / 8388608.0
		}
	case 32:
		for i := 0; i < len(pcmData)-3; i += 4 {
			val := int32(binary.LittleEndian.Uint32(pcmData[i:]))
			pcm[i/4] = float32(val) / 2147483648.0
		}
	default:
		return nil, fmt.Errorf("unsupported bit depth: %d", bitDepth)
	}

	return pcm, nil
}

func init() {
	RegisterReader(&WAVReader{})
}
