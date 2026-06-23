package engine

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type CAFReader struct{}

func (r *CAFReader) Probe(data []byte) (*RexMetadata, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("caf: too short")
	}
	if string(data[:4]) != "caff" {
		return nil, fmt.Errorf("caf: not a CAF file")
	}
	sr, ch, err := readCAFDesc(data)
	if err != nil {
		return nil, err
	}
	return &RexMetadata{SampleRate: sr, Channels: ch}, nil
}

func (r *CAFReader) SupportedExtensions() []string {
	return []string{".caf"}
}

func (r *CAFReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("caf: file too short")
	}
	if string(data[:4]) != "caff" {
		return nil, fmt.Errorf("caf: not a CAF file")
	}

	sampleRate, channels, bitDepth, formatFlags, err := readCAFFullDesc(data)
	if err != nil {
		return nil, fmt.Errorf("caf: %w", err)
	}

	pcmData, err := readCAFData(data)
	if err != nil {
		return nil, fmt.Errorf("caf: %w", err)
	}

	pcm, err := decodeCAFSamples(pcmData, bitDepth, formatFlags)
	if err != nil {
		return nil, fmt.Errorf("caf: %w", err)
	}

	totalFrames := len(pcm) / channels

	meta := RexMetadata{
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   bitDepth,
	}

	beatCount, timeSig := readAppleLoopMeta(data)
	if beatCount > 0 {
		meta.PPQLength = beatCount * 15360
	}
	if timeSig != "" {
		parts := strings.SplitN(timeSig, "/", 2)
		if len(parts) == 2 {
			meta.TimeSignNom, _ = strconv.Atoi(parts[0])
			meta.TimeSignDenom, _ = strconv.Atoi(parts[1])
		}
	}

	if beatCount > 0 {
		meta.OriginalTempo = float64(beatCount) * 60.0 / (float64(totalFrames) / float64(sampleRate))
		meta.Tempo = meta.OriginalTempo
	}

	markers := readCAFBeatMarkers(data)

	if len(markers) == 0 {
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

	slices := make([]SliceExtraction, 0, len(markers))
	startPos := 0
	for _, m := range markers {
		endPos := m
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
			CuePoints:   []WavCueMarker{{SliceID: len(slices), Position: 0, Label: fmt.Sprintf("Slice %02d", len(slices)+1)}},
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

func readCAFDesc(data []byte) (sampleRate, channels int, err error) {
	sr, ch, _, _, err := readCAFFullDesc(data)
	return sr, ch, err
}

func readCAFFullDesc(data []byte) (sampleRate, channels, bitDepth, formatFlags int, err error) {
	pos := 8
	for pos+12 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int64(binary.BigEndian.Uint64(data[pos+4 : pos+12]))
		if chunkSize < 0 || chunkSize > int64(len(data)-pos-12) {
			break
		}
		if chunkID == "desc" {
			if chunkSize < 32 {
				return 0, 0, 0, 0, fmt.Errorf("desc chunk too small")
			}
			rateBits := binary.BigEndian.Uint64(data[pos+12 : pos+20])
			sampleRate = int(math.Float64frombits(rateBits))
			if sampleRate <= 0 {
				sampleRate = 44100
			}
			formatFlags = int(binary.BigEndian.Uint32(data[pos+24 : pos+28]))
			channels = int(binary.BigEndian.Uint32(data[pos+36 : pos+40]))
			bitDepth = int(binary.BigEndian.Uint32(data[pos+40 : pos+44]))
			return sampleRate, channels, bitDepth, formatFlags, nil
		}
		pos += 12 + int(chunkSize)
	}
	return 0, 0, 0, 0, fmt.Errorf("no desc chunk found")
}

func readCAFData(data []byte) ([]byte, error) {
	pos := 8
	for pos+12 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int64(binary.BigEndian.Uint64(data[pos+4 : pos+12]))
		if chunkSize < 0 || chunkSize > int64(len(data)-pos-12) {
			break
		}
		if chunkID == "data" {
			if chunkSize < 4 {
				return nil, fmt.Errorf("data chunk too small")
			}
			pcmStart := pos + 12 + 4
			pcmEnd := pos + 12 + int(chunkSize)
			if pcmStart > pcmEnd {
				return nil, fmt.Errorf("invalid data chunk offset")
			}
			return data[pcmStart:pcmEnd], nil
		}
		pos += 12 + int(chunkSize)
	}
	return nil, fmt.Errorf("no data chunk found")
}

func decodeCAFSamples(pcmData []byte, bitDepth, formatFlags int) ([]float32, error) {
	isLittleEndian := (formatFlags & 2) != 0
	isFloat := (formatFlags & 1) != 0
	isSigned := (formatFlags & 4) != 0

	totalSamples := len(pcmData) / (bitDepth / 8)
	pcm := make([]float32, totalSamples)

	if isFloat && bitDepth == 32 {
		for i := range pcm {
			var bits uint32
			if isLittleEndian {
				bits = binary.LittleEndian.Uint32(pcmData[i*4:])
			} else {
				bits = binary.BigEndian.Uint32(pcmData[i*4:])
			}
			pcm[i] = math.Float32frombits(bits)
		}
		return pcm, nil
	}

	switch bitDepth {
	case 8:
		for i := range pcm {
			if isSigned {
				pcm[i] = float32(int8(pcmData[i])) / 128.0
			} else {
				pcm[i] = float32(pcmData[i])/128.0 - 1.0
			}
		}
	case 16:
		for i := 0; i < len(pcmData)-1; i += 2 {
			var val int16
			if isLittleEndian {
				val = int16(binary.LittleEndian.Uint16(pcmData[i:]))
			} else {
				val = int16(binary.BigEndian.Uint16(pcmData[i:]))
			}
			pcm[i/2] = float32(val) / 32768.0
		}
	case 24:
		for i := 0; i < len(pcmData)-2; i += 3 {
			var val int32
			if isLittleEndian {
				val = int32(int8(pcmData[i+2]))<<16 | int32(pcmData[i+1])<<8 | int32(pcmData[i])
			} else {
				val = int32(int8(pcmData[i]))<<16 | int32(pcmData[i+1])<<8 | int32(pcmData[i+2])
			}
			pcm[i/3] = float32(val) / 8388608.0
		}
	case 32:
		for i := 0; i < len(pcmData)-3; i += 4 {
			var val int32
			if isLittleEndian {
				val = int32(binary.LittleEndian.Uint32(pcmData[i:]))
			} else {
				val = int32(binary.BigEndian.Uint32(pcmData[i:]))
			}
			pcm[i/4] = float32(val) / 2147483648.0
		}
	default:
		return nil, fmt.Errorf("unsupported CAF bit depth: %d", bitDepth)
	}

	return pcm, nil
}

func readAppleLoopMeta(data []byte) (beatCount int, timeSig string) {
	pos := 8
	for pos+12 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int64(binary.BigEndian.Uint64(data[pos+4 : pos+12]))
		if chunkSize < 0 || chunkSize > int64(len(data)-pos-12) {
			break
		}
		if chunkID == "uuid" {
			dataStart := pos + 12
			if int64(dataStart+16) > int64(len(data)) {
				pos += 12 + int(chunkSize)
				continue
			}
			uuid := data[dataStart : dataStart+16]

			if !bytesEqual(uuid, appleLoopMetaUUID) {
				pos += 12 + int(chunkSize)
				continue
			}

			kvStart := dataStart + 16
			kvEnd := dataStart + int(chunkSize)
			if kvStart+4 > kvEnd {
				break
			}
			count := int(binary.BigEndian.Uint32(data[kvStart : kvStart+4]))
			offset := kvStart + 4

			for i := 0; i < count && offset < kvEnd; i++ {
				keyEnd := offset
				for keyEnd < kvEnd && data[keyEnd] != 0 {
					keyEnd++
				}
				if keyEnd >= kvEnd {
					break
				}
				key := string(data[offset:keyEnd])
				offset = keyEnd + 1

				valEnd := offset
				for valEnd < kvEnd && data[valEnd] != 0 {
					valEnd++
				}
				if valEnd > kvEnd {
					break
				}
				val := string(data[offset:valEnd])
				offset = valEnd + 1

				switch key {
				case "beat count":
					beatCount, _ = strconv.Atoi(val)
				case "time signature":
					timeSig = val
				}
			}
			return beatCount, timeSig
		}
		pos += 12 + int(chunkSize)
	}
	return 0, ""
}

func readCAFBeatMarkers(data []byte) []int {
	pos := 8
	for pos+12 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int64(binary.BigEndian.Uint64(data[pos+4 : pos+12]))
		if chunkSize < 0 || chunkSize > int64(len(data)-pos-12) {
			break
		}
		if chunkID == "uuid" {
			dataStart := pos + 12
			if int64(dataStart+16) > int64(len(data)) {
				pos += 12 + int(chunkSize)
				continue
			}
			uuid := data[dataStart : dataStart+16]
			if !bytesEqual(uuid, appleLoopBeatMarkersUUID) {
				pos += 12 + int(chunkSize)
				continue
			}

			bodyStart := dataStart + 16
			bodyEnd := dataStart + int(chunkSize)

			if bodyStart+20 > bodyEnd {
				break
			}

			_ = binary.BigEndian.Uint32(data[bodyStart:])     // unknown
			_ = binary.BigEndian.Uint32(data[bodyStart+4:])   // flags
			_ = binary.BigEndian.Uint16(data[bodyStart+8:])   // version
			_ = binary.BigEndian.Uint16(data[bodyStart+10:])  // unknown
			_ = binary.BigEndian.Uint32(data[bodyStart+12:])  // unknown
			numMarkers := int(binary.BigEndian.Uint32(data[bodyStart+16:]))

			if numMarkers <= 0 || bodyStart+20+numMarkers*12 > bodyEnd {
				break
			}

			markers := make([]int, 0, numMarkers)
			entryPos := bodyStart + 20
			for i := 0; i < numMarkers; i++ {
				_ = binary.BigEndian.Uint16(data[entryPos:])    // flags
				_ = binary.BigEndian.Uint16(data[entryPos+2:])  // padding
				_ = binary.BigEndian.Uint32(data[entryPos+4:])  // padding
				pos := int(binary.BigEndian.Uint32(data[entryPos+8:]))
				markers = append(markers, pos)
				entryPos += 12
			}
			return markers
		}
		pos += 12 + int(chunkSize)
	}
	return nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func init() {
	RegisterReader(&CAFReader{})
}
