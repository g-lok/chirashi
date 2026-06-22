package rexengine

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type OTReader struct{}

func (r *OTReader) Probe(data []byte) (*RexMetadata, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("ot: too short")
	}
	if string(data[:4]) != "OT\x00\x00" {
		return nil, fmt.Errorf("ot: invalid magic")
	}
	return &RexMetadata{SampleRate: 44100, Channels: 2}, nil
}

func (r *OTReader) SupportedExtensions() []string {
	return []string{".ot"}
}

func (r *OTReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("ot: sidecar too short")
	}
	if string(data[:4]) != "OT\x00\x00" {
		return nil, fmt.Errorf("ot: invalid sidecar magic")
	}
	return nil, fmt.Errorf("ot: companion WAV required — provide .ot + .wav together")
}

// ReadOTWithWAV reads an OT sidecar with companion WAV data already loaded.
func ReadOTWithWAV(otData, wavData []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if len(otData) < 8 {
		return nil, fmt.Errorf("ot: sidecar too short")
	}
	if string(otData[:4]) != "OT\x00\x00" {
		return nil, fmt.Errorf("ot: invalid sidecar magic")
	}

	sliceCount := int(binary.BigEndian.Uint16(otData[6:8]))
	if sliceCount > 64 {
		sliceCount = 64
	}
	if sliceCount == 0 {
		sliceCount = 1
	}

	startBytes := make([]uint32, sliceCount)
	endBytes := make([]uint32, sliceCount)
	for i := 0; i < sliceCount; i++ {
		if 8+i*4+4 <= len(otData) {
			startBytes[i] = binary.BigEndian.Uint32(otData[8+i*4 : 12+i*4])
		}
		if 264+i*4+4 <= len(otData) {
			endBytes[i] = binary.BigEndian.Uint32(otData[264+i*4 : 268+i*4])
		}
	}

	sampleRate, channels, bitDepth, err := readWAVFullFmt(wavData)
	if err != nil {
		return nil, fmt.Errorf("ot: companion WAV parse: %w", err)
	}

	pcmRaw, err := readWAVData(wavData)
	if err != nil {
		return nil, fmt.Errorf("ot: companion WAV data: %w", err)
	}

	pcm, err := decodeWAVSamples(pcmRaw, bitDepth)
	if err != nil {
		return nil, fmt.Errorf("ot: companion WAV decode: %w", err)
	}

	totalFrames := len(pcm) / channels
	bytesPerFrame := bitDepth / 8 * channels

	meta := RexMetadata{
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   bitDepth,
	}

	slices := make([]SliceExtraction, 0, sliceCount)
	for i := 0; i < sliceCount; i++ {
		startByte := int(startBytes[i])
		endByte := int(endBytes[i])
		if endByte <= startByte || startByte >= len(wavData) {
			continue
		}
		if endByte > len(wavData) {
			endByte = len(wavData)
		}

		startFrame := startByte / bytesPerFrame
		endFrame := endByte / bytesPerFrame
		if endFrame > totalFrames {
			endFrame = totalFrames
		}

		frameCount := endFrame - startFrame
		if frameCount <= 0 {
			continue
		}

		slicePCM := make([]float32, frameCount*channels)
		copy(slicePCM, pcm[startFrame*channels:endFrame*channels])

		slices = append(slices, SliceExtraction{
			Metadata:    meta,
			CuePoints:   []WavCueMarker{{SliceID: len(slices), Position: 0, Label: fmt.Sprintf("Slice %02d", i+1)}},
			Interleaved: slicePCM,
			TotalFrames: frameCount,
		})
	}

	if len(slices) == 0 {
		cues := []WavCueMarker{{SliceID: 0, Position: 0, Label: "Slice 01"}}
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

func findCompanionWAV(otPath string) ([]byte, error) {
	base := strings.TrimSuffix(otPath, filepath.Ext(otPath))
	wavCandidates := []string{base + ".wav", base + ".WAV"}
	for _, candidate := range wavCandidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("no companion WAV found for %s", otPath)
}

func init() {
	RegisterReader(&OTReader{})
}
