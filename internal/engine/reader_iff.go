package engine

import (
	"encoding/binary"
	"fmt"
)

type IFFReader struct{}

func (r *IFFReader) Probe(data []byte) (*RexMetadata, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("iff: too short")
	}
	if string(data[:4]) != "FORM" {
		return nil, fmt.Errorf("iff: missing FORM")
	}
	subType := string(data[8:12])
	if subType != "8SVX" && subType != "16SV" {
		return nil, fmt.Errorf("iff: unsupported subtype %s", subType)
	}
	return &RexMetadata{Channels: 1}, nil
}

func (r *IFFReader) SupportedExtensions() []string {
	return []string{".8svx", ".16sv", ".iff"}
}

func (r *IFFReader) Read(data []byte, targetSampleRate int) ([]SliceExtraction, error) {
	if _, err := r.Probe(data); err != nil {
		return nil, err
	}

	subType := string(data[8:12])
	var sampleRate int
	var pcm []float32
	var bitDepth int

	pos := 12
	for pos+8 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		if pos+8+chunkSize > len(data) {
			break
		}
		chunkData := data[pos+8 : pos+8+chunkSize]

		switch chunkID {
		case "VHDR":
			if len(chunkData) >= 20 {
				sampleRate = int(binary.BigEndian.Uint16(chunkData[12:14]))
				if sampleRate == 0 {
					sampleRate = 8363
				}
			}
		case "BODY":
			if subType == "8SVX" {
				bitDepth = 8
				pcm = make([]float32, len(chunkData))
				for i, b := range chunkData {
					pcm[i] = float32(int8(b)) / 128.0
				}
			} else {
				bitDepth = 16
				pcm = make([]float32, len(chunkData)/2)
				for i := 0; i < len(chunkData)-1; i += 2 {
					val := int16(binary.BigEndian.Uint16(chunkData[i:]))
					pcm[i/2] = float32(val) / 32768.0
				}
			}
		}

		pos += 8 + chunkSize
		if chunkSize%2 == 1 {
			pos++
		}
	}

	if pcm == nil {
		return nil, fmt.Errorf("iff: missing BODY chunk")
	}

	return []SliceExtraction{
		{
			Metadata: RexMetadata{
				SampleRate: sampleRate,
				Channels:   1,
				BitDepth:   bitDepth,
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
	RegisterReader(&IFFReader{})
}
