package engine

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type OTReader struct{}

func (r *OTReader) Probe(data []byte) (*RexMetadata, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("ot: too short (%d bytes)", len(data))
	}
	switch {
	case len(data) >= 12 && string(data[:4]) == "FORM" && string(data[8:12]) == "DPS1":
		return &RexMetadata{SampleRate: 44100, Channels: 2}, nil
	case string(data[:4]) == "OT\x00\x00":
		return &RexMetadata{SampleRate: 44100, Channels: 2}, nil
	default:
		return nil, fmt.Errorf("ot: unknown magic %02x %02x %02x %02x", data[0], data[1], data[2], data[3])
	}
}

func (r *OTReader) SupportedExtensions() []string {
	return []string{".ot"}
}

func (r *OTReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("ot: sidecar too short (%d bytes)", len(data))
	}

	magic := ""
	if len(data) >= 12 && string(data[:4]) == "FORM" && string(data[8:12]) == "DPS1" {
		magic = "FORM DPS1"
	} else if string(data[:4]) == "OT\x00\x00" {
		magic = "OT"
	} else {
		return nil, fmt.Errorf("ot: unknown magic %02x %02x %02x %02x — expected FORM/DPS1 or OT\\x00\\x00", data[0], data[1], data[2], data[3])
	}
	return nil, fmt.Errorf("ot (%s): companion WAV required — provide .ot + .wav together", magic)
}

func ReadOTWithWAV(otData, wavData []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if len(otData) < 8 {
		return nil, fmt.Errorf("ot: sidecar too short (%d bytes)", len(otData))
	}

	switch {
	case len(otData) >= 12 && string(otData[:4]) == "FORM" && string(otData[8:12]) == "DPS1":
		return readOTDPS1WithWAV(otData, wavData, targetSampleRate)
	case string(otData[:4]) == "OT\x00\x00":
		return readOTLegacyWithWAV(otData, wavData, targetSampleRate)
	default:
		return nil, fmt.Errorf("ot: unknown magic %02x %02x %02x %02x — expected FORM/DPS1 or OT\\x00\\x00", otData[0], otData[1], otData[2], otData[3])
	}
}

func readOTDPS1WithWAV(otData, wavData []byte, targetSampleRate int) ([]SliceExtraction, error) {
	const expectedSize = 0x340
	if len(otData) < expectedSize {
		return nil, fmt.Errorf("ot (FORM DPS1): file too short (%d bytes, need %d)", len(otData), expectedSize)
	}

	if string(otData[:4]) != "FORM" {
		return nil, fmt.Errorf("ot (FORM DPS1): invalid magic %02x %02x %02x %02x", otData[0], otData[1], otData[2], otData[3])
	}
	if string(otData[8:12]) != "DPS1" {
		return nil, fmt.Errorf("ot (FORM DPS1): expected DPS1 form type, got %q", string(otData[8:12]))
	}
	if string(otData[12:16]) != "SMPA" {
		return nil, fmt.Errorf("ot (FORM DPS1): expected SMPA chunk, got %q", string(otData[12:16]))
	}

	chkOff := 0x33E
	wantCS := binary.BigEndian.Uint16(otData[chkOff : chkOff+2])
	var calcCS uint16
	for i := 0x10; i < chkOff; i++ {
		calcCS += uint16(otData[i])
	}
	if wantCS != calcCS {
		return nil, fmt.Errorf("ot (FORM DPS1): checksum mismatch: file=%04x, calculated=%04x", wantCS, calcCS)
	}

	numSliceSlots := 64
	type slot struct {
		start uint32
		end   uint32
		loop  uint32
	}
	var activeSlots []slot
	for i := 0; i < numSliceSlots; i++ {
		slotOff := 58 + i*12
		if slotOff+12 > len(otData) {
			break
		}
		s := slot{
			start: binary.BigEndian.Uint32(otData[slotOff : slotOff+4]),
			end:   binary.BigEndian.Uint32(otData[slotOff+4 : slotOff+8]),
			loop:  binary.BigEndian.Uint32(otData[slotOff+8 : slotOff+12]),
		}
		if s.start == 0 && s.end == 0 {
			continue
		}
		if s.end <= s.start {
			continue
		}
		activeSlots = append(activeSlots, s)
	}

	if len(activeSlots) == 0 {
		return nil, fmt.Errorf("ot (FORM DPS1): no valid slice slots found")
	}

	sampleRate, channels, bitDepth, err := readWAVFullFmt(wavData)
	if err != nil {
		return nil, fmt.Errorf("ot (FORM DPS1): companion WAV parse: %w", err)
	}

	pcmRaw, err := readWAVData(wavData)
	if err != nil {
		return nil, fmt.Errorf("ot (FORM DPS1): companion WAV data: %w", err)
	}

	pcm, err := decodeWAVSamples(pcmRaw, bitDepth)
	if err != nil {
		return nil, fmt.Errorf("ot (FORM DPS1): companion WAV decode: %w", err)
	}

	wavTotalFrames := len(pcm) / channels

	meta := RexMetadata{
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   bitDepth,
	}

	var slices []SliceExtraction
	for i, s := range activeSlots {
		startFrame := int(s.start)
		endFrame := int(s.end)
		if endFrame > wavTotalFrames {
			endFrame = wavTotalFrames
		}
		if startFrame >= wavTotalFrames || endFrame <= startFrame {
			continue
		}
		fc := endFrame - startFrame
		if fc <= 0 {
			continue
		}

		sp := make([]float32, fc*channels)
		copy(sp, pcm[startFrame*channels:endFrame*channels])

		slices = append(slices, SliceExtraction{
			Metadata:    meta,
			CuePoints:   []WavCueMarker{{SliceID: i, Position: 0, Label: fmt.Sprintf("Slice %02d", i+1)}},
			Interleaved: sp,
			TotalFrames: fc,
		})
	}

	if len(slices) == 0 {
		cues := []WavCueMarker{{SliceID: 0, Position: 0, Label: "Slice 01"}}
		slices = []SliceExtraction{
			{
				Metadata:    meta,
				CuePoints:   cues,
				Interleaved: pcm,
				TotalFrames: wavTotalFrames,
			},
		}
	}

	return slices, nil
}

func readOTLegacyWithWAV(otData, wavData []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if len(otData) < 8 {
		return nil, fmt.Errorf("ot (legacy format): sidecar too short (%d bytes)", len(otData))
	}
	if string(otData[:4]) != "OT\x00\x00" {
		return nil, fmt.Errorf("ot (legacy format): invalid magic")
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
		return nil, fmt.Errorf("ot (legacy format): companion WAV parse: %w", err)
	}

	pcmRaw, err := readWAVData(wavData)
	if err != nil {
		return nil, fmt.Errorf("ot (legacy format): companion WAV data: %w", err)
	}

	pcm, err := decodeWAVSamples(pcmRaw, bitDepth)
	if err != nil {
		return nil, fmt.Errorf("ot (legacy format): companion WAV decode: %w", err)
	}

	totalFrames := len(pcm) / channels
	bytesPerFrame := bitDepth / 8 * channels

	meta := RexMetadata{
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   bitDepth,
	}

	var slices []SliceExtraction
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
