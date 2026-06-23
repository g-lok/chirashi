package engine

import (
	"fmt"
	"math"
)

func DownmixToMono(interleaved []float32, numChannels int, mode string) ([]float32, error) {
	if numChannels == 1 {
		return interleaved, nil
	}
	numFrames := len(interleaved) / numChannels
	mono := make([]float32, numFrames)

	switch mode {
	case "sum":
		for i := 0; i < numFrames; i++ {
			var sum float32
			for ch := 0; ch < numChannels; ch++ {
				sum += interleaved[i*numChannels+ch]
			}
			mono[i] = sum / float32(numChannels)
		}
	case "left":
		for i := 0; i < numFrames; i++ {
			mono[i] = interleaved[i*numChannels]
		}
	case "right":
		for i := 0; i < numFrames; i++ {
			mono[i] = interleaved[i*numChannels+1]
		}
	case "difference":
		for i := 0; i < numFrames; i++ {
			mono[i] = (interleaved[i*numChannels] - interleaved[i*numChannels+1]) / 2.0
		}
	case "dual-detect":
		identical := true
		for i := 0; i < numFrames && identical; i++ {
			l := interleaved[i*numChannels]
			r := interleaved[i*numChannels+1]
			if diff := l - r; diff < -1e-7 || diff > 1e-7 {
				identical = false
			}
		}
		if identical {
			for i := 0; i < numFrames; i++ {
				mono[i] = interleaved[i*numChannels]
			}
		} else {
			for i := 0; i < numFrames; i++ {
				mono[i] = (interleaved[i*numChannels] + interleaved[i*numChannels+1]) / 2.0
			}
		}
	default:
		return nil, fmt.Errorf("unknown mono mode: %s", mode)
	}
	return mono, nil
}

func ForceSampleRate(extraction *SliceExtraction, targetRate int) error {
	srcRate := extraction.Metadata.SampleRate
	if srcRate == targetRate {
		return nil
	}
	if srcRate <= 0 || targetRate <= 0 {
		return fmt.Errorf("invalid sample rate: src=%d, dst=%d", srcRate, targetRate)
	}

	ch := extraction.Metadata.Channels
	srcFrames := extraction.TotalFrames
	dstFrames := int(math.Ceil(float64(srcFrames) * float64(targetRate) / float64(srcRate)))
	ratio := float64(srcRate) / float64(targetRate)

	dst := make([]float32, dstFrames*ch)
	for i := 0; i < dstFrames; i++ {
		srcPos := float64(i) * ratio
		lo := int(srcPos)
		hi := lo + 1
		if hi >= srcFrames {
			hi = srcFrames - 1
		}
		frac := float32(srcPos - float64(lo))
		for c := 0; c < ch; c++ {
			s0 := extraction.Interleaved[lo*ch+c]
			s1 := extraction.Interleaved[hi*ch+c]
			dst[i*ch+c] = s0 + frac*(s1-s0)
		}
	}

	extraction.Interleaved = dst
	extraction.TotalFrames = dstFrames
	extraction.Metadata.SampleRate = targetRate
	return nil
}

func ConvertBitDepth(extraction *SliceExtraction, targetBitDepth int) {
	extraction.Metadata.BitDepth = targetBitDepth
}

func ForcePTISpec(extraction *SliceExtraction) error {
	if extraction.Metadata.Channels > 1 {
		mono, err := DownmixToMono(extraction.Interleaved, extraction.Metadata.Channels, "sum")
		if err != nil {
			return err
		}
		extraction.Interleaved = mono
		extraction.TotalFrames = len(mono)
		extraction.Metadata.Channels = 1
	}
	if err := ForceSampleRate(extraction, 44100); err != nil {
		return err
	}
	ConvertBitDepth(extraction, 16)
	return nil
}

func ForceOTSpec(extraction *SliceExtraction) error {
	if err := ForceSampleRate(extraction, 44100); err != nil {
		return err
	}
	return nil
}

func Force44100Spec(extraction *SliceExtraction) error {
	if err := ForceSampleRate(extraction, 44100); err != nil {
		return err
	}
	ConvertBitDepth(extraction, 16)
	return nil
}

func Force48kSpec(extraction *SliceExtraction) error {
	if err := ForceSampleRate(extraction, 48000); err != nil {
		return err
	}
	ConvertBitDepth(extraction, 16)
	return nil
}
