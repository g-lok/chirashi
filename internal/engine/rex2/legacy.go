package rex2

import (
	"encoding/binary"
	"math"
	"sort"
)

func decodeLegacyAIFF(data []byte) (*REX2File, error) {
	f := &REX2File{
		Info: FileInfo{
			Channels:      1,
			SampleRate:    44100,
			Tempo:         120000,
			OriginalTempo: 120000,
			PPQLength:     61440,
			TimeSigNum:    4,
			TimeSigDen:    4,
			BitDepth:      16,
		},
		processingGain:   1000,
		transientEnabled: false,
		transientStretch: 0,
	}

	formEnd := uint64(binary.BigEndian.Uint32(data[4:])) + 8
	if formEnd > uint64(len(data)) {
		formEnd = uint64(len(data))
	}

	var channels, bits uint16
	var frameCount uint32
	var sampleRate int32
	var haveCOMM, haveSSND bool
	var ssndOffset, ssndSize uint64
	var markerLoopStart, markerLoopEnd uint32
	var haveLoopStart, haveLoopEnd bool
	var sawLegacyApp, sawLegacyTempo bool
	legacyLoopLengthSet := true
	var legacySlices []rawSliceEntry
	var legacySlicesHaveExplicitPpq bool
	var legacyRexPpqLength, legacyRexExportedFrameCount uint32

	off := uint64(12)
	for off+8 <= formEnd && off+8 <= uint64(len(data)) {
		id := string(data[off : off+4])
		sz := binary.BigEndian.Uint32(data[off+4:])
		payload := off + 8
		if payload+uint64(sz) > uint64(len(data)) {
			break
		}

		switch id {
		case "COMM":
			if sz >= 18 {
				channels = binary.BigEndian.Uint16(data[payload:])
				frameCount = binary.BigEndian.Uint32(data[payload+2:])
				bits = binary.BigEndian.Uint16(data[payload+6:])
				sampleRate = readAIFFExtendedRate(data[payload+8:])
				haveCOMM = true
			}
		case "SSND":
			if sz >= 8 {
				dataOffset := binary.BigEndian.Uint32(data[payload:])
				dataStart := payload + 8 + uint64(dataOffset)
				if dataStart <= payload+uint64(sz) && dataStart <= uint64(len(data)) {
					ssndOffset = dataStart
					ssndSize = (payload + uint64(sz)) - dataStart
					haveSSND = true
				}
			}
		case "MARK":
			if sz >= 2 {
				pos := uint32(2)
				count := binary.BigEndian.Uint16(data[payload:])
				for i := uint16(0); i < count && uint64(pos)+7 <= uint64(sz); i++ {
					markerPos := binary.BigEndian.Uint32(data[payload+uint64(pos)+2:])
					nameLen := data[payload+uint64(pos)+6]
					pos += 7
					if uint64(pos)+uint64(nameLen) > uint64(sz) {
						break
					}
					name := string(data[payload+uint64(pos) : payload+uint64(pos)+uint64(nameLen)])
					if name == "Loop start" {
						markerLoopStart = markerPos
						haveLoopStart = true
					} else if name == "Loop end" {
						markerLoopEnd = markerPos
						haveLoopEnd = true
					}
					pos += uint32(nameLen)
					if (nameLen+1)&1 != 0 {
						pos++
					}
				}
			}
		case "APPL":
			if sz >= 8 {
				appID := string(data[payload : payload+4])
				appIsREX := appID == "REX "
				appIsReCy := appID == "ReCy"
				if appIsREX || appIsReCy {
					sawLegacyApp = true
					if appIsReCy {
						legacyLoopLengthSet = sz <= 12 || data[payload+12] != 0
						legacySlices = parseLegacyReCycleSlices(data[payload : payload+uint64(sz)])
					} else if appIsREX {
						legacySlices, legacyRexPpqLength, legacyRexExportedFrameCount, legacySlicesHaveExplicitPpq =
							parseLegacyREXSlices(data[payload : payload+uint64(sz)])
					}
					sawLegacyTempo = parseLegacyTempo(data[payload:payload+uint64(sz)], appIsReCy) || sawLegacyTempo
				}
			}
		}

		off = payload + uint64(sz)
		if off&1 != 0 {
			off++
		}
	}

	if !sawLegacyApp || !sawLegacyTempo || !haveCOMM || !haveSSND ||
		channels < 1 || channels > 2 || frameCount == 0 || sampleRate <= 0 {
		return nil, ErrFileCorrupt
	}
	if !legacyLoopLengthSet {
		return nil, ErrInvalidSize
	}
	if bits != 8 && bits != 16 && bits != 24 && bits != 32 {
		return nil, ErrFileCorrupt
	}

	bytesPerSample := uint64((bits + 7) / 8)
	frameBytes := bytesPerSample * uint64(channels)
	if frameBytes == 0 {
		return nil, ErrInvalidSize
	}
	availableFrames := uint64(frameCount)
	if max := ssndSize / frameBytes; max < availableFrames {
		availableFrames = max
	}
	if availableFrames == 0 {
		return nil, ErrInvalidSize
	}

	f.totalFrames = uint32(availableFrames)
	f.PCM = make([]int32, availableFrames*uint64(channels))
	src := data[ssndOffset:]
	for frame := uint32(0); frame < f.totalFrames; frame++ {
		for ch := uint16(0); ch < channels; ch++ {
			sampleOff := (uint64(frame)*uint64(channels) + uint64(ch)) * bytesPerSample
			f.PCM[uint64(frame)*uint64(channels)+uint64(ch)] = readSignedSampleBE(src[sampleOff:], bits)
		}
	}

	f.loopStart = 0
	if haveLoopStart && markerLoopStart < f.totalFrames {
		f.loopStart = markerLoopStart
	}
	f.loopEnd = f.totalFrames
	if haveLoopEnd && markerLoopEnd > f.loopStart && markerLoopEnd <= f.totalFrames {
		f.loopEnd = markerLoopEnd
	}

	f.Info.Channels = int(channels)
	f.Info.SampleRate = int(sampleRate)
	f.Info.PPQLength = int(legacyRexPpqLength)
	if f.Info.PPQLength <= 0 {
		f.Info.PPQLength = kREXPPQ * 4
	}
	f.Info.BitDepth = int(bits)
	f.Info.TotalFrames = int(f.totalFrames)
	f.Info.LoopStart = int(f.loopStart)
	f.Info.LoopEnd = int(f.loopEnd)

	if legacyRexExportedFrameCount > 0 && f.Info.PPQLength > 0 {
		beats := float64(f.Info.PPQLength) / float64(kREXPPQ)
		bpm := beats * 60.0 * float64(f.Info.SampleRate) / float64(legacyRexExportedFrameCount)
		if t := int(math.Round(bpm * 1000.0)); t > 0 {
			f.Info.OriginalTempo = t
		}
	}

	if len(legacySlices) > 0 {
		sort.Slice(legacySlices, func(i, j int) bool {
			return legacySlices[i].sampleStart < legacySlices[j].sampleStart
		})
		// Remove duplicates
		dedup := legacySlices[:1]
		for i := 1; i < len(legacySlices); i++ {
			if legacySlices[i].sampleStart != legacySlices[i-1].sampleStart {
				dedup = append(dedup, legacySlices[i])
			}
		}
		legacySlices = dedup

		sliceEnd := f.loopEnd
		if sliceEnd <= f.loopStart {
			sliceEnd = f.totalFrames
		}
		for i := range legacySlices {
			var next uint32 = sliceEnd
			if i+1 < len(legacySlices) {
				next = legacySlices[i+1].sampleStart
			}
			if !legacySlicesHaveExplicitPpq {
				if next > legacySlices[i].sampleStart {
					legacySlices[i].sampleLength = next - legacySlices[i].sampleStart
				} else {
					legacySlices[i].sampleLength = 1
				}
			}
		}

		f.rawSlices = legacySlices
		if !legacySlicesHaveExplicitPpq {
			f.finalizeSlices()
		} else {
			f.Info.SliceCount = len(f.rawSlices)
		}
	} else {
		start := f.loopStart
		length := f.totalFrames - f.loopStart
		if f.loopEnd > f.loopStart {
			length = f.loopEnd - f.loopStart
		}
		if length < 1 {
			length = 1
		}
		f.rawSlices = []rawSliceEntry{{
			ppqPos:       0,
			sampleStart:  start,
			sampleLength: length,
			points:       0x7fff,
			selectedFlag: true,
		}}
		f.Info.SliceCount = 1
	}

	f.buildSliceInfo()
	return f, nil
}

func readAIFFExtendedRate(p []byte) int32 {
	expon := binary.BigEndian.Uint16(p)
	var mant uint64
	for i := 0; i < 8; i++ {
		mant = (mant << 8) | uint64(p[2+i])
	}
	if expon == 0 || mant == 0 {
		return 0
	}
	sign := 1
	if expon&0x8000 != 0 {
		sign = -1
	}
	exp := int(expon&0x7fff) - 16383
	value := float64(sign) * float64(mant) * math.Ldexp(1.0, exp-63)
	if value <= 0 || value > float64(math.MaxInt32) {
		return 0
	}
	return int32(math.Round(value))
}

func readSignedSampleBE(p []byte, bits uint16) int32 {
	switch bits {
	case 8:
		return int32(int8(p[0])) * 256
	case 16:
		return int32(int16(binary.BigEndian.Uint16(p)))
	case 24:
		v := (int32(p[0]) << 16) | (int32(p[1]) << 8) | int32(p[2])
		if v&0x800000 != 0 {
			v |= ^0xffffff
		}
		return v
	case 32:
		return int32(binary.BigEndian.Uint32(p)) >> 16
	default:
		return 0
	}
}

func parseLegacyTempo(d []byte, appIsReCy bool) bool {
	tempoOffset := uint32(14)
	if !appIsReCy {
		tempoOffset = 16
	}
	if uint32(len(d)) < tempoOffset+4 {
		return false
	}
	v := binary.BigEndian.Uint32(d[tempoOffset:])
	if v < 20000 || v > 450000 {
		return false
	}
	return true
}

func parseLegacyReCycleSlices(d []byte) []rawSliceEntry {
	if len(d) < 4+0xa0+1 {
		return nil
	}
	binaryData := d[4:]
	if uint32(len(binaryData)) < 0xa0 || binary.BigEndian.Uint32(binaryData) != 0xd1daded0 {
		return nil
	}
	sensitivity := binary.BigEndian.Uint16(binaryData[0x14:])
	filterPoints := legacyReCycleFilterPoints(sensitivity)
	storedCount := binary.BigEndian.Uint16(binaryData[0x9e:])
	if storedCount == 0 || storedCount > 1000 {
		return nil
	}
	needLen := 0xa0 + uint32(storedCount)*8
	if uint32(len(binaryData)) < needLen {
		return nil
	}

	var slices []rawSliceEntry
	for i := uint16(0); i < storedCount; i++ {
		rec := binaryData[0xa0+uint32(i)*8:]
		state := rec[0] & 0x7f
		selected := (rec[0] & 0x80) != 0
		start := (uint32(rec[1]) << 24) | (uint32(rec[2]) << 16) | (uint32(rec[3]) << 8) | uint32(rec[4])
		points := binary.BigEndian.Uint16(rec[6:])

		if !selected && state == 0 && points <= filterPoints {
			continue
		}

		s := rawSliceEntry{
			sampleStart:  start,
			sampleLength: 1,
			points:       points,
			selectedFlag: selected,
		}
		switch state {
		case 1:
			s.state = 2 // locked
		case 2:
			s.state = 1 // muted
		}
		slices = append(slices, s)
	}

	if len(slices) == 0 {
		return nil
	}
	return slices
}

func legacyReCycleFilterPoints(sensitivity uint16) uint16 {
	sens := uint32(sensitivity)
	if sens > 1000 {
		sens = 1000
	}
	visibleRange := (sens*0x7fff + 999) / 1000
	return uint16(0x7fff - visibleRange)
}

func parseLegacyREXSlices(d []byte) ([]rawSliceEntry, uint32, uint32, bool) {
	if len(d) < 4+0x3f8+1 {
		return nil, 0, 0, false
	}
	binaryData := d[4:]
	if uint32(len(binaryData)) < 0x3f8 || binary.BigEndian.Uint32(binaryData) != 0xd1d1d1da {
		return nil, 0, 0, false
	}

	storedPpqLength := binary.BigEndian.Uint32(binaryData[6:])
	if storedPpqLength == 0 {
		return nil, 0, 0, false
	}

	storedCount := binary.BigEndian.Uint16(binaryData[0x0a:])
	if storedCount == 0 || storedCount > 1000 {
		return nil, 0, 0, false
	}

	needLen := 0x3f8 + uint32(storedCount)*12
	if uint32(len(binaryData)) < needLen {
		return nil, 0, 0, false
	}

	var slices []rawSliceEntry
	var ppqPositions []uint32
	var sourceLengths []uint32

	for i := uint16(0); i < storedCount; i++ {
		rec := binaryData[0x3f8+uint32(i)*12:]
		start := binary.BigEndian.Uint32(rec)
		length := binary.BigEndian.Uint32(rec[4:])
		ppq16 := binary.BigEndian.Uint32(rec[8:])
		if ppq16 > storedPpqLength {
			return nil, 0, 0, false
		}
		if length == 0 {
			continue
		}
		slices = append(slices, rawSliceEntry{
			ppqPos:       ppq16 * 16,
			sampleStart:  start,
			sampleLength: length,
			points:       0x7fff,
			selectedFlag: i == 0,
		})
		ppqPositions = append(ppqPositions, ppq16)
		sourceLengths = append(sourceLengths, length)
	}

	if len(slices) == 0 {
		return nil, 0, 0, false
	}

	var samplesPerPpq float64
	for i := range ppqPositions {
		nextPpq := storedPpqLength
		for j := i + 1; j < len(ppqPositions); j++ {
			if ppqPositions[j] != ppqPositions[i] {
				nextPpq = ppqPositions[j]
				break
			}
		}
		if nextPpq <= ppqPositions[i] {
			continue
		}
		delta := nextPpq - ppqPositions[i]
		ratio := float64(sourceLengths[i]) / float64(delta)
		if ratio > samplesPerPpq {
			samplesPerPpq = ratio
		}
	}

	if samplesPerPpq <= 0 {
		return nil, 0, 0, false
	}

	exportedFrameCount := uint32(samplesPerPpq * float64(storedPpqLength))
	return slices, storedPpqLength * 16, exportedFrameCount, true
}
