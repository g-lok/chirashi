package rexengine

import (
	"encoding/binary"
	"fmt"
	"io"
)

// EncodeWavContainer writes a PCM WAV with fmt, data, and optional cue chunks.
// Chunk order: RIFF > WAVE > fmt > data > cue. No LIST/INFO metadata.
// Writes sequentially in one pass — no post-hoc seeking.
func EncodeWavContainer(w io.WriteSeeker, extraction *SliceExtraction, targetBitDepth int) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode WAV: extraction data is empty")
	}

	bitDepth := 16
	if extraction.Metadata.BitDepth > 0 {
		bitDepth = extraction.Metadata.BitDepth
	}
	if targetBitDepth > 0 {
		if targetBitDepth < bitDepth {
			if targetBitDepth == 8 || targetBitDepth == 16 || targetBitDepth == 24 {
				bitDepth = targetBitDepth
			} else {
				return fmt.Errorf("unsupported hardware PCM bit depth requested: %d", targetBitDepth)
			}
		}
	}

	numChannels := extraction.Metadata.Channels
	sampleRate := extraction.Metadata.SampleRate
	bytesPerSample := bitDepth / 8
	bytesPerFrame := numChannels * bytesPerSample
	numFrames := len(extraction.Interleaved) / numChannels
	dataSize := numFrames * bytesPerFrame

	numCuePoints := len(extraction.CuePoints)
	cueChunkDataSize := 4 + numCuePoints*24

	totalSize := 12 + 24 + 8 + dataSize + 8 + cueChunkDataSize
	riffSize := uint32(totalSize - 8)

	w.Write([]byte("RIFF"))
	binary.Write(w, binary.LittleEndian, riffSize)
	w.Write([]byte("WAVE"))

	w.Write([]byte("fmt "))
	binary.Write(w, binary.LittleEndian, uint32(16))
	binary.Write(w, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(w, binary.LittleEndian, uint16(numChannels))
	binary.Write(w, binary.LittleEndian, uint32(sampleRate))
	binary.Write(w, binary.LittleEndian, uint32(sampleRate*bytesPerFrame))
	binary.Write(w, binary.LittleEndian, uint16(bytesPerFrame))
	binary.Write(w, binary.LittleEndian, uint16(bitDepth))

	w.Write([]byte("data"))
	binary.Write(w, binary.LittleEndian, uint32(dataSize))

	switch bitDepth {
	case 8:
		for _, fSample := range extraction.Interleaved {
			if fSample > 1.0 {
				fSample = 1.0
			} else if fSample < -1.0 {
				fSample = -1.0
			}
			binary.Write(w, binary.LittleEndian, uint8(int(fSample*127.0)+128.0))
		}
	case 24:
		for _, fSample := range extraction.Interleaved {
			if fSample > 1.0 {
				fSample = 1.0
			} else if fSample < -1.0 {
				fSample = -1.0
			}
			val := int32(fSample * 8388607.0)
			buf := []byte{byte(val), byte(val >> 8), byte(val >> 16)}
			w.Write(buf)
		}
	default:
		for _, fSample := range extraction.Interleaved {
			if fSample > 1.0 {
				fSample = 1.0
			} else if fSample < -1.0 {
				fSample = -1.0
			}
			binary.Write(w, binary.LittleEndian, int16(fSample*32767.0))
		}
	}

	if numCuePoints > 0 {
		w.Write([]byte("cue "))
		binary.Write(w, binary.LittleEndian, uint32(cueChunkDataSize))
		binary.Write(w, binary.LittleEndian, uint32(numCuePoints))
		for _, cp := range extraction.CuePoints {
			binary.Write(w, binary.LittleEndian, uint32(cp.SliceID+1)) // dwName: 1-based
			binary.Write(w, binary.LittleEndian, cp.Position)          // dwPosition: sample offset (M8: data-relative)
			w.Write([]byte("data"))                                     // fccChunk: always "data"
			binary.Write(w, binary.LittleEndian, uint32(0))            // dwChunkStart: 0 (M8 compatible)
			binary.Write(w, binary.LittleEndian, uint32(0))            // dwBlockStart: 0
			binary.Write(w, binary.LittleEndian, cp.Position)          // dwSampleOffset: frame index
		}
	}

	return nil
}
