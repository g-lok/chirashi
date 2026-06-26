package engine

import (
	"fmt"

	"github.com/g-lok/chirashi/internal/engine/rex2"
)

func HasREX() bool { return true }

func InitEngine(verbose bool) error {
	if verbose {
		fmt.Println("Pure Go REX2 decoder initialized")
	}
	return nil
}

func CloseEngine() error {
	return nil
}

func RenderLoopPreview(fileData []byte, targetSampleRate, tempo int) (*SliceExtraction, error) {
	f, err := rex2.Decode(fileData)
	if err != nil {
		return nil, err
	}

	// Tempo stored as BPM*1000 in REX2 format, divide to get actual BPM
	sampleRate := f.Info.SampleRate
	if targetSampleRate > 0 {
		sampleRate = targetSampleRate
	}

	meta := RexMetadata{
		Channels:      f.Info.Channels,
		SampleRate:    sampleRate,
		Tempo:         float64(f.Info.Tempo) / 1000.0,
		OriginalTempo: float64(f.Info.OriginalTempo) / 1000.0,
		TimeSignNom:   f.Info.TimeSigNum,
		TimeSignDenom: f.Info.TimeSigDen,
		BitDepth:      f.Info.BitDepth,
		PPQLength:     f.Info.PPQLength,
	}

	startFrame := 0
	totalFrames := int(f.Info.TotalFrames)
	if f.Info.LoopEnd > f.Info.LoopStart {
		startFrame = int(f.Info.LoopStart)
		totalFrames = int(f.Info.LoopEnd - f.Info.LoopStart)
	}

	interleaved := make([]float32, totalFrames*f.Info.Channels)
	srcOffset := startFrame * f.Info.Channels
	for i := 0; i < totalFrames*f.Info.Channels; i++ {
		if srcOffset+i < len(f.PCM) {
			interleaved[i] = rex2.PCMToFloat32(f.PCM[srcOffset+i], f.Info.BitDepth)
		}
	}

	cuePoints := make([]WavCueMarker, len(f.Slices))
	for i, s := range f.Slices {
		actualTempo := f.Info.Tempo
		if tempo > 0 {
			actualTempo = tempo * 1000
		}
		framePos := int(float64(f.Info.SampleRate) * 1000.0 * float64(s.PPQPos) / (float64(actualTempo) * 256.0))
		if framePos > totalFrames {
			framePos = totalFrames
		}
		cuePoints[i] = WavCueMarker{
			SliceID:  i,
			Position: uint32(framePos),
			Label:    fmt.Sprintf("Slice %02d", i+1),
		}
	}

	return &SliceExtraction{
		Metadata:    meta,
		CuePoints:   cuePoints,
		Interleaved: interleaved,
		TotalFrames: totalFrames,
	}, nil
}

func RenderSlicesPreview(fileData []byte, targetSampleRate, tempo int) ([]SliceExtraction, error) {
	f, err := rex2.Decode(fileData)
	if err != nil {
		return nil, err
	}

	sampleRate := f.Info.SampleRate
	if targetSampleRate > 0 {
		sampleRate = targetSampleRate
	}

	meta := RexMetadata{
		Channels:      f.Info.Channels,
		SampleRate:    sampleRate,
		Tempo:         float64(f.Info.Tempo) / 1000.0,
		OriginalTempo: float64(f.Info.OriginalTempo) / 1000.0,
		TimeSignNom:   f.Info.TimeSigNum,
		TimeSignDenom: f.Info.TimeSigDen,
		BitDepth:      f.Info.BitDepth,
		PPQLength:     f.Info.PPQLength,
	}

	result := make([]SliceExtraction, len(f.Slices))
	for i, s := range f.Slices {
		frameLen := s.SampleLength
		totalSamples := frameLen * f.Info.Channels

		pcm := make([]float32, totalSamples)
		srcStart := s.SampleStart * f.Info.Channels
		for j := 0; j < totalSamples && srcStart+j < len(f.PCM); j++ {
			pcm[j] = rex2.PCMToFloat32(f.PCM[srcStart+j], f.Info.BitDepth)
		}

		result[i] = SliceExtraction{
			Metadata:    meta,
			CuePoints:   []WavCueMarker{{SliceID: i, Position: 0, Label: fmt.Sprintf("Slice %02d", i+1)}},
			Interleaved: pcm,
			TotalFrames: frameLen,
		}
	}

	return result, nil
}
