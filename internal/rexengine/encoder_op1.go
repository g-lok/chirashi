package rexengine

import (
	"encoding/binary"
	"fmt"
	"io"
)

func EncodeOP1AIF(w io.Writer, extraction *SliceExtraction) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode OP-1 AIFF: extraction data is empty")
	}

	channels := extraction.Metadata.Channels
	sampleRate := extraction.Metadata.SampleRate
	bitDepth := 16
	if extraction.Metadata.BitDepth > 0 {
		bitDepth = extraction.Metadata.BitDepth
	}
	numChannels := uint16(channels)
	bitsPerSample := uint16(bitDepth)
	bytesPerSample := uint16(bitDepth / 8)
	blockAlign := numChannels * bytesPerSample

	totalSamples := len(extraction.Interleaved)
	numFrames := totalSamples / channels
	dataSize := uint32(numFrames * int(blockAlign))

	jsonData := buildOP1Metadata(extraction)
	applSize := uint32(0x1004)
	applPad := int(applSize) - len(jsonData) - 8

	commSize := uint32(18)
	ssndOffset := uint32(0)
	ssndBlockSize := uint32(0)

	formSize := uint32(12) + 8 + commSize + 8 + applSize + 8 + 8 + dataSize

	writeAIFHeader := func() error {
		if _, err := w.Write([]byte("FORM")); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, formSize); err != nil {
			return err
		}
		if _, err := w.Write([]byte("AIFF")); err != nil {
			return err
		}
		return nil
	}

	writeCOMM := func() error {
		if _, err := w.Write([]byte("COMM")); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, commSize); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, numChannels); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, uint32(numFrames)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, bitsPerSample); err != nil {
			return err
		}
		sampleRateBits := aiffSampleRate(sampleRate)
		if _, err := w.Write(sampleRateBits[:]); err != nil {
			return err
		}
		return nil
	}

	writeAPPL := func() error {
		if _, err := w.Write([]byte("APPL")); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, applSize); err != nil {
			return err
		}
		if _, err := w.Write([]byte("op-1")); err != nil {
			return err
		}
		if _, err := w.Write([]byte(jsonData)); err != nil {
			return err
		}
		pad := make([]byte, applPad)
		if _, err := w.Write(pad); err != nil {
			return err
		}
		return nil
	}

	writeSSND := func() error {
		if _, err := w.Write([]byte("SSND")); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, dataSize+8); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, ssndOffset); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, ssndBlockSize); err != nil {
			return err
		}
		for _, s := range extraction.Interleaved {
			clamped := s
			if clamped > 1.0 {
				clamped = 1.0
			} else if clamped < -1.0 {
				clamped = -1.0
			}
			if err := binary.Write(w, binary.BigEndian, int16(clamped*32767.0)); err != nil {
				return err
			}
		}
		return nil
	}

	if err := writeAIFHeader(); err != nil {
		return err
	}
	if err := writeCOMM(); err != nil {
		return err
	}
	if err := writeAPPL(); err != nil {
		return err
	}
	return writeSSND()
}

func aiffSampleRate(rate int) [10]byte {
	var result [10]byte

	mantissa := uint64(rate)
	exp := uint16(16383)
	for mantissa >= 1<<52 {
		mantissa >>= 1
		exp++
	}
	for mantissa < 1<<51 {
		mantissa <<= 1
		exp--
	}
	mantissa &= (1 << 52) - 1

	result[0] = byte(exp >> 8)
	result[1] = byte(exp & 0xFF)
	result[2] = byte(mantissa >> 44)
	result[3] = byte((mantissa >> 36) & 0xFF)
	result[4] = byte((mantissa >> 28) & 0xFF)
	result[5] = byte((mantissa >> 20) & 0xFF)
	result[6] = byte((mantissa >> 12) & 0xFF)
	result[7] = byte((mantissa >> 4) & 0xFF)
	result[8] = byte((mantissa & 0xF) << 4)
	result[9] = 0

	return result
}

func buildOP1Metadata(extraction *SliceExtraction) string {
	numSlices := len(extraction.CuePoints)
	if numSlices > 24 {
		numSlices = 24
	}

	scaleFactor := 2147483646.0 / (44100.0 * 20.0)
	if extraction.Metadata.Channels == 1 {
		scaleFactor = 2147483646.0 / (44100.0 * 12.0)
	}

	json := `{"name":"REXConverter","type":"drum","drum_version":2,"stereo":` +
		fmt.Sprintf(`%v`, extraction.Metadata.Channels == 2) +
		`,"octave":0,"original_folder":"rexconverter","mtime":1682173750,` +
		`"fx_active":false,"fx_type":"delay","fx_params":[8000,8000,8000,8000,8000,8000,8000,8000],` +
		`"lfo_active":false,"lfo_type":"tremolo","lfo_params":[16000,16000,16000,16000,16000,16000,16000,16000],` +
		`"dyna_env":[0,8192,0,8192,0,0,0,0],`

	json += `"volume":[`
	for i := 0; i < 24; i++ {
		if i > 0 {
			json += ","
		}
		json += "8192"
	}
	json += `],"pan":[`
	for i := 0; i < 24; i++ {
		if i > 0 {
			json += ","
		}
		json += "16384"
	}
	json += `],"pan_ab":[`
	for i := 0; i < 24; i++ {
		if i > 0 {
			json += ","
		}
		json += "false"
	}
	json += `],"pitch":[`
	for i := 0; i < 24; i++ {
		if i > 0 {
			json += ","
		}
		json += "0"
	}
	json += `],"playmode":[`
	for i := 0; i < 24; i++ {
		if i > 0 {
			json += ","
		}
		json += "12288"
	}
	json += `],"reverse":[`
	for i := 0; i < 24; i++ {
		if i > 0 {
			json += ","
		}
		json += "8192"
	}
	json += `],"start":[`
	for i := 0; i < 24; i++ {
		if i > 0 {
			json += ","
		}
		if i < numSlices {
			scaled := int64(float64(extraction.CuePoints[i].Position) * scaleFactor)
			json += fmt.Sprintf("%d", scaled)
		} else {
			json += "0"
		}
	}
	json += `],"end":[`
	for i := 0; i < 24; i++ {
		if i > 0 {
			json += ","
		}
		if i < numSlices {
			var endPos uint32
			if i+1 < len(extraction.CuePoints) {
				endPos = extraction.CuePoints[i+1].Position
			} else {
				endPos = uint32(extraction.TotalFrames)
			}
			scaled := int64(float64(endPos) * scaleFactor)
			json += fmt.Sprintf("%d", scaled)
		} else {
			json += "0"
		}
	}
	json += `],"attack":[`
	for i := 0; i < 24; i++ {
		if i > 0 {
			json += ","
		}
		json += "0"
	}
	json += `],"decay":[`
	for i := 0; i < 24; i++ {
		if i > 0 {
			json += ","
		}
		json += "0"
	}
	json += `]}`

	return json
}
