package engine

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

type OTSlice struct {
	Start uint32
	End   uint32
	Loop  int32
}

func EncodeOT(w io.Writer, extraction *SliceExtraction, bpm float64) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode OT: extraction data is empty")
	}

	totalSamples := len(extraction.Interleaved)
	channels := extraction.Metadata.Channels
	sampleFrames := totalSamples / channels

	if bpm <= 0 {
		bpm = 120.0
	}

	bpmInt := uint32(bpm * 24.0)
	barsVal := (float64(sampleFrames) / float64(extraction.Metadata.SampleRate)) / ((60.0 / bpm) * 4.0)
	trimLen := uint32(math.Round(barsVal * 100.0))
	loopLen := trimLen

	var buf [0x340]byte

	copy(buf[0:4], []byte("FORM"))
	binary.BigEndian.PutUint32(buf[4:8], uint32(len(buf)-8))
	copy(buf[8:12], []byte("DPS1"))
	copy(buf[12:16], []byte("SMPA"))
	binary.BigEndian.PutUint32(buf[16:20], uint32(0x330-8))
	buf[20] = 0x02
	buf[21] = 0x00
	buf[22] = 0x00

	binary.BigEndian.PutUint32(buf[23:27], bpmInt)
	binary.BigEndian.PutUint32(buf[27:31], trimLen)
	binary.BigEndian.PutUint32(buf[31:35], loopLen)

	buf[35] = 0
	buf[36] = 0
	buf[37] = 0

	buf[38] = 0
	buf[39] = 0

	binary.BigEndian.PutUint16(buf[40:42], 0x30)
	buf[42] = 0xFF

	binary.BigEndian.PutUint32(buf[43:47], 0)
	binary.BigEndian.PutUint32(buf[47:51], uint32(sampleFrames))
	binary.BigEndian.PutUint32(buf[51:55], 0)

	numSlices := len(extraction.CuePoints)
	if numSlices > 64 {
		numSlices = 64
	}

	for i := 0; i < 64; i++ {
		slot := 58 + i*12
		if i < numSlices {
			startPos := extraction.CuePoints[i].Position
			var endPos uint32
			if i+1 < len(extraction.CuePoints) {
				endPos = extraction.CuePoints[i+1].Position
			} else {
				endPos = uint32(sampleFrames)
			}
			binary.BigEndian.PutUint32(buf[slot:slot+4], startPos)
			binary.BigEndian.PutUint32(buf[slot+4:slot+8], endPos)
			binary.BigEndian.PutUint32(buf[slot+8:slot+12], 0xFFFFFFFF)
		} else {
			binary.BigEndian.PutUint32(buf[slot:slot+4], 0)
			binary.BigEndian.PutUint32(buf[slot+4:slot+8], 0)
			binary.BigEndian.PutUint32(buf[slot+8:slot+12], 0xFFFFFFFF)
		}
	}

	binary.BigEndian.PutUint32(buf[0x33A:0x33E], uint32(numSlices))

	var checksum uint16
	for i := 0x10; i < 0x33E; i++ {
		checksum += uint16(buf[i])
	}
	binary.BigEndian.PutUint16(buf[0x33E:0x340], checksum)

	_, err := w.Write(buf[:])
	return err
}
