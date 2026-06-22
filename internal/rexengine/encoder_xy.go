package rexengine

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
)

func EncodeXYPreset(w io.Writer, extraction *SliceExtraction) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode XY preset: extraction data is empty")
	}

	zw := zip.NewWriter(w)

	slices := splitExtractionIntoSlices(extraction)
	if len(slices) > 24 {
		slices = slices[:24]
	}

	patchJSON := buildXYPatchJSON(slices)
	h := &zip.FileHeader{
		Name:   "patch.json",
		Method: zip.Deflate,
	}
	fw, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	if _, err := fw.Write([]byte(patchJSON)); err != nil {
		return err
	}

	for i, s := range slices {
		var wavBuf bytes.Buffer
		if err := EncodeWavContainer(&writeSeekBuffer{Buffer: &wavBuf}, &s, 16); err != nil {
			return err
		}

		filename := fmt.Sprintf("slice_%02d.wav", i+1)
		h := &zip.FileHeader{
			Name:   filename,
			Method: zip.Store,
		}
		fw, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err := fw.Write(wavBuf.Bytes()); err != nil {
			return err
		}
	}

	return zw.Close()
}

func buildXYPatchJSON(slices []SliceExtraction) string {

	json := `{`
	json += `"engine":{"bendrange":8191,"highpass":0,` +
		`"modulation":{"aftertouch":{"amount":16384,"target":0}},` +
		`"params":[16384,16384,16384,16384,16384,16384,16384,16384],` +
		`"playmode":"poly","transpose":0,"tuning":{"root":0,"scale":0},` +
		`"velocity":{"sensitivity":19660},"volume":28505,"width":0},`
	json += `"envelope":{"amp":{"attack":0,"decay":0,"release":32767,"sustain":32604},` +
		`"filter":{"attack":0,"decay":0,"release":32767,"sustain":32604}},`
	json += `"fx":{"active":false},"lfo":{"active":false},`
	json += `"octave":0,"platform":"OP-XY","type":"drum","version":4,`
	json += `"regions":[`

	for i, s := range slices {
		if i > 0 {
			json += ","
		}
		key := 53 + i
		frameCount := s.TotalFrames
		var endFrame uint32
		if len(s.CuePoints) > 1 {
			endFrame = s.CuePoints[1].Position
		} else {
			endFrame = uint32(frameCount)
		}

		json += `{`
		json += `"fade.in":0,"fade.out":0,`
		json += `"framecount":` + fmt.Sprintf("%d", frameCount) + `,`
		json += `"gain":0,`
		json += `"hikey":` + fmt.Sprintf("%d", key) + `,`
		json += `"lokey":` + fmt.Sprintf("%d", key) + `,`
		json += `"pan":0,`
		json += `"pitch.keycenter":60,`
		json += `"playmode":"oneshot",`
		json += `"reverse":false,`
		json += `"sample":"slice_` + fmt.Sprintf("%02d", i+1) + `.wav",`
		json += `"sample.end":` + fmt.Sprintf("%d", endFrame) + `,`
		json += `"sample.start":0,`
		json += `"transpose":0,`
		json += `"tune":0`
		json += `}`
	}

	json += `]}`
	return json
}

func splitExtractionIntoSlices(extraction *SliceExtraction) []SliceExtraction {
	numCuePoints := len(extraction.CuePoints)
	if numCuePoints == 0 {
		return []SliceExtraction{*extraction}
	}

	ch := extraction.Metadata.Channels
	totalFrames := extraction.TotalFrames
	data := extraction.Interleaved

	var slices []SliceExtraction
	for i, cp := range extraction.CuePoints {
		startFrame := int(cp.Position)
		var endFrame int
		if i+1 < numCuePoints {
			endFrame = int(extraction.CuePoints[i+1].Position)
		} else {
			endFrame = totalFrames
		}
		if startFrame > totalFrames {
			startFrame = totalFrames
		}
		if endFrame > totalFrames {
			endFrame = totalFrames
		}
		if endFrame <= startFrame {
			endFrame = startFrame + 1
		}
		frameLen := endFrame - startFrame

		slicePCM := make([]float32, frameLen*ch)
		copy(slicePCM, data[startFrame*ch:(startFrame+frameLen)*ch])

		slices = append(slices, SliceExtraction{
			Metadata:    extraction.Metadata,
			CuePoints:   []WavCueMarker{{SliceID: 0, Position: 0, Label: fmt.Sprintf("Slice %02d", i+1)}},
			Interleaved: slicePCM,
			TotalFrames: frameLen,
		})
	}

	return slices
}
