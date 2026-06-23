package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type PTIHeader struct {
	Magic         [4]byte
	Name          [28]byte
	Version       uint8
	RootNote      uint8
	FineTune      uint8
	Volume        uint8
	Pan           uint8
	SampleRateIdx uint8
	LoopMode      uint8
	LoopStart     uint32
	LoopEnd       uint32
	SampleLength  uint32
	MIDILow       uint8
	MIDIHigh      uint8
	MIDIRoot      uint8
	Reserved      [338]byte
}

type PTIReader struct{}

func (r *PTIReader) Probe(data []byte) (*RexMetadata, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("pti: too short (%d bytes)", len(data))
	}
	switch {
	case len(data) >= 4 && string(data[:2]) == "TI":
		return &RexMetadata{SampleRate: 44100, Channels: 1}, nil
	case string(data[:4]) == "PTI\x00":
		return &RexMetadata{SampleRate: 44100, Channels: 1}, nil
	default:
		return nil, fmt.Errorf("pti: unknown magic %02x %02x %02x %02x", data[0], data[1], data[2], data[3])
	}
}

func (r *PTIReader) SupportedExtensions() []string {
	return []string{".pti"}
}

func (r *PTIReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("pti: file too short (%d bytes)", len(data))
	}

	switch {
	case len(data) >= 2 && data[0] == 'T' && data[1] == 'I':
		return readPTITIFormat(data, targetSampleRate)
	case string(data[:4]) == "PTI\x00":
		return readPTILegacyFormat(data, targetSampleRate)
	default:
		return nil, fmt.Errorf("pti: unknown magic %02x %02x %02x %02x — expected TI or PTI\\x00", data[0], data[1], data[2], data[3])
	}
}

func readPTITIFormat(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if len(data) < 392 {
		return nil, fmt.Errorf("pti (TI format): header too short (%d bytes, need 392)", len(data))
	}

	if data[0] != 'T' || data[1] != 'I' {
		return nil, fmt.Errorf("pti (TI format): invalid magic %02x %02x", data[0], data[1])
	}

	sampleLen := int(binary.LittleEndian.Uint32(data[60:64]))
	numSlices := int(data[376])
	if numSlices <= 0 {
		numSlices = 1
	}
	if numSlices > 48 {
		numSlices = 48
	}

	playbackMode := data[76]
	isSliced := (playbackMode == 5)

	pcmRaw := data[392:]
	totalSamples := len(pcmRaw) / 2
	if sampleLen > 0 && sampleLen < totalSamples {
		totalSamples = sampleLen
	}
	if totalSamples == 0 {
		return nil, fmt.Errorf("pti (TI format): zero PCM samples")
	}

	pcm := make([]float32, totalSamples)
	for i := 0; i < totalSamples; i++ {
		if i*2+1 >= len(pcmRaw) {
			return nil, fmt.Errorf("pti (TI format): PCM data truncated at sample %d (need byte %d, have %d)", i, i*2+1, len(pcmRaw))
		}
		val := int16(binary.LittleEndian.Uint16(pcmRaw[i*2:]))
		pcm[i] = float32(val) / 32768.0
	}

	if !isSliced || numSlices <= 1 {
		return []SliceExtraction{
			{
				Metadata: RexMetadata{
					SampleRate: 44100,
					Channels:   1,
					BitDepth:   16,
				},
				CuePoints:   []WavCueMarker{{SliceID: 0, Position: 0, Label: "Slice 01"}},
				Interleaved: pcm,
				TotalFrames: totalSamples,
			},
		}, nil
	}

	var slicePositions []uint32
	for i := 0; i < 48; i++ {
		off := 280 + i*2
		if off+1 >= len(data) {
			return nil, fmt.Errorf("pti (TI format): ratio table truncated at entry %d (offset %d)", i, off)
		}
		val := binary.LittleEndian.Uint16(data[off : off+2])
		if val >= 65535 {
			break
		}
		pos := uint32(float64(val) / 65535.0 * float64(totalSamples))
		slicePositions = append(slicePositions, pos)
	}
	if len(slicePositions) > numSlices {
		slicePositions = slicePositions[:numSlices]
	}

	if len(slicePositions) == 0 {
		return []SliceExtraction{
			{
				Metadata: RexMetadata{
					SampleRate: 44100,
					Channels:   1,
					BitDepth:   16,
				},
				CuePoints:   []WavCueMarker{{SliceID: 0, Position: 0, Label: "Slice 01"}},
				Interleaved: pcm,
				TotalFrames: totalSamples,
			},
		}, nil
	}

	var slices []SliceExtraction
	for i, start := range slicePositions {
		var end uint32 = uint32(totalSamples)
		if i+1 < len(slicePositions) && slicePositions[i+1] > start {
			end = slicePositions[i+1]
		}
		fc := int(end - start)
		if fc <= 0 {
			continue
		}
		sp := make([]float32, fc)
		copy(sp, pcm[start:end])
		slices = append(slices, SliceExtraction{
			Metadata: RexMetadata{
				SampleRate: 44100,
				Channels:   1,
				BitDepth:   16,
			},
			CuePoints:   []WavCueMarker{{SliceID: i, Position: 0, Label: fmt.Sprintf("Slice %02d", i+1)}},
			Interleaved: sp,
			TotalFrames: fc,
		})
	}

	if len(slices) == 0 {
		slices = append(slices, SliceExtraction{
			Metadata: RexMetadata{
				SampleRate: 44100,
				Channels:   1,
				BitDepth:   16,
			},
			CuePoints:   []WavCueMarker{{SliceID: 0, Position: 0, Label: "Slice 01"}},
			Interleaved: pcm,
			TotalFrames: totalSamples,
		})
	}

	return slices, nil
}

func readPTILegacyFormat(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if len(data) < 392 {
		return nil, fmt.Errorf("pti (legacy format): file too short (%d bytes, need 392+ for header)", len(data))
	}
	if string(data[:4]) != "PTI\x00" {
		return nil, fmt.Errorf("pti (legacy format): invalid magic")
	}

	var hdr PTIHeader
	if err := binary.Read(bytes.NewReader(data[:392]), binary.LittleEndian, &hdr); err != nil {
		return nil, fmt.Errorf("pti (legacy format): header parse: %w", err)
	}

	sampleRate := sampleRateFromPTIIndex(hdr.SampleRateIdx)
	pcmData := data[392:]

	sampleCount := len(pcmData) / 2
	pcm := make([]float32, sampleCount)
	for i := 0; i < sampleCount; i++ {
		if i*2+1 >= len(pcmData) {
			return nil, fmt.Errorf("pti (legacy format): PCM truncated at sample %d", i)
		}
		val := int16(binary.LittleEndian.Uint16(pcmData[i*2:]))
		pcm[i] = float32(val) / 32768.0
	}

	frameCount := sampleCount
	name := nullTerminatedString(hdr.Name[:])

	meta := RexMetadata{
		SampleRate:  sampleRate,
		Channels:    1,
		BitDepth:    16,
		CreatorName: name,
	}

	cues := []WavCueMarker{
		{SliceID: 0, Position: 0, Label: name},
	}

	return []SliceExtraction{
		{
			Metadata:    meta,
			CuePoints:   cues,
			Interleaved: pcm,
			TotalFrames: frameCount,
		},
	}, nil
}

func sampleRateFromPTIIndex(idx uint8) int {
	rates := []int{8000, 11025, 16000, 22050, 32000, 44100, 48000, 96000}
	if int(idx) < len(rates) {
		return rates[idx]
	}
	return 44100
}

func nullTerminatedString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func init() {
	RegisterReader(&PTIReader{})
}
