package engine

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

type XYPresetJSON struct {
	Regions []XYRegion `json:"regions"`
}

type XYRegion struct {
	Name       string `json:"name"`
	RootNote   int    `json:"rootNote"`
	SampleStart int   `json:"sampleStart"`
	SampleEnd  int    `json:"sampleEnd"`
}

type XYReader struct{}

func (r *XYReader) Probe(data []byte) (*RexMetadata, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if strings.EqualFold(filepath.Base(f.Name), "patch.json") {
			return &RexMetadata{SampleRate: 44100, Channels: 2}, nil
		}
	}
	return nil, fmt.Errorf("xy: no patch.json found")
}

func (r *XYReader) SupportedExtensions() []string {
	return []string{".xy"}
}

func (r *XYReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("xy: invalid zip: %w", err)
	}

	var patchData []byte
	var wavFiles []*zip.File

	for _, f := range zr.File {
		base := strings.ToLower(filepath.Base(f.Name))
		if base == "patch.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("xy: open patch.json: %w", err)
			}
			patchData, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("xy: read patch.json: %w", err)
			}
		} else if strings.HasSuffix(base, ".wav") {
			wavFiles = append(wavFiles, f)
		}
	}

	if patchData == nil {
		return nil, fmt.Errorf("xy: no patch.json found in zip")
	}

	var preset XYPresetJSON
	if err := json.Unmarshal(patchData, &preset); err != nil {
		return nil, fmt.Errorf("xy: parse patch.json: %w", err)
	}

	sort.Slice(wavFiles, func(i, j int) bool {
		return wavFiles[i].Name < wavFiles[j].Name
	})

	meta := RexMetadata{
		SampleRate: 44100,
		Channels:   2,
		BitDepth:   16,
	}

	slices := make([]SliceExtraction, 0, len(preset.Regions))

	for i, region := range preset.Regions {
		if i < len(wavFiles) {
			rc, err := wavFiles[i].Open()
			if err != nil {
				continue
			}
			wavData, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			sr, ch, bd, err := readWAVFullFmt(wavData)
			if err != nil {
				continue
			}
			pcmRaw, err := readWAVData(wavData)
			if err != nil {
				continue
			}
			pcm, err := decodeWAVSamples(pcmRaw, bd)
			if err != nil {
				continue
			}

			totalFrames := len(pcm) / ch
			sliceMeta := meta
			sliceMeta.SampleRate = sr
			sliceMeta.Channels = ch
			sliceMeta.BitDepth = bd
			if region.Name != "" {
				sliceMeta.CreatorName = region.Name
			}

			label := region.Name
			if label == "" {
				label = fmt.Sprintf("Slice %02d", i+1)
			}

			slices = append(slices, SliceExtraction{
				Metadata:    sliceMeta,
				CuePoints:   []WavCueMarker{{SliceID: len(slices), Position: 0, Label: label}},
				Interleaved: pcm,
				TotalFrames: totalFrames,
			})
		}
	}

	if len(slices) == 0 {
		return nil, fmt.Errorf("xy: no slices could be decoded")
	}

	return slices, nil
}

func init() {
	RegisterReader(&XYReader{})
}
