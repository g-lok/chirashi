package engine

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type DT2Manifest struct {
	Name       string          `json:"name"`
	SampleRate int             `json:"sampleRate"`
	Payload    string          `json:"Payload"`
	KeyZones   []struct {
		Note int    `json:"note"`
		Name string `json:"name"`
	} `json:"keyZones"`
	Samples []struct {
		FileName string `json:"FileName"`
	} `json:"Samples"`
}

type DT2PSTReader struct{}

func (r *DT2PSTReader) Probe(data []byte) (*RexMetadata, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		base := strings.ToLower(filepath.Base(f.Name))
		if base == "manifest.json" {
			return &RexMetadata{SampleRate: 48000, Channels: 2}, nil
		}
	}
	return nil, fmt.Errorf("dt2pst: no manifest.json found")
}

func (r *DT2PSTReader) SupportedExtensions() []string {
	return []string{".dt2pst"}
}

func (r *DT2PSTReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("dt2pst: invalid zip: %w", err)
	}

	var manifest DT2Manifest
	var manifestData []byte

	// First pass: find manifest
	for _, f := range zr.File {
		if strings.ToLower(filepath.Base(f.Name)) == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			manifestData, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			json.Unmarshal(manifestData, &manifest)
			break
		}
	}

	// Determine expected WAV path from manifest
	wantWAV := ""
	if len(manifest.Samples) > 0 && manifest.Samples[0].FileName != "" {
		wantWAV = manifest.Samples[0].FileName
	}
	wantPayload := manifest.Payload

	var wavData, payloadData []byte

	for _, f := range zr.File {
		base := strings.ToLower(filepath.Base(f.Name))
		lowerFull := strings.ToLower(f.Name)

		// Match WAV: exact path from manifest, or basename "sample.wav" fallback
		if (wantWAV != "" && lowerFull == strings.ToLower(wantWAV)) ||
			(wantWAV == "" && base == "sample.wav") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			wavData, _ = io.ReadAll(rc)
			rc.Close()
			continue
		}

		// Skip manifest itself
		if base == "manifest.json" {
			continue
		}

		// Matching payload by basename or full path
		if wantPayload != "" && base == strings.ToLower(filepath.Base(wantPayload)) {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			payloadData, _ = io.ReadAll(rc)
			rc.Close()
			continue
		}

		// Fallback: first non-manifest non-wav file is payload
		if payloadData == nil && wavData != nil {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			payloadData, _ = io.ReadAll(rc)
			rc.Close()
		}
	}

	// Still no WAV? Try manifest's basename
	if wavData == nil && wantWAV != "" {
		wantBase := strings.ToLower(filepath.Base(wantWAV))
		for _, f := range zr.File {
			if strings.ToLower(filepath.Base(f.Name)) == wantBase {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				wavData, _ = io.ReadAll(rc)
				rc.Close()
				break
			}
		}
	}

	if wavData == nil {
		return nil, fmt.Errorf("dt2pst: no sample WAV found in zip")
	}

	sr, ch, bd, err := readWAVFullFmt(wavData)
	if err != nil {
		return nil, fmt.Errorf("dt2pst: wav parse: %w", err)
	}

	pcmRaw, err := readWAVData(wavData)
	if err != nil {
		return nil, fmt.Errorf("dt2pst: wav data: %w", err)
	}

	pcm, err := decodeWAVSamples(pcmRaw, bd)
	if err != nil {
		return nil, fmt.Errorf("dt2pst: wav decode: %w", err)
	}

	totalFrames := len(pcm) / ch

	meta := RexMetadata{
		SampleRate:  sr,
		Channels:    ch,
		BitDepth:    bd,
		CreatorName: manifest.Name,
	}

	if len(payloadData) > 0 {
		slicePositions := parseDT2PresetPositions(payloadData, int(sr))
		if len(slicePositions) > 0 {
			return buildSlicesFromPositions(pcm, ch, totalFrames, slicePositions, manifest, meta), nil
		}
	}

	cues := []WavCueMarker{{SliceID: 0, Position: 0, Label: "Slice 01"}}
	return []SliceExtraction{
		{
			Metadata:    meta,
			CuePoints:   cues,
			Interleaved: pcm,
			TotalFrames: totalFrames,
		},
	}, nil
}

func parseDT2PresetPositions(data []byte, sampleRate int) []int {
	var positions []int
	pos := 0
	for pos+8 <= len(data) {
		if data[pos] == 0x00 && data[pos+1] == 0x22 {
			posVal := binary.LittleEndian.Uint32(data[pos+2 : pos+6])
			if data[pos+6] == 0x00 && data[pos+7] == 0x08 {
				positions = append(positions, int(posVal))
				pos += 8
				continue
			}
		}
		pos++
	}
	return positions
}

func buildSlicesFromPositions(pcm []float32, ch, totalFrames int, positions []int, manifest DT2Manifest, meta RexMetadata) []SliceExtraction {
	slices := make([]SliceExtraction, 0, len(positions)+1)
	startPos := 0
	for idx, endPos := range positions {
		if endPos > totalFrames {
			endPos = totalFrames
		}
		if endPos <= startPos {
			startPos = endPos
			continue
		}
		frameCount := endPos - startPos
		slicePCM := make([]float32, frameCount*ch)
		copy(slicePCM, pcm[startPos*ch:endPos*ch])

		label := fmt.Sprintf("Slice %02d", idx+1)
		if idx < len(manifest.KeyZones) {
			label = manifest.KeyZones[idx].Name
		}

		slices = append(slices, SliceExtraction{
			Metadata:    meta,
			CuePoints:   []WavCueMarker{{SliceID: len(slices), Position: 0, Label: label}},
			Interleaved: slicePCM,
			TotalFrames: frameCount,
		})
		startPos = endPos
	}
	if startPos < totalFrames {
		frameCount := totalFrames - startPos
		slicePCM := make([]float32, frameCount*ch)
		copy(slicePCM, pcm[startPos*ch:])
		slices = append(slices, SliceExtraction{
			Metadata:    meta,
			CuePoints:   []WavCueMarker{{SliceID: len(slices), Position: 0, Label: fmt.Sprintf("Slice %02d", len(slices)+1)}},
			Interleaved: slicePCM,
			TotalFrames: frameCount,
		})
	}
	return slices
}

func init() {
	RegisterReader(&DT2PSTReader{})
}
