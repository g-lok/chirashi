package engine

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

func EncodePTI(w io.Writer, extraction *SliceExtraction) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode PTI: extraction data is empty")
	}

	numSamples := len(extraction.Interleaved)
	sampleLen := numSamples
	numSlices := len(extraction.CuePoints)
	if numSlices == 0 {
		numSlices = 1
	}

	playbackMode := uint8(5)
	if numSlices <= 1 {
		playbackMode = 0
	}

	var header [392]byte

	header[0] = 'T'
	header[1] = 'I'
	header[2] = 1
	header[3] = 0
	header[4] = 1
	header[5] = 2

	if numSlices > 1 {
		header[6] = 2
	} else {
		header[6] = 0
	}

	header[7] = 1
	binary.LittleEndian.PutUint32(header[8:12], 6)

	header[12] = 116
	header[13] = 1
	header[14] = 0
	header[15] = 0
	header[16] = 1
	header[17] = 0
	header[18] = 0
	header[19] = 0
	header[20] = 0

	name := []byte("REXConverter")
	copy(header[21:51], name)

	binary.LittleEndian.PutUint32(header[56:60], 0)
	binary.LittleEndian.PutUint32(header[60:64], uint32(sampleLen))
	binary.LittleEndian.PutUint16(header[64:66], 2048)
	header[66] = 0
	header[67] = 0
	binary.LittleEndian.PutUint16(header[68:70], 0)
	header[70] = 0
	header[71] = 0
	header[72] = 0
	header[73] = 0
	header[74] = 0
	header[75] = 0
	header[76] = playbackMode
	header[77] = 0
	binary.LittleEndian.PutUint16(header[78:80], 0)
	binary.LittleEndian.PutUint16(header[80:82], 1)
	binary.LittleEndian.PutUint16(header[82:84], 65534)
	binary.LittleEndian.PutUint16(header[84:86], 65535)
	header[86] = 0
	header[87] = 0
	binary.LittleEndian.PutUint16(header[88:90], 0)
	header[90] = 0
	header[91] = 0

	writePTIDefaultEnvelopes(header[:])

	sampleRateK := float64(extraction.Metadata.SampleRate) / 1000.0
	totalMs := float64(sampleLen) / sampleRateK
	for i := 0; i < 48; i++ {
		offset := 280 + i*2
		if i < numSlices && i < len(extraction.CuePoints) && totalMs > 0 {
			sliceStartMs := float64(extraction.CuePoints[i].Position) / sampleRateK
			ratio := sliceStartMs / totalMs
			if ratio > 1.0 {
				ratio = 1.0
			}
			val := uint16(ratio * 65535.0)
			binary.LittleEndian.PutUint16(header[offset:offset+2], val)
		} else {
			binary.LittleEndian.PutUint16(header[offset:offset+2], 65535)
		}
	}

	header[376] = uint8(numSlices)
	header[377] = 0
	binary.LittleEndian.PutUint16(header[378:380], 441)
	binary.LittleEndian.PutUint16(header[380:382], 0)
	header[382] = 0
	header[383] = 0
	header[384] = 0
	header[385] = 0
	header[386] = 16
	header[387] = 0

	if _, err := w.Write(header[:]); err != nil {
		return err
	}

	for _, s := range extraction.Interleaved {
		clamped := s
		if clamped > 1.0 {
			clamped = 1.0
		} else if clamped < -1.0 {
			clamped = -1.0
		}
		if err := binary.Write(w, binary.LittleEndian, int16(clamped*32767.0)); err != nil {
			return err
		}
	}

	return nil
}

func writePTIDefaultEnvelopes(header []byte) {
	type autoBlock struct {
		offset int
	}

	blocks := []autoBlock{
		{offset: 92},  // Volume
		{offset: 112}, // Panning
		{offset: 132}, // Cutoff
		{offset: 152}, // Wavetable pos
		{offset: 172}, // Granular pos
		{offset: 192}, // Finetune
	}

	for _, b := range blocks {
		binary.LittleEndian.PutUint32(header[b.offset:b.offset+4], math.Float32bits(1.0))
		header[b.offset+4] = 0
		header[b.offset+5] = 0
		binary.LittleEndian.PutUint16(header[b.offset+6:b.offset+8], 0)
		header[b.offset+8] = 0
		header[b.offset+9] = 0
		binary.LittleEndian.PutUint16(header[b.offset+10:b.offset+12], 0)
		binary.LittleEndian.PutUint32(header[b.offset+12:b.offset+16], math.Float32bits(1.0))
		binary.LittleEndian.PutUint16(header[b.offset+16:b.offset+18], 1000)
		binary.LittleEndian.PutUint16(header[b.offset+18:b.offset+20], 0)
	}

	for i := 0; i < 6; i++ {
		lfoOff := 212 + i*8
		header[lfoOff] = 2
		header[lfoOff+1] = 0
		header[lfoOff+2] = 0
		header[lfoOff+3] = 0
		binary.LittleEndian.PutUint32(header[lfoOff+4:lfoOff+8], math.Float32bits(0.5))
	}

	binary.LittleEndian.PutUint32(header[260:264], math.Float32bits(1.0))
	binary.LittleEndian.PutUint32(header[264:268], math.Float32bits(0.0))
	binary.LittleEndian.PutUint16(header[268:270], 0)

	header[270] = 0
	header[271] = 0
	header[272] = 50
	header[273] = 0
	header[274] = 0
	header[275] = 0
}
