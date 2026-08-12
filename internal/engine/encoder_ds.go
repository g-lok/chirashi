package engine

import (
	"encoding/xml"
	"io"
)

type dsSample struct {
	Path     string `xml:"path,attr"`
	RootNote int    `xml:"rootNote,attr"`
	LoNote   int    `xml:"loNote,attr"`
	HiNote   int    `xml:"hiNote,attr"`
	Start    uint32 `xml:"start,attr"`
	End      int    `xml:"end,attr"`
}

type dsGroup struct {
	Samples []dsSample `xml:"sample"`
}

type dsGroups struct {
	Groups []dsGroup `xml:"group"`
}

type dsPreset struct {
	XMLName xml.Name `xml:"DecentSampler"`
	Groups  dsGroups `xml:"groups"`
}

// EncodeDecentSampler writes a .dspreset XML file mapping slices.
func EncodeDecentSampler(w io.Writer, extraction *SliceExtraction, wavName string) error {
	preset := dsPreset{}
	group := dsGroup{}

	startNote := 24
	for i, cp := range extraction.CuePoints {
		note := startNote + i
		if note > 127 {
			break
		}

		end := extraction.TotalFrames
		if i+1 < len(extraction.CuePoints) {
			end = int(extraction.CuePoints[i+1].Position)
		}

		group.Samples = append(group.Samples, dsSample{
			Path:     wavName,
			RootNote: note,
			LoNote:   note,
			HiNote:   note,
			Start:    cp.Position,
			End:      end,
		})
	}

	preset.Groups.Groups = append(preset.Groups.Groups, group)

	_, err := io.WriteString(w, xml.Header)
	if err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(preset)
}
