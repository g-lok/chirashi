package engine

import (
	"encoding/xml"
	"io"
)

type xpmLayer struct {
	Number      int    `xml:"number,attr"`
	Active      bool   `xml:"Active"`
	Volume      float64 `xml:"Volume"`
	Pan         float64 `xml:"Pan"`
	Pitch       float64 `xml:"Pitch"`
	SampleName  string `xml:"SampleName"`
	SampleStart int    `xml:"SampleStart"`
	SampleEnd   int    `xml:"SampleEnd"`
	LoopStart   int    `xml:"LoopStart"`
	LoopEnd     int    `xml:"LoopEnd"`
	LoopMode    int    `xml:"LoopMode"` // 0=off
}

type xpmLayers struct {
	Layers []xpmLayer `xml:"Layer"`
}

type xpmInstrument struct {
	Number int       `xml:"number,attr"`
	Layers xpmLayers `xml:"Layers"`
}

type xpmProgram struct {
	Type         string          `xml:"type,attr"`
	ProgramName  string          `xml:"ProgramName"`
	Instruments  []xpmInstrument `xml:"Instruments>Instrument"`
}

type xpmObject struct {
	XMLName xml.Name   `xml:"MPCVObject"`
	Version string     `xml:"Version>File_Version"`
	Program xpmProgram `xml:"Program"`
}

// EncodeXPM writes an Akai MPC modern drum program (.xpm).
// It maps each slice to a pad (Instrument 1-128).
func EncodeXPM(w io.Writer, extraction *SliceExtraction, wavName string, programName string) error {
	obj := xpmObject{
		Version: "2.0",
		Program: xpmProgram{
			Type:        "DRUM",
			ProgramName: programName,
		},
	}

	for i, cp := range extraction.CuePoints {
		if i >= 128 {
			break
		}

		end := extraction.TotalFrames
		if i+1 < len(extraction.CuePoints) {
			end = int(extraction.CuePoints[i+1].Position)
		}

		inst := xpmInstrument{
			Number: i + 1,
			Layers: xpmLayers{
				Layers: []xpmLayer{
					{
						Number:      1,
						Active:      true,
						Volume:      1.0,
						SampleName:  wavName,
						SampleStart: int(cp.Position),
						SampleEnd:   end,
						LoopMode:    0,
					},
				},
			},
		}
		obj.Program.Instruments = append(obj.Program.Instruments, inst)
	}

	_, err := io.WriteString(w, xml.Header)
	if err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(obj)
}
