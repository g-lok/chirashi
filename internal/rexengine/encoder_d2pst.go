package rexengine

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"strings"
	"time"
)

const (
	zipMethodStore   uint16 = 0
	zipMethodDeflate uint16 = 8
)

type writeSeekBuffer struct {
	*bytes.Buffer
}

func (w *writeSeekBuffer) Seek(_ int64, _ int) (int64, error) {
	return 0, nil
}

func EncodeDT2Preset(w io.Writer, extraction *SliceExtraction, name string) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode DT2 preset: extraction data is empty")
	}

	payloadName := sanitizeDT2Name(name)
	if payloadName == "" {
		payloadName = "OUTPUT"
	}

	transferDir := fmt.Sprintf("Samples/transfers-%s", time.Now().Format("060102"))
	sampleName := fmt.Sprintf("%s.wav", payloadName)

	var wavBuf bytes.Buffer
	if err := EncodeWavContainer(&writeSeekBuffer{Buffer: &wavBuf}, extraction, 16); err != nil {
		return err
	}
	wavData := wavBuf.Bytes()

	wavPCM := pcmChunkData(wavData)
	hash := crc32.ChecksumIEEE(wavPCM)

	presetBin := buildDT2PresetBinary(extraction, payloadName, hash)

	manifestData := buildDT2Manifest(payloadName, transferDir+"/"+sampleName, len(wavData), hash)

	entries := []zipEntry{
		{name: "manifest.json", data: []byte(manifestData), method: zipMethodDeflate},
		{name: transferDir + "/" + sampleName, data: wavData, method: zipMethodDeflate},
		{name: payloadName, data: presetBin, method: zipMethodDeflate},
	}

	return writeZIP(w, entries)
}

func pcmChunkData(wav []byte) []byte {
	if len(wav) < 12 {
		return nil
	}
	pos := 12
	for pos+8 <= len(wav) {
		cid := string(wav[pos : pos+4])
		csz := int(binary.LittleEndian.Uint32(wav[pos+4 : pos+8]))
		if cid == "data" {
			end := pos + 8 + csz
			if end > len(wav) {
				end = len(wav)
			}
			return wav[pos+8 : end]
		}
		pos += 8 + csz
	}
	return nil
}

type zipEntry struct {
	name   string
	data   []byte
	method uint16
}

func dosTime(t time.Time) (uint16, uint16) {
	timeVal := uint16(t.Hour())<<11 | uint16(t.Minute())<<5 | uint16(t.Second()/2)
	dateVal := uint16(t.Year()-1980)<<9 | uint16(t.Month())<<5 | uint16(t.Day())
	return timeVal, dateVal
}

func writeZIP(w io.Writer, entries []zipEntry) error {
	localOffsets := make([]uint32, len(entries))
	var centralBuf bytes.Buffer
	var totalSize uint32

	const sigLocal uint32 = 0x04034b50
	const sigCentral uint32 = 0x02014b50
	const sigEOCD uint32 = 0x06054b50

	modTime, modDate := dosTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	for i, e := range entries {
		localOffsets[i] = totalSize

		uncompSize := uint32(len(e.data))
		crc := crc32.ChecksumIEEE(e.data)

		var compressedData []byte
		method := e.method
		if method == zipMethodDeflate {
			var compBuf bytes.Buffer
			fw, err := flate.NewWriter(&compBuf, flate.DefaultCompression)
			if err != nil {
				return err
			}
			if _, err := fw.Write(e.data); err != nil {
				return err
			}
			if err := fw.Close(); err != nil {
				return err
			}
			compressedData = compBuf.Bytes()
		} else {
			compressedData = e.data
			method = zipMethodStore
		}
		compSize := uint32(len(compressedData))

		nameBytes := []byte(e.name)
		nameLen := uint16(len(nameBytes))

		var hdr bytes.Buffer
		binary.Write(&hdr, binary.LittleEndian, sigLocal)
		binary.Write(&hdr, binary.LittleEndian, uint16(20))
		binary.Write(&hdr, binary.LittleEndian, uint16(0))
		binary.Write(&hdr, binary.LittleEndian, method)
		binary.Write(&hdr, binary.LittleEndian, modTime)
		binary.Write(&hdr, binary.LittleEndian, modDate)
		binary.Write(&hdr, binary.LittleEndian, crc)
		binary.Write(&hdr, binary.LittleEndian, compSize)
		binary.Write(&hdr, binary.LittleEndian, uncompSize)
		binary.Write(&hdr, binary.LittleEndian, nameLen)
		binary.Write(&hdr, binary.LittleEndian, uint16(0))
		hdr.Write(nameBytes)

		if _, err := w.Write(hdr.Bytes()); err != nil {
			return err
		}
		if _, err := w.Write(compressedData); err != nil {
			return err
		}

		totalSize += uint32(hdr.Len()) + compSize

		var ce bytes.Buffer
		binary.Write(&ce, binary.LittleEndian, sigCentral)
		binary.Write(&ce, binary.LittleEndian, uint16(20))
		binary.Write(&ce, binary.LittleEndian, uint16(20))
		binary.Write(&ce, binary.LittleEndian, uint16(0))
		binary.Write(&ce, binary.LittleEndian, method)
		binary.Write(&ce, binary.LittleEndian, modTime)
		binary.Write(&ce, binary.LittleEndian, modDate)
		binary.Write(&ce, binary.LittleEndian, crc)
		binary.Write(&ce, binary.LittleEndian, compSize)
		binary.Write(&ce, binary.LittleEndian, uncompSize)
		binary.Write(&ce, binary.LittleEndian, nameLen)
		binary.Write(&ce, binary.LittleEndian, uint16(0))
		binary.Write(&ce, binary.LittleEndian, uint16(0))
		binary.Write(&ce, binary.LittleEndian, uint16(0))
		binary.Write(&ce, binary.LittleEndian, uint16(0))
		binary.Write(&ce, binary.LittleEndian, uint32(0))
		binary.Write(&ce, binary.LittleEndian, localOffsets[i])
		ce.Write(nameBytes)

		if _, err := centralBuf.Write(ce.Bytes()); err != nil {
			return err
		}
	}

	var eocd bytes.Buffer
	centralSize := uint32(centralBuf.Len())
	binary.Write(&eocd, binary.LittleEndian, sigEOCD)
	binary.Write(&eocd, binary.LittleEndian, uint16(0))
	binary.Write(&eocd, binary.LittleEndian, uint16(0))
	binary.Write(&eocd, binary.LittleEndian, uint16(len(entries)))
	binary.Write(&eocd, binary.LittleEndian, uint16(len(entries)))
	binary.Write(&eocd, binary.LittleEndian, centralSize)
	binary.Write(&eocd, binary.LittleEndian, totalSize)
	binary.Write(&eocd, binary.LittleEndian, uint16(0))

	if _, err := w.Write(centralBuf.Bytes()); err != nil {
		return err
	}
	if _, err := w.Write(eocd.Bytes()); err != nil {
		return err
	}

	return nil
}

func buildDT2Manifest(payloadName, samplePath string, wavSize int, hash uint32) string {
	return fmt.Sprintf(`{"FormatVersion":"1.0","ProductType":[],"Payload":"%s","FileType":"Sound","FirmwareVersion":"1.15B","MetaInfo":{"Tags":[]},"Samples":[{"FileName":"%s","FileSize":%d,"Hash":"%d"}]}`,
		payloadName, samplePath, wavSize, hash)
}

func sanitizeDT2Name(name string) string {
	sanitized := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == ' ' {
			sanitized = append(sanitized, c)
		} else {
			sanitized = append(sanitized, '_')
		}
	}
	result := strings.TrimSpace(string(sanitized))
	if len(result) > 12 {
		result = result[:12]
	}
	if result == "" {
		result = "OUTPUT"
	}
	return result
}

func buildDT2PresetBinary(extraction *SliceExtraction, payloadName string, hash uint32) []byte {
	numSlices := len(extraction.CuePoints)
	if numSlices == 0 {
		numSlices = 1
	}
	if numSlices > 64 {
		numSlices = 64
	}

	header := []byte{
		0xac, 0x11, 0xd3, 0x03, 0x02, 0x00, 0x04, 0x00,
		0x10, 0x30, 0x30, 0x37, 0x30, 0x00, 0x00, 0x00,
		0x03, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x04, 0x5a, 0x01, 0x0c, 0x00,
		0x00, 0x01, 0x3a, 0x91, 0x00, 0x00, 0x00, 0x03,
		0x00, 0xbe, 0xef, 0xba, 0xce, 0x09, 0x00, 0xf1,
		0x01, 0x00, 0x00, 0x00, 'S', 'O', 'L', 'E',
		' ', 'D', 'I', 'S', 'P', 'L', 'A', 'Y',
		0x00, 0x01, 0x00, 0x11, 0x70, 0x02, 0x00, 0x51,
		0x00, 0x00, 0x01, 0x00, 0x03, 0x23, 0x00, 0x11,
		0x40, 0x02, 0x00, 0x01, 0x1b, 0x00, 0x01, 0x02,
		0x00, 0x11, 0x01, 0x06, 0x00, 0x0e, 0x02, 0x00,
		0x00, 0x26, 0x00, 0x00, 0x28, 0x00, 0x11, 0x40,
		0x36, 0x00, 0x26, 0x00, 0x11, 0x22, 0x00, 0x51,
		0x64, 0x00, 0x00, 0x00, 0x7f, 0x1a, 0x00, 0x00,
		0x14, 0x00, 0x00, 0x06, 0x00, 0x00, 0x04, 0x00,
		0x02, 0x14, 0x00, 0x00, 0x02, 0x00, 0x70, 0x7f,
		0x00, 0x06, 0x00, 0x7f, 0x00, 0x20, 0x0b, 0x00,
		0x01, 0x26, 0x00, 0x11, 0x6e, 0x66, 0x00, 0x31,
		0x3f, 0x00, 0x27, 0x10, 0x00, 0x0f, 0x02, 0x00,
		0x11, 0x7f, 0x06, 0x00, 0x04, 0x00, 0x00, 0x01,
		0x02, 0x2b, 0x00, 0x11, 0x0f, 0x02, 0x00, 0x12,
		0x11, 0x02, 0xb4, 0x00, 0xf1, 0x02, 0x00, 0x00,
		0x00, 0x9c, 0x61, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x02, 0xdf, 0xe2, 0x00, 0x00, 0x87, 0xda, 0x13,
		0x00, 0x36, 0x00, 0x16, 0xfc, 0x08,
	}

	footer := []byte{
		0x00, 0x13, 0xfc, 0x80, 0x00, 0x61, 0xfc, 0xda,
		0x00, 0x01, 0x13, 0xd6, 0x18, 0x00, 0x00, 0x08,
		0x00, 0x32, 0x01, 0x2a, 0xd3, 0x0c, 0x00, 0x62,
		0x2a, 0xd3, 0x00, 0x01, 0x41, 0xcf, 0x0c, 0x00,
		0x62, 0x41, 0xcf, 0x00, 0x01, 0x58, 0xcc, 0x0c,
		0x00, 0x61, 0x58, 0xcc, 0x00, 0x01, 0x6f, 0xc9,
		0x0c, 0x00, 0x0f, 0x02, 0x00, 0xff, 0xff, 0x2f,
		0x6c, 0x0d, 0x00, 0x0e, 0x0f, 0xff, 0xff, 0x46,
		0x02, 0x50, 0x00, 0xba, 0xce, 0xf0, 0x0c, 0x00,
		0x00, 0x00, 0x00, 0xf2, 0xf4, 0x44, 0x11, 0x00,
		0x00, 0x01, 0x42, 0xaa, 0xa1, 0xda, 0xaa,
	}

	var buf bytes.Buffer
	buf.Write(header)

	for i := 0; i < numSlices; i++ {
		var position uint32
		if i < len(extraction.CuePoints) {
			position = extraction.CuePoints[i].Position
		}
		buf.WriteByte(0x00)
		buf.WriteByte(0x22)
		binary.Write(&buf, binary.LittleEndian, position)
		buf.WriteByte(0x00)
		buf.WriteByte(0x08)
	}

	buf.Write(footer)

	bin := buf.Bytes()

	nameBytes := []byte(payloadName)
	for i := 0; i < 12 && i < len(nameBytes); i++ {
		bin[0x34+i] = nameBytes[i]
	}
	for i := len(nameBytes); i < 12; i++ {
		bin[0x34+i] = 0x00
	}

	bin[0xBB] = byte(hash >> 24)
	bin[0xBC] = byte(hash >> 16)
	bin[0xBD] = byte(hash >> 8)
	bin[0xBE] = byte(hash)

	return bin
}

var _ io.Writer = (*bytes.Buffer)(nil)
