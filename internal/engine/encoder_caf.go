package engine

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

const (
	cafFormatIDlpcm  = 0x6c70636d // 'lpcm'
	cafFlagSignedInt = 1 << 2
	cafFlagPacked    = 1 << 3
)

var (
	appleLoopMetaUUID = []byte{
		0x29, 0x81, 0x92, 0x73, 0xb5, 0xbf, 0x4a, 0xef,
		0xb7, 0x8d, 0x62, 0xd1, 0xef, 0x90, 0xbb, 0x2c,
	}
	appleLoopBeatMarkersUUID = []byte{
		0x03, 0x52, 0x81, 0x1b, 0x9d, 0x5d, 0x42, 0xe1,
		0x88, 0x2d, 0x6a, 0xf6, 0x1a, 0x6b, 0x33, 0x0c,
	}
)

func EncodeCAF(w io.Writer, extraction *SliceExtraction) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode CAF: extraction data is empty")
	}

	channels := extraction.Metadata.Channels
	sampleRate := extraction.Metadata.SampleRate
	bitDepth := 16
	if extraction.Metadata.BitDepth > 0 {
		bitDepth = extraction.Metadata.BitDepth
	}

	totalSamples := len(extraction.Interleaved)
	numFrames := totalSamples / channels
	bytesPerSample := bitDepth / 8
	blockAlign := channels * bytesPerSample

	if _, err := w.Write([]byte("caff")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint16(1)); err != nil { // version
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint16(0)); err != nil { // flags
		return err
	}

	descData := buildCAFDesc(sampleRate, channels, bitDepth, blockAlign)
	if _, err := w.Write([]byte("desc")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint64(len(descData))); err != nil {
		return err
	}
	if _, err := w.Write(descData); err != nil {
		return err
	}

	pcmSize := uint64(numFrames * blockAlign)
	if _, err := w.Write([]byte("data")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint64(4+pcmSize)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(0)); err != nil { // edit count
		return err
	}
	if err := writeCAFPCM(w, extraction.Interleaved, numFrames, channels, bitDepth); err != nil {
		return err
	}

	infoData := buildCAFInfoData()
	if _, err := w.Write([]byte("info")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint64(len(infoData))); err != nil {
		return err
	}
	if _, err := w.Write(infoData); err != nil {
		return err
	}

	metaData := buildAppleLoopMetaData(extraction)
	if _, err := w.Write([]byte("uuid")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint64(16+len(metaData))); err != nil {
		return err
	}
	if _, err := w.Write(appleLoopMetaUUID); err != nil {
		return err
	}
	if _, err := w.Write(metaData); err != nil {
		return err
	}

	beatData := buildBeatMarkerData(extraction)
	if _, err := w.Write([]byte("uuid")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint64(16+len(beatData))); err != nil {
		return err
	}
	if _, err := w.Write(appleLoopBeatMarkersUUID); err != nil {
		return err
	}
	if _, err := w.Write(beatData); err != nil {
		return err
	}

	return nil
}

func buildCAFDesc(sampleRate, channels, bitDepth, blockAlign int) []byte {
	var d []byte
	d = appendUint64(d, math.Float64bits(float64(sampleRate))) // mSampleRate
	d = appendUint32(d, cafFormatIDlpcm)              // mFormatID
	d = appendUint32(d, cafFlagSignedInt|cafFlagPacked) // mFormatFlags
	d = appendUint32(d, uint32(blockAlign))           // mBytesPerPacket
	d = appendUint32(d, 1)                            // mFramesPerPacket
	d = appendUint32(d, uint32(channels))             // mChannelsPerFrame
	d = appendUint32(d, uint32(bitDepth))             // mBitsPerChannel
	return d
}

func buildCAFInfoData() []byte {
	var b []byte
	b = appendUint32(b, 1)
	b = append(b, "genre"...)
	b = append(b, 0)
	b = append(b, "Other Genre"...)
	b = append(b, 0)
	return b
}

func buildAppleLoopMetaData(extraction *SliceExtraction) []byte {
	meta := extraction.Metadata
	beatCount := meta.PPQLength / 15360
	if beatCount <= 0 {
		if meta.Tempo > 0 && meta.SampleRate > 0 {
			durationSec := float64(extraction.TotalFrames) / float64(meta.SampleRate)
			beatCount = int(meta.Tempo / 60.0 * durationSec)
		}
		if beatCount <= 0 {
			beatCount = 4
		}
	}

	timeSig := fmt.Sprintf("%d/%d", meta.TimeSignNom, meta.TimeSignDenom)
	if meta.TimeSignNom == 0 || meta.TimeSignDenom == 0 {
		timeSig = "4/4"
	}

	kv := []struct{ k, v string }{
		{"category", "Mixed"},
		{"subcategory", "Loop"},
		{"genre", "Other Genre"},
		{"beat count", fmt.Sprintf("%d", beatCount)},
		{"time signature", timeSig},
		{"descriptors", "Loop,Grooving"},
	}

	var b []byte
	b = appendUint32(b, uint32(len(kv)))
	for _, e := range kv {
		b = append(b, e.k...)
		b = append(b, 0)
		b = append(b, e.v...)
		b = append(b, 0)
	}
	return b
}

func buildBeatMarkerData(extraction *SliceExtraction) []byte {
	seen := make(map[uint32]bool)
	var positions []uint32
	for _, cp := range extraction.CuePoints {
		if !seen[cp.Position] {
			positions = append(positions, cp.Position)
			seen[cp.Position] = true
		}
	}
	endPos := uint32(extraction.TotalFrames)
	if !seen[endPos] {
		positions = append(positions, endPos)
	}

	var b []byte
	b = appendUint32(b, 0)                           // unknown
	b = appendUint32(b, 0x00010000)                  // flags
	b = appendUint16(b, 0x0032)                      // version?
	b = appendUint16(b, 0x0010)                      // unknown
	b = appendUint32(b, 0)                           // unknown
	b = appendUint32(b, uint32(len(positions)))       // marker count

	for _, pos := range positions {
		b = appendUint16(b, 0x0001) // flags
		b = appendUint16(b, 0)      // padding
		b = appendUint32(b, 0)      // padding
		b = appendUint32(b, pos)    // sample position
	}
	return b
}

func writeCAFPCM(w io.Writer, interleaved []float32, numFrames, channels, bitDepth int) error {
	bytesPerSample := bitDepth / 8
	blockAlign := channels * bytesPerSample
	buf := make([]byte, 1024*blockAlign)
	written := 0
	needed := numFrames * blockAlign

	for written < needed {
		remaining := needed - written
		chunk := remaining
		if chunk > len(buf) {
			chunk = len(buf)
		}
		frameStart := written / blockAlign
		for i := 0; i*bytesPerSample < chunk; i++ {
			sampleIdx := frameStart*channels + i
			val := int16(interleaved[sampleIdx] * 32768)
			if val > 32767 {
				val = 32767
			} else if val < -32768 {
				val = -32768
			}
			pos := i * 2
			buf[pos] = byte(val >> 8)
			buf[pos+1] = byte(val)
		}
		written += chunk
		if _, err := w.Write(buf[:chunk]); err != nil {
			return err
		}
	}
	return nil
}


func appendUint32(b []byte, v uint32) []byte {
	return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func appendUint64(b []byte, v uint64) []byte {
	return append(b, byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32), byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func appendUint16(b []byte, v uint16) []byte {
	return append(b, byte(v>>8), byte(v))
}
