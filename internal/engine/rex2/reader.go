package rex2

import (
	"encoding/binary"
	"errors"
	"math"
	"sort"
)

var (
	ErrInvalidSize   = errors.New("rex2: invalid size")
	ErrFileCorrupt   = errors.New("rex2: file corrupt")
	ErrInvalidTempo  = errors.New("rex2: invalid tempo")
	ErrFileTooNew    = errors.New("rex2: file version too new")
	ErrUnknownFormat = errors.New("rex2: unknown file format")
)

const kMaxTotalFrames = 3600 * 192000

type rawSliceEntry struct {
	ppqPos           uint32
	sampleStart      uint32
	sampleLength     uint32
	renderedLength   uint32
	points           uint16
	marker           bool
	selectedFlag     bool
	state            uint8 // 0=normal, 1=muted, 2=locked
	syntheticLeading bool
}

type REX2File struct {
	Info       FileInfo
	PCM        []int32
	Slices     []SliceInfo
	SliceFlags []int32
	Creator    CreatorInfo

	rawSlices            []rawSliceEntry
	processingGain       int
	transientEnabled     bool
	transientAttack      uint16
	transientDecay       uint16
	transientStretch     int
	analysisSensitivity  uint8
	gateSensitivity      uint16
	globBars             uint16
	globBeats            uint8
	loopStart, loopEnd   uint32
	totalFrames          uint32
}

func Decode(data []byte) (*REX2File, error) {
	if len(data) < 12 {
		return nil, ErrInvalidSize
	}

	if string(data[0:4]) == "FORM" && string(data[8:12]) == "AIFF" {
		return decodeLegacyAIFF(data)
	}

	if string(data[0:4]) != "CAT " {
		return nil, ErrUnknownFormat
	}

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
		transientEnabled: true,
		transientStretch: 0x28,
	}

	var dwopOffset, dwopSize uint64
	hasDWOP := false
	f.parseIFF(8+4, uint64(len(data)), data, &dwopOffset, &dwopSize, &hasDWOP)

	if !hasDWOP || dwopSize == 0 || f.totalFrames == 0 {
		return nil, ErrFileCorrupt
	}
	if dwopOffset+dwopSize > uint64(len(data)) {
		return nil, ErrInvalidSize
	}
	if f.totalFrames > kMaxTotalFrames {
		return nil, ErrInvalidSize
	}

	f.finalizeSlices()

	pcmElements := int(f.totalFrames) * f.Info.Channels
	f.PCM = make([]int32, pcmElements)

	dec := newDWOPDecompressor(data[dwopOffset : dwopOffset+dwopSize])
	var done uint32
	for done < f.totalFrames {
		chunk := f.totalFrames - done
		if chunk > 0x100000 {
			chunk = 0x100000
		}
		if f.Info.Channels == 1 {
			chunkOut, ok := dec.DecompressMono(int(chunk), f.Info.BitDepth)
			if !ok {
				return nil, ErrFileCorrupt
			}
			copy(f.PCM[done:], chunkOut)
		} else {
			chunkOut, ok := dec.DecompressStereo(int(chunk), f.Info.BitDepth)
			if !ok {
				return nil, ErrFileCorrupt
			}
			copy(f.PCM[done*2:], chunkOut)
		}
		done += chunk
	}

	f.buildSliceInfo()

	return f, nil
}

func (f *REX2File) parseIFF(start, end uint64, data []byte, dwopOffset, dwopSize *uint64, hasDWOP *bool) {
	off := start
	for off+8 < end && off+8 < uint64(len(data)) {
		id := string(data[off : off+4])
		off += 4
		sz := binary.BigEndian.Uint32(data[off : off+4])
		off += 4
		if off+uint64(sz) > uint64(len(data)) {
			break
		}

		d := data[off : off+uint64(sz)]

		switch id {
		case "HEAD":
			f.parseHEAD(d)
		case "SINF":
			f.parseSINF(d)
		case "GLOB":
			f.parseGLOB(d)
		case "SLCE":
			f.parseSLCE(d)
		case "CREI":
			f.parseCREI(d)
		case "TRSH":
			f.parseTRSH(d)
		case "RECY":
			if len(d) >= 12 {
				if t := int32(binary.BigEndian.Uint32(d[8:])); t > 0 {
					f.Info.OriginalTempo = int(t)
				}
			}
		case "SDAT", "DWOP":
			if !*hasDWOP {
				*dwopOffset = off
				*dwopSize = uint64(sz)
				*hasDWOP = true
			}
		case "CAT ":
			if sz >= 4 {
				f.parseIFF(off+4, off+uint64(sz), data, dwopOffset, dwopSize, hasDWOP)
			}
		}

		off += uint64(sz)
		if off&1 != 0 {
			off++
		}
	}
}

func (f *REX2File) parseHEAD(d []byte) {
	if len(d) < 6 {
		return
	}
	if binary.BigEndian.Uint32(d) != 0x490cf18d || d[4] != 0xbc {
		return
	}
	if d[5] > 0x03 {
		return
	}
}

func (f *REX2File) parseSINF(d []byte) {
	if len(d) < 18 {
		return
	}
	f.Info.Channels = int(d[0])
	bd := d[1]
	f.Info.SampleRate = int(binary.BigEndian.Uint32(d[2:]))
	f.totalFrames = binary.BigEndian.Uint32(d[6:])
	f.loopStart = binary.BigEndian.Uint32(d[10:])
	f.loopEnd = binary.BigEndian.Uint32(d[14:])
	f.Info.TotalFrames = int(f.totalFrames)
	f.Info.LoopStart = int(f.loopStart)
	f.Info.LoopEnd = int(f.loopEnd)

	switch bd {
	case 1:
		f.Info.BitDepth = 8
	case 3:
		f.Info.BitDepth = 16
	case 5:
		f.Info.BitDepth = 24
	case 7:
		f.Info.BitDepth = 32
	default:
		f.Info.BitDepth = 16
	}

	frames := f.loopEnd - f.loopStart
	if f.loopEnd <= f.loopStart {
		frames = f.totalFrames
	}
	if frames > 0 && f.Info.SampleRate > 0 && f.Info.PPQLength > 0 {
		beats := float64(f.Info.PPQLength) / float64(kREXPPQ)
		bpm := beats * 60.0 * float64(f.Info.SampleRate) / float64(frames)
		if t := int(math.Round(bpm * 1000.0)); t > 0 {
			f.Info.OriginalTempo = t
		}
	}
	if f.Info.OriginalTempo == 0 {
		f.Info.OriginalTempo = f.Info.Tempo
	}
	if f.Info.Channels != 1 && f.Info.Channels != 2 {
		f.Info.Channels = 1
	}
}

func (f *REX2File) parseGLOB(d []byte) {
	if len(d) < 22 {
		return
	}
	f.Info.SliceCount = int(binary.BigEndian.Uint32(d))
	f.globBars = binary.BigEndian.Uint16(d[4:])
	f.globBeats = d[6]
	f.Info.TimeSigNum = int(d[7])
	f.Info.TimeSigDen = int(d[8])
	f.analysisSensitivity = d[9]
	f.gateSensitivity = binary.BigEndian.Uint16(d[10:])
	f.processingGain = int(binary.BigEndian.Uint16(d[12:]))

	tempo := binary.BigEndian.Uint32(d[16:])
	if tempo < 20000 || tempo > 450000 {
		return
	}
	f.Info.Tempo = int(tempo)

	timeSigNum := f.Info.TimeSigNum
	if timeSigNum <= 0 {
		timeSigNum = 4
	}
	totalBeats := int(f.globBars)*timeSigNum + int(f.globBeats)
	ppqLen := int64(totalBeats) * kREXPPQ
	if totalBeats <= 0 {
		ppqLen = 4 * kREXPPQ
	}
	if ppqLen < 1 {
		ppqLen = 1
	}
	if ppqLen > 1<<31-1 {
		ppqLen = 1<<31 - 1
	}
	f.Info.PPQLength = int(ppqLen)
}

func (f *REX2File) parseCREI(d []byte) {
	var off uint32
	readStr := func() string {
		if off+4 > uint32(len(d)) {
			return ""
		}
		n := binary.BigEndian.Uint32(d[off:])
		off += 4
		if off+n > uint32(len(d)) {
			off = uint32(len(d))
			return ""
		}
		s := string(d[off : off+n])
		off += n
		return s
	}
	f.Creator.Name = readStr()
	f.Creator.Copyright = readStr()
	f.Creator.URL = readStr()
	f.Creator.Email = readStr()
	f.Creator.FreeText = readStr()
}

func (f *REX2File) parseTRSH(d []byte) {
	if len(d) < 7 {
		return
	}
	f.transientEnabled = d[0] != 0
	f.transientAttack = binary.BigEndian.Uint16(d[1:])
	f.transientDecay = binary.BigEndian.Uint16(d[3:])
	f.transientStretch = int(binary.BigEndian.Uint16(d[5:]))
}

func (f *REX2File) parseSLCE(d []byte) {
	if len(d) < 10 {
		return
	}
	s := rawSliceEntry{
		sampleStart:  binary.BigEndian.Uint32(d),
		sampleLength: binary.BigEndian.Uint32(d[4:]),
		points:       binary.BigEndian.Uint16(d[8:]),
		marker:       binary.BigEndian.Uint32(d[4:]) <= 1,
	}
	if len(d) > 10 {
		flags := d[10]
		s.selectedFlag = (flags & 0x04) != 0
		switch {
		case flags&0x02 != 0:
			s.state = 2 // locked
		case flags&0x01 != 0:
			s.state = 1 // muted
		}
	}
	if len(f.rawSlices) < kMaxSlices {
		f.rawSlices = append(f.rawSlices, s)
	}
}

func rex2FilterPoints(sensitivity uint8) uint16 {
	sens := uint32(sensitivity)
	if sens > 99 {
		sens = 99
	}
	visibleRange := (sens*0x7fff + 98) / 99
	return uint16(0x7fff - visibleRange)
}

func (f *REX2File) isVisibleSliceBoundary(s rawSliceEntry) bool {
	if s.state == 1 {
		return false
	}
	if s.sampleLength > 1 {
		return true
	}
	if s.state == 2 {
		return true
	}
	return s.points > rex2FilterPoints(f.analysisSensitivity)
}

func (f *REX2File) defaultSliceEnd(start uint32) uint32 {
	if f.loopEnd > f.loopStart && start < f.loopEnd {
		return f.loopEnd
	}
	return f.totalFrames
}

func (f *REX2File) gateLengthFrames() uint32 {
	if f.gateSensitivity == 0 {
		return 0
	}
	sr := uint32(f.Info.SampleRate)
	if sr == 0 {
		sr = 44100
	}
	frames := (uint64(f.gateSensitivity)*uint64(sr) + 4500) / 9000
	if frames < 1 {
		frames = 1
	}
	return uint32(((frames + 64) / 128) * 128)
}

func (f *REX2File) finalizeSlices() {
	denom := f.loopEnd - f.loopStart
	if denom == 0 {
		denom = f.totalFrames
		if denom == 0 {
			denom = 1
		}
	}
	gatedFrames := f.gateLengthFrames()

	sort.Slice(f.rawSlices, func(i, j int) bool {
		return f.rawSlices[i].sampleStart < f.rawSlices[j].sampleStart
	})

	var out []rawSliceEntry
	for _, s := range f.rawSlices {
		if f.loopEnd > f.loopStart && s.sampleStart < f.totalFrames && s.sampleStart >= f.loopEnd {
			continue
		}
		if !f.isVisibleSliceBoundary(s) {
			continue
		}
		rel := s.sampleStart - f.loopStart
		if s.sampleStart < f.loopStart {
			rel = 0
		}
		s.ppqPos = uint32((uint64(rel)*uint64(f.Info.PPQLength) + uint64(denom)/2) / uint64(denom))
		s.marker = false
		out = append(out, s)
	}

	for i := range out {
		start := out[i].sampleStart
		next := f.defaultSliceEnd(start)
		for j := i + 1; j < len(out); j++ {
			if out[j].sampleStart > start {
				next = out[j].sampleStart
				break
			}
		}
		derived := next - start
		if derived < 1 {
			derived = 1
		}

		if f.gateSensitivity == 0 || out[i].sampleLength <= 1 {
			out[i].sampleLength = derived
			if f.gateSensitivity != 0 && gatedFrames > 0 && out[i].sampleLength > gatedFrames {
				out[i].sampleLength = gatedFrames
			}
		} else if derived > 1 && out[i].sampleLength > derived {
			out[i].sampleLength = derived
		}

		if out[i].sampleLength < 1 {
			out[i].sampleLength = 1
		}
	}

	if len(out) > 0 && f.loopEnd > f.loopStart &&
		out[0].sampleStart > f.loopStart && out[0].sampleStart <= f.totalFrames {
		leading := rawSliceEntry{
			ppqPos:           0,
			sampleStart:      f.loopStart,
			sampleLength:     out[0].sampleStart - f.loopStart,
			points:           0x7fff,
			selectedFlag:     true,
			state:            0,
			syntheticLeading: true,
		}
		out = append([]rawSliceEntry{leading}, out...)
	}

	f.rawSlices = out
	f.Info.SliceCount = len(out)
}

func (f *REX2File) sourceEndForSlice(s rawSliceEntry) uint32 {
	if s.sampleStart >= f.totalFrames {
		return s.sampleStart
	}
	frames := s.sampleLength
	if max := f.totalFrames - s.sampleStart; frames > max {
		frames = max
	}
	if f.loopEnd > f.loopStart && s.sampleStart < f.loopEnd {
		if max := f.loopEnd - s.sampleStart; frames > max {
			frames = max
		}
	}
	return s.sampleStart + frames
}

func (f *REX2File) calcRenderedLength(s rawSliceEntry) uint32 {
	start := s.sampleStart
	end := f.sourceEndForSlice(s)
	if end <= start {
		return 1
	}
	if !f.transientEnabled || f.transientStretch == 0 {
		return end - start
	}
	return 0 // simplified; transient stretch handled later
}

func (f *REX2File) finalizeRenderedLengths() {
	for i, s := range f.rawSlices {
		_ = i
		s.renderedLength = f.calcRenderedLength(s)
	}
}

func (f *REX2File) buildSliceInfo() {
	f.Slices = make([]SliceInfo, len(f.rawSlices))
	f.SliceFlags = make([]int32, len(f.rawSlices))
	for i, s := range f.rawSlices {
		f.Slices[i] = SliceInfo{
			PPQPos:       int(s.ppqPos),
			SampleStart:  int(s.sampleStart),
			SampleLength: int(s.sampleLength),
		}
		var flags int32
		switch s.state {
		case 1:
			flags |= 1 // muted
		case 2:
			flags |= 2 // locked
		}
		if s.selectedFlag {
			flags |= 4
		}
		if s.marker {
			flags |= 8
		}
		if s.syntheticLeading {
			flags |= 16
		}
		f.SliceFlags[i] = flags
	}
}
