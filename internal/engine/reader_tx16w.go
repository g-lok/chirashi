package engine

import (
	"fmt"
)

type TX16WReader struct{}

func (r *TX16WReader) Probe(data []byte) (*RexMetadata, error) {
	if len(data) < 32 {
		return nil, fmt.Errorf("tx16w: too short")
	}
	if string(data[:6]) != "LM8953" {
		return nil, fmt.Errorf("tx16w: invalid header")
	}
	sr, _, _, _ := r.parseHeader(data)
	return &RexMetadata{SampleRate: sr, Channels: 1}, nil
}

func (r *TX16WReader) parseHeader(data []byte) (sampleRate, attackLen, repeatLen int, looped bool) {
	format := data[22]
	looped = (format == 0x49)
	srCode := data[23]
	
	atc0 := uint32(data[24])
	atc1 := uint32(data[25])
	atc2 := uint32(data[26])
	
	rpt0 := uint32(data[27])
	rpt1 := uint32(data[28])
	rpt2 := uint32(data[29])

	// bit level decoding of lengths
	// attack length (17 bits)
	attackLen = int(atc0 | (atc1 << 8) | ((atc2 & 0x01) << 16))
	// repeat length (17 bits)
	repeatLen = int(rpt0 | (rpt1 << 8) | ((rpt2 & 0x01) << 16))

	// sample rate
	switch srCode {
	case 1: sampleRate = 33333
	case 2: sampleRate = 50000
	case 3: sampleRate = 16667
	default:
		// Squeezed in bits if srCode is unknown
		srHash1 := atc2 & 0xFE
		srHash2 := rpt2 & 0xFE
		if srHash1 == 0x06 && srHash2 == 0x52 {
			sampleRate = 33333
		} else if srHash1 == 0x10 && srHash2 == 0x00 {
			sampleRate = 50000
		} else if srHash1 == 0xF6 && srHash2 == 0x52 {
			sampleRate = 16667
		} else {
			sampleRate = 33333 // default
		}
	}
	return
}

func (r *TX16WReader) SupportedExtensions() []string {
	return []string{".w01", ".w02", ".w03", ".w04", ".w05", ".w06", ".w07", ".w08", ".w09", ".w10",
		".w11", ".w12", ".w13", ".w14", ".w15", ".w16", ".w17", ".w18", ".w19", ".w20",
		".w21", ".w22", ".w23", ".w24", ".w25", ".w26", ".w27", ".w28", ".w29", ".w30",
		".w31", ".w32", ".txw"}
}

func (r *TX16WReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if _, err := r.Probe(data); err != nil {
		return nil, err
	}

	sampleRate, _, _, _ := r.parseHeader(data)
	payload := data[32:]
	// 12-bit packed: 2 samples per 3 bytes
	// Bits: [S1_0..7] [S1_8..11 S2_0..3] [S2_4..11]
	numPairs := len(payload) / 3
	pcm := make([]float32, numPairs*2)
	
	for i := 0; i < numPairs; i++ {
		b1 := uint32(payload[i*3])
		b2 := uint32(payload[i*3+1])
		b3 := uint32(payload[i*3+2])
		
		// Sample 1: b1 as LSB, low nibble of b2 as MSB
		s1 := int16(b1 | ((b2 & 0x0F) << 8))
		// Sign extend 12-bit to 16-bit
		if s1&0x0800 != 0 {
			s1 = int16(uint16(s1) | 0xF000)
		}

		// Sample 2: high nibble of b2 as LSB, b3 as MSB
		s2 := int16(((b2 & 0xF0) >> 4) | (b3 << 4))
		if s2&0x0800 != 0 {
			s2 = int16(uint16(s2) | 0xF000)
		}
		
		pcm[i*2] = float32(s1) / 2048.0
		pcm[i*2+1] = float32(s2) / 2048.0
	}

	return []SliceExtraction{
		{
			Metadata: RexMetadata{
				SampleRate: sampleRate,
				Channels:   1,
				BitDepth:   12,
			},
			Interleaved: pcm,
			TotalFrames: len(pcm),
			CuePoints: []WavCueMarker{
				{SliceID: 0, Position: 0, Label: "Sample"},
			},
		},
	}, nil
}

func init() {
	RegisterReader(&TX16WReader{})
}
