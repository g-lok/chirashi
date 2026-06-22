package rexengine

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"

	"github.com/mewkiz/flac"
)

type xrniInstrument struct {
	XMLName xml.Name `xml:"RenoiseInstrument"`
	Name    string   `xml:"Name"`
	Generator struct {
		Samples []xrniSample `xml:"Samples>Sample"`
	} `xml:"SampleGenerator"`
}

type xrniSample struct {
	IsAlias    bool       `xml:"IsAlias"`
	FileName   string     `xml:"FileName"`
	SliceMarks []xrniMark `xml:"SliceMarkers>SliceMarker"`
	Mapping    struct {
		BaseNote int `xml:"BaseNote"`
	} `xml:"Mapping"`
	LoopEnd int `xml:"LoopEnd"`
}

type xrniMark struct {
	SamplePosition int `xml:"SamplePosition"`
}

type XRNIReader struct{}

func (r *XRNIReader) Probe(data []byte) (*RexMetadata, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if strings.EqualFold(filepath.Base(f.Name), "instrument.xml") {
			return &RexMetadata{SampleRate: 44100, Channels: 2}, nil
		}
	}
	return nil, fmt.Errorf("no instrument.xml in ZIP")
}

func (r *XRNIReader) SupportedExtensions() []string {
	return []string{".xrni"}
}

func (r *XRNIReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("xrni: invalid zip: %w", err)
	}

	var instr xrniInstrument
	var audioZipFile *zip.File
	for _, f := range zr.File {
		base := strings.ToLower(filepath.Base(f.Name))
		if base == "instrument.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("xrni: open instrument.xml: %w", err)
			}
			xmlData, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("xrni: read instrument.xml: %w", err)
			}
			if err := xml.Unmarshal(xmlData, &instr); err != nil {
				return nil, fmt.Errorf("xrni: parse instrument.xml: %w", err)
			}
		} else if strings.HasPrefix(base, "sampledata") || strings.Contains(base, ".flac") || strings.Contains(base, ".wav") || strings.Contains(base, ".aif") {
			audioZipFile = f
		}
	}

	if instr.Generator.Samples == nil {
		return nil, fmt.Errorf("xrni: no samples found")
	}

	var primarySample *xrniSample
	var aliasSamples []xrniSample
	for i := range instr.Generator.Samples {
		s := &instr.Generator.Samples[i]
		if !s.IsAlias {
			primarySample = s
		} else {
			aliasSamples = append(aliasSamples, *s)
		}
	}

	if primarySample == nil {
		return nil, fmt.Errorf("xrni: no primary sample with audio data")
	}

	if primarySample.FileName != "" && audioZipFile == nil {
		audioName := filepath.Base(primarySample.FileName)
		for _, f := range zr.File {
			if strings.HasSuffix(strings.ToLower(f.Name), strings.ToLower(audioName)) {
				audioZipFile = f
				break
			}
		}
		if audioZipFile == nil {
			for _, f := range zr.File {
				if !f.FileInfo().IsDir() {
					audioZipFile = f
					break
				}
			}
		}
	}

	if audioZipFile == nil {
		return nil, fmt.Errorf("xrni: no audio file found in zip")
	}

	rc, err := audioZipFile.Open()
	if err != nil {
		return nil, fmt.Errorf("xrni: open audio: %w", err)
	}
	defer rc.Close()

	audioData, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("xrni: read audio: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(audioZipFile.Name))
	pcm, sampleRate, channels, err := decodeAudio(audioData, ext)
	if err != nil {
		return nil, fmt.Errorf("xrni: decode audio: %w", err)
	}

	slicePositions := make([]int, 0, len(primarySample.SliceMarks)+1)
	for _, m := range primarySample.SliceMarks {
		slicePositions = append(slicePositions, m.SamplePosition)
	}

	totalFrames := len(pcm) / channels
	slices := make([]SliceExtraction, 0, len(slicePositions)+1)

	startPos := 0
	sliceIdx := 0
	for _, endPos := range slicePositions {
		if endPos > totalFrames {
			endPos = totalFrames
		}
		if endPos <= startPos {
			startPos = endPos
			sliceIdx++
			continue
		}
		frameCount := endPos - startPos
		slicePCM := make([]float32, frameCount*channels)
		copy(slicePCM, pcm[startPos*channels:endPos*channels])

		meta := RexMetadata{
			SampleRate: sampleRate,
			Channels:   channels,
			BitDepth:   16,
		}

		cues := []WavCueMarker{
			{SliceID: sliceIdx, Position: 0, Label: fmt.Sprintf("Slice %02d", sliceIdx+1)},
		}

		note := primarySample.Mapping.BaseNote + sliceIdx
		if sliceIdx < len(aliasSamples) {
			note = aliasSamples[sliceIdx].Mapping.BaseNote
		}
		if note > 0 {
			meta.CreatorName = fmt.Sprintf("MIDI %d", note)
		}

		slices = append(slices, SliceExtraction{
			Metadata:    meta,
			CuePoints:   cues,
			Interleaved: slicePCM,
			TotalFrames: frameCount,
		})

		startPos = endPos
		sliceIdx++
	}

	if startPos < totalFrames {
		frameCount := totalFrames - startPos
		slicePCM := make([]float32, frameCount*channels)
		copy(slicePCM, pcm[startPos*channels:])

		meta := RexMetadata{
			SampleRate: sampleRate,
			Channels:   channels,
			BitDepth:   16,
		}
		cues := []WavCueMarker{
			{SliceID: sliceIdx, Position: 0, Label: fmt.Sprintf("Slice %02d", sliceIdx+1)},
		}
		if sliceIdx < len(aliasSamples) {
			note := aliasSamples[sliceIdx].Mapping.BaseNote
			if note > 0 {
				meta.CreatorName = fmt.Sprintf("MIDI %d", note)
			}
		}

		slices = append(slices, SliceExtraction{
			Metadata:    meta,
			CuePoints:   cues,
			Interleaved: slicePCM,
			TotalFrames: frameCount,
		})
	}

	if len(slices) == 0 {
		meta := RexMetadata{
			SampleRate: sampleRate,
			Channels:   channels,
			BitDepth:   16,
		}
		cues := []WavCueMarker{
			{SliceID: 0, Position: 0, Label: "Slice 01"},
		}
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

func decodeAudio(data []byte, ext string) (pcm []float32, sampleRate, channels int, err error) {
	switch ext {
	case ".flac":
		return decodeFLAC(data)
	case ".wav":
		return decodeWAVPCM(data)
	case ".aif", ".aiff":
		return decodeAIFFPCM(data)
	default:
		return decodeFLAC(data)
	}
}

func decodeFLAC(data []byte) ([]float32, int, int, error) {
	stream, err := flac.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("flac parse: %w", err)
	}
	defer stream.Close()

	sampleRate := int(stream.Info.SampleRate)
	channels := int(stream.Info.NChannels)
	totalSamples := int(stream.Info.NSamples)
	if totalSamples <= 0 {
		totalSamples = int(stream.Info.NSamples)
	}

	bits := stream.Info.BitsPerSample
	if bits < 1 {
		bits = 16
	}
	maxVal := float32(int32(1) << (bits - 1))
	pcm := make([]float32, 0, totalSamples)
	for {
		frame, err := stream.ParseNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, 0, fmt.Errorf("flac frame: %w", err)
		}
		if len(frame.Subframes) != channels {
			return nil, 0, 0, fmt.Errorf("flac: channel count mismatch")
		}
		for s := 0; s < int(frame.BlockSize); s++ {
			for ch := 0; ch < channels; ch++ {
				sample := frame.Subframes[ch].Samples[s]
				pcm = append(pcm, float32(sample)/maxVal)
			}
		}
	}
	if len(pcm) == 0 {
		return nil, 0, 0, fmt.Errorf("flac: no samples decoded")
	}
	return pcm, sampleRate, channels, nil
}

func decodeWAVPCM(data []byte) ([]float32, int, int, error) {
	if len(data) < 12 {
		return nil, 0, 0, fmt.Errorf("wav: too short")
	}
	if string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, 0, fmt.Errorf("wav: invalid header")
	}

	var sampleRate, channels, bitDepth int
	var pcmData []byte

	pos := 12
	for pos+8 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		if chunkSize > len(data)-pos-8 {
			break
		}
		chunkData := data[pos+8 : pos+8+chunkSize]

		switch chunkID {
		case "fmt ":
			if len(chunkData) < 16 {
				return nil, 0, 0, fmt.Errorf("wav: fmt chunk too small")
			}
			audioFormat := binary.LittleEndian.Uint16(chunkData[0:2])
			if audioFormat != 1 && audioFormat != 3 {
				return nil, 0, 0, fmt.Errorf("wav: unsupported format %d", audioFormat)
			}
			channels = int(binary.LittleEndian.Uint16(chunkData[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(chunkData[4:8]))
			bitDepth = int(binary.LittleEndian.Uint16(chunkData[14:16]))
		case "data":
			pcmData = chunkData
		}

		if chunkSize%2 == 1 {
			chunkSize++
		}
		pos += 8 + chunkSize
	}

	if pcmData == nil || sampleRate == 0 {
		return nil, 0, 0, fmt.Errorf("wav: missing fmt or data chunk")
	}

	totalSamples := len(pcmData) / (bitDepth / 8)
	pcm := make([]float32, totalSamples)

	switch bitDepth {
	case 8:
		for i := range pcm {
			pcm[i] = float32(int(pcmData[i])-128) / 128.0
		}
	case 16:
		for i := 0; i < len(pcmData)-1; i += 2 {
			val := int16(binary.LittleEndian.Uint16(pcmData[i:]))
			pcm[i/2] = float32(val) / 32768.0
		}
	case 24:
		for i := 0; i < len(pcmData)-2; i += 3 {
			val := int32(int8(pcmData[i+2]))<<16 | int32(pcmData[i+1])<<8 | int32(pcmData[i])
			pcm[i/3] = float32(val) / 8388608.0
		}
	case 32:
		for i := 0; i < len(pcmData)-3; i += 4 {
			val := int32(binary.LittleEndian.Uint32(pcmData[i:]))
			pcm[i/4] = float32(val) / 2147483648.0
		}
	default:
		return nil, 0, 0, fmt.Errorf("wav: unsupported bit depth %d", bitDepth)
	}

	return pcm, sampleRate, channels, nil
}

func decodeAIFFPCM(data []byte) ([]float32, int, int, error) {
	if len(data) < 12 {
		return nil, 0, 0, fmt.Errorf("aiff: too short")
	}
	if string(data[:4]) != "FORM" {
		return nil, 0, 0, fmt.Errorf("aiff: invalid FORM")
	}

	formType := string(data[8:12])
	if formType != "AIFF" && formType != "AIFC" {
		return nil, 0, 0, fmt.Errorf("aiff: unsupported form %s", formType)
	}

	var sampleRate int
	var channels int
	var bitDepth int
	var ssndData []byte
	var ssndOffset int

	pos := 12
	for pos+8 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		if chunkSize < 0 || chunkSize > len(data)-pos-8 {
			break
		}
		chunkData := data[pos+8 : pos+8+chunkSize]

		switch chunkID {
		case "COMM":
			if len(chunkData) < 18 {
				return nil, 0, 0, fmt.Errorf("aiff: COMM too small")
			}
			channels = int(binary.BigEndian.Uint16(chunkData[0:2]))
			frameCount := int(binary.BigEndian.Uint32(chunkData[2:6]))
			bitDepth = int(binary.BigEndian.Uint16(chunkData[6:8]))
			rateBits := binary.BigEndian.Uint16(chunkData[8:10])
			rateFrac := binary.BigEndian.Uint64(chunkData[10:18])
			sampleRate = int(float64(rateBits) + float64(rateFrac)/math.Pow(2, 64))
			_ = frameCount
		case "SSND":
			ssndOffset = int(binary.BigEndian.Uint32(chunkData[0:4]))
			blockSize := int(binary.BigEndian.Uint32(chunkData[4:8]))
			_ = blockSize
			ssndData = chunkData[8+ssndOffset:]
		}

		if chunkSize%2 == 1 {
			chunkSize++
		}
		pos += 8 + chunkSize
	}

	if ssndData == nil || sampleRate == 0 {
		return nil, 0, 0, fmt.Errorf("aiff: missing SSND or COMM chunk")
	}

	totalSamples := len(ssndData) / (bitDepth / 8)
	pcm := make([]float32, totalSamples)

	switch bitDepth {
	case 8:
		for i := range pcm {
			pcm[i] = float32(int8(ssndData[i])) / 128.0
		}
	case 16:
		for i := 0; i < len(ssndData)-1; i += 2 {
			val := int16(binary.BigEndian.Uint16(ssndData[i:]))
			pcm[i/2] = float32(val) / 32768.0
		}
	case 24:
		for i := 0; i < len(ssndData)-2; i += 3 {
			val := int32(int8(ssndData[i]))<<16 | int32(ssndData[i+1])<<8 | int32(ssndData[i+2])
			pcm[i/3] = float32(val) / 8388608.0
		}
	default:
		return nil, 0, 0, fmt.Errorf("aiff: unsupported bit depth %d", bitDepth)
	}

	return pcm, sampleRate, channels, nil
}

func init() {
	RegisterReader(&XRNIReader{})
}
