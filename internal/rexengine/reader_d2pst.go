package rexengine

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
	Name        string `json:"name"`
	SampleRate  int    `json:"sampleRate"`
	KeyZones    []struct {
		Note int `json:"note"`
		Name string `json:"name"`
	} `json:"keyZones"`
}

type D2PSTReader struct{}

func (r *D2PSTReader) Probe(data []byte) (*RexMetadata, error) {
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
	return nil, fmt.Errorf("d2pst: no manifest.json found")
}

func (r *D2PSTReader) SupportedExtensions() []string {
	return []string{".d2pst"}
}

func (r *D2PSTReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("d2pst: invalid zip: %w", err)
	}

	var manifestData, wavData, tlvData []byte

	for _, f := range zr.File {
		base := strings.ToLower(filepath.Base(f.Name))
		rc, err := f.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		switch {
		case base == "manifest.json":
			manifestData = content
		case base == "sample.wav":
			wavData = content
		case strings.HasSuffix(base, ".tlv"):
			tlvData = content
		}
	}

	if wavData == nil {
		return nil, fmt.Errorf("d2pst: no sample.wav found in zip")
	}

	var manifest DT2Manifest
	if manifestData != nil {
		json.Unmarshal(manifestData, &manifest)
	}

	sr, ch, bd, err := readWAVFullFmt(wavData)
	if err != nil {
		return nil, fmt.Errorf("d2pst: wav parse: %w", err)
	}

	pcmRaw, err := readWAVData(wavData)
	if err != nil {
		return nil, fmt.Errorf("d2pst: wav data: %w", err)
	}

	pcm, err := decodeWAVSamples(pcmRaw, bd)
	if err != nil {
		return nil, fmt.Errorf("d2pst: wav decode: %w", err)
	}

	totalFrames := len(pcm) / ch

	meta := RexMetadata{
		SampleRate:  sr,
		Channels:    ch,
		BitDepth:    bd,
		CreatorName: manifest.Name,
	}

	if len(tlvData) > 0 {
		slicePositions := parseDT2TLV(tlvData, int(sr))
		if len(slicePositions) > 0 {
			slices := make([]SliceExtraction, 0, len(slicePositions)+1)
			startPos := 0
			for idx, endPos := range slicePositions {
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
			if len(slices) > 0 {
				return slices, nil
			}
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

func parseDT2TLV(tlvData []byte, sampleRate int) []int {
	var positions []int
	pos := 0
	for pos+4 <= len(tlvData) {
		tag := binary.BigEndian.Uint16(tlvData[pos : pos+2])
		length := int(binary.BigEndian.Uint16(tlvData[pos+2 : pos+4]))
		if length < 0 || pos+4+length > len(tlvData) {
			break
		}
		_ = tag
		pos += 4 + length
	}
	return positions
}

func init() {
	RegisterReader(&D2PSTReader{})
}
