package engine

import (
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type abletonDoc struct {
	XMLName  xml.Name          `xml:"Ableton"`
	LiveSet  *liveSet          `xml:"LiveSet"`
	Simpler  *originalSimpler  `xml:"OriginalSimpler"`
	MajorVer string            `xml:"MajorVersion,attr"`
}

type liveSet struct {
	Tracks struct {
		Track []liveTrack `xml:"MidiTrack"`
	} `xml:"Tracks"`
}

type liveTrack struct {
	Name    string  `xml:"Name>EffectiveName"`
	Devices struct {
		Simplers []originalSimpler `xml:"OriginalSimpler"`
	} `xml:"DeviceChain>Devices"`
}

type originalSimpler struct {
	Player   *player     `xml:"Player"`
	SampleRef *sampleRef `xml:"SampleRef"`
}

type player struct {
	Map *multiSampleMap `xml:"MultiSampleMap"`
}

type multiSampleMap struct {
	Parts []multiSamplePart `xml:"SampleParts>MultiSamplePart"`
}

type multiSamplePart struct {
	Name       string  `xml:"Name>Value"`
	HasSlicing string  `xml:"HasImportedSlicePoints,attr"`
	SampleRef  *sampleRef `xml:"SampleRef"`
	SlicePts   []slicePoint  `xml:"InitialSlicePointsFromOnsets>SlicePoint"`
}

type slicePoint struct {
	TimeInSeconds string `xml:"TimeInSeconds,attr"`
	Rank          string `xml:"Rank,attr"`
}

type sampleRef struct {
	FileRef struct {
		Path struct {
			Value string `xml:"Value,attr"`
		} `xml:"Path"`
		RelativePath struct {
			Value string `xml:"Value,attr"`
		} `xml:"RelativePath"`
	} `xml:"FileRef"`
	DefaultDuration   int `xml:"DefaultDuration>Value"`
	DefaultSampleRate int `xml:"DefaultSampleRate>Value"`
}

type SimplerReader struct{}

func (r *SimplerReader) Probe(data []byte) (*RexMetadata, error) {
	raw, err := gunzipMaybe(data)
	if err != nil {
		return nil, err
	}
	if isSimplerPreset(raw) {
		return &RexMetadata{SampleRate: 44100, Channels: 2}, nil
	}
	return nil, fmt.Errorf("not a Simpler preset")
}

func (r *SimplerReader) SupportedExtensions() []string {
	return []string{".adv", ".als"}
}

func (r *SimplerReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	raw, err := gunzipMaybe(data)
	if err != nil {
		return nil, fmt.Errorf("simpler: decompress: %w", err)
	}

	var doc abletonDoc
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("simpler: parse xml: %w", err)
	}

	if !isSimplerPreset(raw) {
		return nil, fmt.Errorf("simpler: not a Simpler/ALS preset")
	}

	var simpler *originalSimpler

	if doc.Simpler != nil {
		simpler = doc.Simpler
	}

	if simpler == nil && doc.LiveSet != nil {
		for i := range doc.LiveSet.Tracks.Track {
			for j := range doc.LiveSet.Tracks.Track[i].Devices.Simplers {
				simpler = &doc.LiveSet.Tracks.Track[i].Devices.Simplers[j]
				if simpler != nil {
					break
				}
			}
			if simpler != nil {
				break
			}
		}
	}

	if simpler == nil {
		return nil, fmt.Errorf("simpler: no Simpler device found")
	}

	if simpler.Player != nil && simpler.Player.Map != nil && len(simpler.Player.Map.Parts) > 0 {
		return readSimplerParts(simpler.Player.Map.Parts)
	}
	if simpler.SampleRef != nil {
		return readSimplerSingle(simpler.SampleRef)
	}

	return nil, fmt.Errorf("simpler: no sample data found")
}

func readSimplerParts(parts []multiSamplePart) ([]SliceExtraction, error) {
	var slices []SliceExtraction
	for _, part := range parts {
		pcm, sampleRate, channels, err := resolveAndDecode(part.SampleRef.FileRef.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("simpler: load sample '%s': %w", part.SampleRef.FileRef.Path.Value, err)
		}

		meta := RexMetadata{
			SampleRate: sampleRate,
			Channels:   channels,
			BitDepth:   16,
		}

		totalFrames := len(pcm) / channels

		hasSlicing := part.HasSlicing == "true"
		if !hasSlicing || len(part.SlicePts) == 0 {
			cues := []WavCueMarker{
				{SliceID: 0, Position: 0, Label: part.Name},
			}
			slices = append(slices, SliceExtraction{
				Metadata:    meta,
				CuePoints:   cues,
				Interleaved: pcm,
				TotalFrames: totalFrames,
			})
			continue
		}

		slicePositions := make([]int, 0, len(part.SlicePts))
		for _, sp := range part.SlicePts {
			t := 0.0
			fmt.Sscanf(sp.TimeInSeconds, "%f", &t)
			pos := int(t * float64(sampleRate))
			if pos < 0 {
				pos = 0
			}
			if pos > totalFrames {
				pos = totalFrames
			}
			slicePositions = append(slicePositions, pos)
		}

		startPos := 0
		for _, endPos := range slicePositions {
			if endPos <= startPos {
				startPos = endPos
				continue
			}
			frameCount := endPos - startPos
			slicePCM := make([]float32, frameCount*channels)
			copy(slicePCM, pcm[startPos*channels:endPos*channels])

			slices = append(slices, SliceExtraction{
				Metadata:    meta,
				CuePoints:   []WavCueMarker{{SliceID: len(slices), Position: 0, Label: fmt.Sprintf("%s_%02d", part.Name, len(slices)+1)}},
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
				CuePoints:   []WavCueMarker{{SliceID: len(slices), Position: 0, Label: fmt.Sprintf("%s_%02d", part.Name, len(slices)+1)}},
				Interleaved: slicePCM,
				TotalFrames: frameCount,
			})
		}
	}
	if len(slices) == 0 {
		return nil, fmt.Errorf("simpler: no slices produced")
	}
	return slices, nil
}

func readSimplerSingle(sr *sampleRef) ([]SliceExtraction, error) {
	pcm, sampleRate, channels, err := resolveAndDecode(sr.FileRef.Path.Value)
	if err != nil {
		return nil, fmt.Errorf("simpler: load sample: %w", err)
	}
	meta := RexMetadata{
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   16,
	}
	totalFrames := len(pcm) / channels
	cues := []WavCueMarker{{SliceID: 0, Position: 0, Label: "Slice 01"}}
	slices := []SliceExtraction{
		{
			Metadata:    meta,
			CuePoints:   cues,
			Interleaved: pcm,
			TotalFrames: totalFrames,
		},
	}
	return slices, nil
}

func gunzipMaybe(data []byte) ([]byte, error) {
	if len(data) < 2 {
		return data, nil
	}
	if data[0] == 0x1f && data[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		return io.ReadAll(gr)
	}
	return data, nil
}

func isSimplerPreset(data []byte) bool {
	checkLen := len(data)
	if checkLen > 2048 {
		checkLen = 2048
	}
	head := strings.ToLower(string(data[:checkLen]))
	return strings.Contains(head, "<originalsimpler")
}

func resolveAndDecode(path string) ([]float32, int, int, error) {
	if path == "" {
		return nil, 0, 0, fmt.Errorf("empty file path")
	}

	candidates := []string{path}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".wav" && ext != ".aif" && ext != ".aiff" && ext != ".flac" {
		for _, tryExt := range []string{".wav", ".aif", ".aiff", ".flac"} {
			candidates = append(candidates, path+tryExt)
		}
	}

	// Add library path variants for Ableton cross-machine resolution
	if sampleLibraryPath != "" {
		base := filepath.Base(path)
		candidates = append(candidates,
			filepath.Join(sampleLibraryPath, path),
			filepath.Join(sampleLibraryPath, "Samples", "Imported", base),
			filepath.Join(sampleLibraryPath, "Samples", base),
		)
	}

	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		return decodeAudio(data, strings.ToLower(filepath.Ext(candidate)))
	}

	return nil, 0, 0, fmt.Errorf("file not found: %s", path)
}

func init() {
	RegisterReader(&SimplerReader{})
}
