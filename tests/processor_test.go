package rexconverter_test

import (
	"math"
	"testing"
)

// Replicate the types needed for processor testing without CGo dependency.
type rexMetadata struct {
	Channels      int
	SampleRate    int
	Tempo         float64
	OriginalTempo float64
	TimeSignNom   int
	TimeSignDenom int
	BitDepth      int
}

type rexSlicePayload struct {
	SliceIndex  int
	PPQPos      int
	FrameLength int
	PCMData     []float32
}

type rawExtraction struct {
	Metadata rexMetadata
	Slices   []rexSlicePayload
}

type wavCueMarker struct {
	SliceID  int
	Position uint32
	Label    string
}

type sliceExtraction struct {
	Metadata    rexMetadata
	CuePoints   []wavCueMarker
	Interleaved []float32
	TotalFrames int
}

// Pure Go port of ProcessSlices and stitchGroup for testing.
func processSlices(raw *rawExtraction, maxSlices int, normalize bool) ([]sliceExtraction, error) {
	if raw == nil || len(raw.Slices) == 0 {
		return nil, nil
	}
	totalSlices := len(raw.Slices)
	if maxSlices <= 0 {
		return []sliceExtraction{stitchGroup(raw, raw.Slices)}, nil
	}
	var partitionSizes []int
	if normalize && maxSlices > 1 && totalSlices > maxSlices {
		numFiles := int(math.Ceil(float64(totalSlices) / float64(maxSlices)))
		baseSize := totalSlices / numFiles
		remainder := totalSlices % numFiles
		for i := 0; i < numFiles; i++ {
			size := baseSize
			if i < remainder {
				size++
			}
			partitionSizes = append(partitionSizes, size)
		}
	} else {
		remaining := totalSlices
		for remaining > 0 {
			take := maxSlices
			if remaining < maxSlices {
				take = remaining
			}
			partitionSizes = append(partitionSizes, take)
			remaining -= take
		}
	}
	var results []sliceExtraction
	currentIndex := 0
	for _, size := range partitionSizes {
		sliceGroup := raw.Slices[currentIndex : currentIndex+size]
		results = append(results, stitchGroup(raw, sliceGroup))
		currentIndex += size
	}
	return results, nil
}

func stitchGroup(raw *rawExtraction, sliceGroup []rexSlicePayload) sliceExtraction {
	var interleavedAudio []float32
	var cuePoints []wavCueMarker
	var currentFrameOffset uint32 = 0
	for subIndex, slice := range sliceGroup {
		cuePoints = append(cuePoints, wavCueMarker{
			SliceID:  subIndex,
			Position: currentFrameOffset,
			Label:    "",
		})
		interleavedAudio = append(interleavedAudio, slice.PCMData...)
		currentFrameOffset += uint32(slice.FrameLength)
	}
	return sliceExtraction{
		Metadata:    raw.Metadata,
		CuePoints:   cuePoints,
		Interleaved: interleavedAudio,
		TotalFrames: int(currentFrameOffset),
	}
}

// ---------- Tests ----------

func makeTestSlice(index, ppqPos, frameLen int) rexSlicePayload {
	pcm := make([]float32, frameLen)
	for i := range pcm {
		pcm[i] = float32(index*1000+i) * 0.001
	}
	return rexSlicePayload{
		SliceIndex:  index,
		PPQPos:      ppqPos,
		FrameLength: frameLen,
		PCMData:     pcm,
	}
}

func TestProcessSlices_NoLimit(t *testing.T) {
	slices := []rexSlicePayload{
		makeTestSlice(0, 0, 100),
		makeTestSlice(1, 100, 200),
		makeTestSlice(2, 300, 150),
	}
	raw := &rawExtraction{
		Metadata: rexMetadata{Channels: 1, SampleRate: 44100, BitDepth: 16},
		Slices:   slices,
	}
	results, err := processSlices(raw, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].TotalFrames != 100+200+150 {
		t.Fatalf("expected %d frames, got %d", 100+200+150, results[0].TotalFrames)
	}
	if len(results[0].Interleaved) != 100+200+150 {
		t.Fatalf("expected %d PCM values, got %d", 100+200+150, len(results[0].Interleaved))
	}
}

func TestProcessSlices_HardLimit(t *testing.T) {
	slices := make([]rexSlicePayload, 10)
	for i := range slices {
		slices[i] = makeTestSlice(i, i*100, 100)
	}
	raw := &rawExtraction{Slices: slices}

	results, _ := processSlices(raw, 3, false)
	if len(results) != 4 {
		t.Fatalf("10/3 hard limit: expected 4 groups, got %d", len(results))
	}
	// 3, 3, 3, 1
	sizes := []int{3, 3, 3, 1}
	for i, r := range results {
		if len(r.CuePoints) != sizes[i] {
			t.Fatalf("group %d: expected %d slices, got %d", i, sizes[i], len(r.CuePoints))
		}
	}
}

func TestProcessSlices_Normalize(t *testing.T) {
	slices := make([]rexSlicePayload, 10)
	for i := range slices {
		slices[i] = makeTestSlice(i, i*100, 100)
	}
	raw := &rawExtraction{Slices: slices}

	results, _ := processSlices(raw, 4, true)
	// 10 slices, limit 4: numFiles = ceil(10/4) = 3, baseSize = 3, remainder = 1
	// sizes: 4, 3, 3
	if len(results) != 3 {
		t.Fatalf("10/4 normalized: expected 3 groups, got %d", len(results))
	}
	sizes := []int{4, 3, 3}
	for i, r := range results {
		if len(r.CuePoints) != sizes[i] {
			t.Fatalf("group %d: expected %d slices, got %d", i, sizes[i], len(r.CuePoints))
		}
	}
}

func TestProcessSlices_ExactLimit(t *testing.T) {
	slices := make([]rexSlicePayload, 8)
	for i := range slices {
		slices[i] = makeTestSlice(i, i*100, 100)
	}
	raw := &rawExtraction{Slices: slices}

	results, _ := processSlices(raw, 8, false)
	if len(results) != 1 {
		t.Fatalf("8/8: expected 1 group, got %d", len(results))
	}
	if len(results[0].CuePoints) != 8 {
		t.Fatalf("expected 8 slices, got %d", len(results[0].CuePoints))
	}
}

func TestProcessSlices_SingleSlice(t *testing.T) {
	slices := []rexSlicePayload{makeTestSlice(0, 0, 100)}
	raw := &rawExtraction{Slices: slices}

	results, _ := processSlices(raw, 1, false)
	if len(results) != 1 {
		t.Fatalf("single slice: expected 1 group, got %d", len(results))
	}
}

func TestStitchGroup_CuePositions(t *testing.T) {
	slices := []rexSlicePayload{
		makeTestSlice(0, 0, 100),
		makeTestSlice(1, 100, 200),
		makeTestSlice(2, 300, 150),
	}
	raw := &rawExtraction{
		Metadata: rexMetadata{Channels: 1, SampleRate: 44100},
		Slices:   slices,
	}
	result := stitchGroup(raw, slices)

	if len(result.CuePoints) != 3 {
		t.Fatalf("expected 3 cue points, got %d", len(result.CuePoints))
	}
	expectedOffsets := []uint32{0, 100, 300}
	for i, cp := range result.CuePoints {
		if cp.Position != expectedOffsets[i] {
			t.Fatalf("cue %d: expected position %d, got %d", i, expectedOffsets[i], cp.Position)
		}
	}
	if result.TotalFrames != 100+200+150 {
		t.Fatalf("expected %d total frames, got %d", 100+200+150, result.TotalFrames)
	}
}

func TestProcessSlices_Empty(t *testing.T) {
	raw := &rawExtraction{Slices: []rexSlicePayload{}}
	results, err := processSlices(raw, 8, false)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Fatal("expected nil results for empty slices")
	}
}
