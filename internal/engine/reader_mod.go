package engine

import (
	"encoding/binary"
	"fmt"
	"strings"
)

type MODReader struct{}

type modSampleHeader struct {
	Name       [22]byte
	Length     uint16 // in words (2 bytes)
	FineTune   uint8  // lower 4 bits
	Volume     uint8
	LoopStart  uint16 // in words
	LoopLength uint16 // in words
}

func (r *MODReader) Probe(data []byte) (*RexMetadata, error) {
	if len(data) < 1084 {
		return nil, fmt.Errorf("mod: too short")
	}
	magic := string(data[1080:1084])
	// Common MOD magics
	validMagics := map[string]bool{
		"M.K.": true, "M!K!": true, "4CHN": true, "6CHN": true, "8CHN": true,
		"FLT4": true, "FLT8": true, "2CHN": true, "10CH": true, "12CH": true,
		"14CH": true, "16CH": true, "18CH": true, "20CH": true, "22CH": true,
		"24CH": true, "26CH": true, "28CH": true, "30CH": true, "32CH": true,
	}
	if !validMagics[magic] {
		return nil, fmt.Errorf("mod: invalid magic %s", magic)
	}
	return &RexMetadata{SampleRate: 8363, Channels: 1}, nil
}

func (r *MODReader) SupportedExtensions() []string {
	return []string{".mod"}
}

func (r *MODReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if _, err := r.Probe(data); err != nil {
		return nil, err
	}

	// ProTracker MOD has 31 samples
	samples := make([]SliceExtraction, 0, 31)
	offset := 20 // skip song name

	headers := make([]modSampleHeader, 31)
	for i := 0; i < 31; i++ {
		h := modSampleHeader{}
		copy(h.Name[:], data[offset:offset+22])
		h.Length = binary.BigEndian.Uint16(data[offset+22 : offset+24])
		h.FineTune = data[offset+24] & 0x0F
		h.Volume = data[offset+25]
		h.LoopStart = binary.BigEndian.Uint16(data[offset+26 : offset+28])
		h.LoopLength = binary.BigEndian.Uint16(data[offset+28 : offset+30])
		headers[i] = h
		offset += 30
	}

	// Pattern data starts after song length (1B), restart pos (1B), and order list (128B) + magic (4B)
	// Offset should now be 20 + 31*30 = 950
	// numOrders := int(data[950])
	// Find max pattern index to calculate offset to sample data
	maxPat := -1
	for i := 0; i < 128; i++ {
		p := int(data[952+i])
		if p > maxPat {
			maxPat = p
		}
	}
	if maxPat < 0 {
		maxPat = 0
	}
	// Pattern data starts at 1084. Each pattern is 1024 bytes (4 channels * 64 rows * 4 bytes)
	// Actually depends on channel count from magic
	magic := string(data[1080:1084])
	numChans := 4
	switch magic {
	case "6CHN": numChans = 6
	case "8CHN", "FLT8": numChans = 8
	case "2CHN": numChans = 2
	// ... add more if needed
	}
	
	sampleDataOffset := 1084 + (maxPat+1)*numChans*64*4

	currentSampleOffset := sampleDataOffset
	for i, h := range headers {
		lengthBytes := int(h.Length) * 2
		if lengthBytes == 0 {
			continue
		}
		if currentSampleOffset+lengthBytes > len(data) {
			break
		}

		pcmRaw := data[currentSampleOffset : currentSampleOffset+lengthBytes]
		pcm := make([]float32, len(pcmRaw))
		for j, b := range pcmRaw {
			// MOD samples are signed 8-bit
			pcm[j] = float32(int8(b)) / 128.0
		}

		name := strings.TrimRight(string(h.Name[:]), "\x00 ")
		if name == "" {
			name = fmt.Sprintf("Sample %02d", i+1)
		}

		samples = append(samples, SliceExtraction{
			Metadata: RexMetadata{
				SampleRate: 8363, // Amiga C-3 default
				Channels:   1,
				BitDepth:   8,
			},
			Interleaved: pcm,
			TotalFrames: len(pcm),
			CuePoints: []WavCueMarker{
				{SliceID: 0, Position: 0, Label: name},
			},
		})
		currentSampleOffset += lengthBytes
	}

	return samples, nil
}

func init() {
	RegisterReader(&MODReader{})
}
