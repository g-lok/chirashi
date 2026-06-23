package engine

import (
	"encoding/xml"
	"fmt"
	"strings"
	"sync"
)

type adgDoc struct {
	XMLName     xml.Name           `xml:"Ableton"`
	GroupDevice *groupDevicePreset `xml:"GroupDevicePreset"`
}

type groupDevicePreset struct {
	Device struct {
		DrumGroup *drumGroupDevice `xml:"DrumGroupDevice"`
	} `xml:"Device"`
	BranchPresets struct {
		Branches []drumBranch `xml:"DrumBranchPreset"`
	} `xml:"BranchPresets"`
}

type drumGroupDevice struct {
	PadsWrapper *padsWrapper `xml:"DrumPadsListWrapper"`
	Branches    *struct {
		Items []drumBranch `xml:"DrumBranchPreset"`
	} `xml:"Branches"`
}

type padsWrapper struct {
	Pads []pad `xml:"Pads>Pad"`
}

type pad struct {
	MidiNote int      `xml:"MidiNote>Value"`
	Chain    padChain `xml:"Chain"`
}

type padChain struct {
	DeviceChain struct {
		Devices struct {
			Simplers []drumSimpler `xml:"OriginalSimpler"`
		} `xml:"Devices"`
	} `xml:"DeviceChain"`
}

type drumBranch struct {
	DevicePresets struct {
		Preset struct {
			Device struct {
				DrumCell *drumCell `xml:"DrumCell"`
			} `xml:"Device"`
		} `xml:"AbletonDevicePreset"`
	} `xml:"DevicePresets"`
	ZoneSettings struct {
		ReceivingNote int `xml:"Value,attr"`
	} `xml:"ZoneSettings"`
}

type attrString struct {
	Value string `xml:"Value,attr"`
}

type drumCell struct {
	UserSample struct {
		Value struct {
			SampleRef struct {
				FileRef struct {
					Path attrString `xml:"Path"`
				} `xml:"FileRef"`
			} `xml:"SampleRef"`
		} `xml:"Value"`
	} `xml:"UserSample"`
}

type drumSimpler struct {
	SampleRef struct {
		FileRef struct {
			Path attrString `xml:"Path"`
		} `xml:"FileRef"`
	} `xml:"SampleRef"`
}

type DrumRackReader struct{}

func (r *DrumRackReader) Probe(data []byte) (*RexMetadata, error) {
	raw, err := gunzipMaybe(data)
	if err != nil {
		return nil, err
	}
	head := strings.ToLower(string(raw[:min(len(raw), 512)]))
	if strings.Contains(head, "<drumgrouppadslistwrapper") || strings.Contains(head, "<drumbranchpreset") {
		return &RexMetadata{SampleRate: 44100, Channels: 2}, nil
	}
	return nil, fmt.Errorf("not a Drum Rack")
}

func (r *DrumRackReader) SupportedExtensions() []string {
	return []string{".adg"}
}

func (r *DrumRackReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	raw, err := gunzipMaybe(data)
	if err != nil {
		return nil, fmt.Errorf("drumrack: decompress: %w", err)
	}

	var doc adgDoc
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("drumrack: parse xml: %w", err)
	}

	var branches []drumBranch

	if doc.GroupDevice != nil && doc.GroupDevice.BranchPresets.Branches != nil {
		branches = doc.GroupDevice.BranchPresets.Branches
	} else if doc.GroupDevice != nil && doc.GroupDevice.Device.DrumGroup != nil {
		dg := doc.GroupDevice.Device.DrumGroup
		if dg.Branches != nil {
			branches = dg.Branches.Items
		} else if dg.PadsWrapper != nil {
			return readPadsLegacy(dg.PadsWrapper.Pads)
		}
	}

	if len(branches) == 0 {
		return nil, fmt.Errorf("drumrack: no pads found")
	}

	type padResult struct {
		slice SliceExtraction
		err   error
	}
	results := make([]padResult, len(branches))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, br := range branches {
		wg.Add(1)
		go func(idx int, branch drumBranch) {
			defer wg.Done()

			if branch.DevicePresets.Preset.Device.DrumCell == nil {
				mu.Lock()
				results[idx] = padResult{err: fmt.Errorf("branch %d: no DrumCell", idx)}
				mu.Unlock()
				return
			}

			cell := branch.DevicePresets.Preset.Device.DrumCell
			path := cell.UserSample.Value.SampleRef.FileRef.Path.Value
			if path == "" {
				mu.Lock()
				results[idx] = padResult{err: fmt.Errorf("branch %d: empty sample path", idx)}
				mu.Unlock()
				return
			}

			midiNote := branch.ZoneSettings.ReceivingNote

			pcm, sampleRate, channels, err := resolveAndDecode(path)
			if err != nil {
				mu.Lock()
				results[idx] = padResult{err: fmt.Errorf("branch %d (MIDI %d): %w", idx, midiNote, err)}
				mu.Unlock()
				return
			}

			totalFrames := len(pcm) / channels
			meta := RexMetadata{
				SampleRate:  sampleRate,
				Channels:    channels,
				BitDepth:    16,
				CreatorName: fmt.Sprintf("MIDI %d", midiNote),
			}
			cues := []WavCueMarker{
				{SliceID: 0, Position: 0, Label: fmt.Sprintf("MIDI %d", midiNote)},
			}

			mu.Lock()
			results[idx] = padResult{
				slice: SliceExtraction{
					Metadata:    meta,
					CuePoints:   cues,
					Interleaved: pcm,
					TotalFrames: totalFrames,
				},
			}
			mu.Unlock()
		}(i, br)
	}

	wg.Wait()

	var slices []SliceExtraction
	for _, r := range results {
		if r.err != nil {
			continue
		}
		slices = append(slices, r.slice)
	}

	if len(slices) == 0 {
		return nil, fmt.Errorf("drumrack: no valid pad samples found")
	}

	return slices, nil
}

func readPadsLegacy(pads []pad) ([]SliceExtraction, error) {
	if len(pads) == 0 {
		return nil, fmt.Errorf("drumrack: no legacy pads")
	}

	results := make([]SliceExtraction, 0, len(pads))
	for _, p := range pads {
		simpler := findSimplerInChain(p.Chain)
		if simpler == nil {
			continue
		}
		path := simpler.SampleRef.FileRef.Path.Value
		if path == "" {
			continue
		}

		pcm, sampleRate, channels, err := resolveAndDecode(path)
		if err != nil {
			continue
		}

		totalFrames := len(pcm) / channels
		meta := RexMetadata{
			SampleRate:  sampleRate,
			Channels:    channels,
			BitDepth:    16,
			CreatorName: fmt.Sprintf("MIDI %d", p.MidiNote),
		}
		cues := []WavCueMarker{
			{SliceID: 0, Position: 0, Label: fmt.Sprintf("MIDI %d", p.MidiNote)},
		}

		results = append(results, SliceExtraction{
			Metadata:    meta,
			CuePoints:   cues,
			Interleaved: pcm,
			TotalFrames: totalFrames,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("drumrack: no valid legacy pads")
	}
	return results, nil
}

func findSimplerInChain(chain padChain) *drumSimpler {
	for i := range chain.DeviceChain.Devices.Simplers {
		s := &chain.DeviceChain.Devices.Simplers[i]
		if s.SampleRef.FileRef.Path.Value != "" {
			return s
		}
	}
	return nil
}

func init() {
	RegisterReader(&DrumRackReader{})
}
