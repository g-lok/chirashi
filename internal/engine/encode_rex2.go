package engine

import "github.com/g-lok/chirashi/internal/engine/rex2"

// EncodeREX2 writes a REX2 file from a SliceExtraction.
// extraction.Metadata provides file-level info, Interleaved holds PCM,
// and CuePoints define slice boundaries.
// tempoOverride is BPM from CLI flag (0 = use metadata tempo).
func EncodeREX2(extraction *SliceExtraction, tempoOverride int) ([]byte, error) {
	bitDepth := extraction.Metadata.BitDepth
	if bitDepth <= 0 {
		bitDepth = 16
	}

	channels := extraction.Metadata.Channels
	if channels <= 0 {
		channels = 1
	}

	sampleRate := extraction.Metadata.SampleRate
	if sampleRate <= 0 {
		sampleRate = 44100
	}

	// Convert float32 PCM to int32
	pcm := make([]int32, len(extraction.Interleaved))
	for i, s := range extraction.Interleaved {
		pcm[i] = rex2.Float32ToPCM(s, bitDepth)
	}

	totalFrames := extraction.TotalFrames
	if totalFrames <= 0 && len(pcm) > 0 {
		totalFrames = len(pcm) / channels
	}

	// Build slice entries from cue points
	var slices []rex2.SliceInfo
	if len(extraction.CuePoints) > 0 {
		slices = make([]rex2.SliceInfo, len(extraction.CuePoints))
		for i, cp := range extraction.CuePoints {
			start := int(cp.Position)
			var length int
			if i+1 < len(extraction.CuePoints) {
				nextStart := int(extraction.CuePoints[i+1].Position)
				if nextStart > start {
					length = nextStart - start
				} else {
					length = 1
				}
			} else {
				length = totalFrames - start
			}
			if length < 1 {
				length = 1
			}
			if start+length > totalFrames {
				length = totalFrames - start
			}
			if length < 1 {
				length = 1
			}
			slices[i] = rex2.SliceInfo{
				PPQPos:       0,
				SampleStart:  start,
				SampleLength: length,
			}
		}
	} else {
		slices = []rex2.SliceInfo{{
			PPQPos:       0,
			SampleStart:  0,
			SampleLength: totalFrames,
		}}
	}

	// Resolve tempo: metadata → CLI override → default
	// REX spec requires 20-450 BPM (stored as BPM*1000 internally)
	const minTempo = 20000  // 20 BPM
	const maxTempo = 450000 // 450 BPM
	const defaultTempo = 120000 // 120 BPM

	tempo := int(extraction.Metadata.Tempo * 1000)
	if tempo <= 0 {
		if tempoOverride > 0 {
			tempo = tempoOverride * 1000
		} else {
			tempo = defaultTempo
		}
	}
	// Clamp to valid REX range
	if tempo < minTempo {
		tempo = minTempo
	}
	if tempo > maxTempo {
		tempo = maxTempo
	}

	origTempo := int(extraction.Metadata.OriginalTempo * 1000)
	if origTempo <= 0 {
		origTempo = tempo
	}

	info := rex2.FileInfo{
		Channels:      channels,
		SampleRate:    sampleRate,
		SliceCount:    len(slices),
		Tempo:         tempo,
		OriginalTempo: origTempo,
		PPQLength:     extraction.Metadata.PPQLength,
		TimeSigNum:    extraction.Metadata.TimeSignNom,
		TimeSigDen:    extraction.Metadata.TimeSignDenom,
		BitDepth:      bitDepth,
		TotalFrames:   totalFrames,
	}

	// Ensure valid defaults
	if info.PPQLength <= 0 {
		info.PPQLength = 61440
	}
	if info.TimeSigNum <= 0 {
		info.TimeSigNum = 4
	}
	if info.TimeSigDen <= 0 {
		info.TimeSigDen = 4
	}

	return rex2.Encode(pcm, info, slices, rex2.CreatorInfo{})
}


