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
	if len(data) < 4 {
		return nil, fmt.Errorf("pti: too short")
	}
	if string(data[:4]) != "PTI\x00" {
		return nil, fmt.Errorf("pti: invalid magic")
	}
	return &RexMetadata{SampleRate: 44100, Channels: 1}, nil
}

func (r *PTIReader) SupportedExtensions() []string {
	return []string{".pti"}
}

func (r *PTIReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if len(data) < 392 {
		return nil, fmt.Errorf("pti: file too short (%d bytes, need 392+ for header)", len(data))
	}
	if string(data[:4]) != "PTI\x00" {
		return nil, fmt.Errorf("pti: invalid magic bytes")
	}

	var hdr PTIHeader
	if err := binary.Read(bytes.NewReader(data[:392]), binary.LittleEndian, &hdr); err != nil {
		return nil, fmt.Errorf("pti: header parse: %w", err)
	}

	sampleRate := sampleRateFromPTIIndex(hdr.SampleRateIdx)
	pcmData := data[392:]

	sampleCount := len(pcmData) / 2
	pcm := make([]float32, sampleCount)
	for i := 0; i < sampleCount; i++ {
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
