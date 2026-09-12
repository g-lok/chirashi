package engine

import (
	"encoding/binary"
	"io"
)

func Encode8SVX(w io.WriteSeeker, extraction *SliceExtraction) error {
	return encodeIFFSVX(w, extraction, 8)
}

func Encode16SV(w io.WriteSeeker, extraction *SliceExtraction) error {
	return encodeIFFSVX(w, extraction, 16)
}

func encodeIFFSVX(w io.WriteSeeker, extraction *SliceExtraction, bitDepth int) error {
	pcm := extraction.Interleaved
	// For IFF SVX, we only support mono.
	if extraction.Metadata.Channels > 1 {
		// Just take first channel
		mono := make([]float32, len(pcm)/extraction.Metadata.Channels)
		for i := 0; i < len(mono); i++ {
			mono[i] = pcm[i*extraction.Metadata.Channels]
		}
		pcm = mono
	}

	subType := "8SVX"
	if bitDepth == 16 {
		subType = "16SV"
	}

	bodySize := len(pcm)
	if bitDepth == 16 {
		bodySize *= 2
	}

	// FORM(4) + len(4) + SubType(4) + VHDR(4) + vhdrLen(4) + vhdrData(20) + BODY(4) + bodyLen(4) + bodyData(N)
	totalSize := 4 + 4 + 4 + 4 + 4 + 20 + 4 + 4 + bodySize
	if bodySize%2 == 1 {
		totalSize++
	}

	w.Write([]byte("FORM"))
	binary.Write(w, binary.BigEndian, uint32(totalSize-8))
	w.Write([]byte(subType))

	// VHDR chunk
	w.Write([]byte("VHDR"))
	binary.Write(w, binary.BigEndian, uint32(20))
	binary.Write(w, binary.BigEndian, uint32(len(pcm))) // oneShotHiSamples
	binary.Write(w, binary.BigEndian, uint32(0))      // repeatHiSamples
	binary.Write(w, binary.BigEndian, uint32(0))      // samplesPerHiCycle
	binary.Write(w, binary.BigEndian, uint16(extraction.Metadata.SampleRate))
	w.Write([]byte{1, 0}) // ctOctave=1, sCompression=0
	binary.Write(w, binary.BigEndian, uint32(65536)) // volume (unity)

	// BODY chunk
	w.Write([]byte("BODY"))
	binary.Write(w, binary.BigEndian, uint32(bodySize))
	if bitDepth == 8 {
		for _, s := range pcm {
			if s > 1.0 { s = 1.0 }
			if s < -1.0 { s = -1.0 }
			w.Write([]byte{byte(int8(s * 127.0))})
		}
	} else {
		for _, s := range pcm {
			if s > 1.0 { s = 1.0 }
			if s < -1.0 { s = -1.0 }
			binary.Write(w, binary.BigEndian, int16(s*32767.0))
		}
	}

	if bodySize%2 == 1 {
		w.Write([]byte{0})
	}

	return nil
}
