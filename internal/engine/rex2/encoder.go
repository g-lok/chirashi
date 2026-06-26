package rex2

import (
	"encoding/binary"
)

// bitWriter accumulates bits into big-endian 32-bit words (DWOP format).
type bitWriter struct {
	bytes      []byte
	current    uint32
	bitCount   int
}

func (w *bitWriter) writeBit(bit bool) {
	if bit {
		w.current |= uint32(1) << (31 - w.bitCount)
	}
	w.bitCount++
	if w.bitCount == 32 {
		w.flushWord()
	}
}

func (w *bitWriter) writeBits(value uint32, count int) {
	for i := count - 1; i >= 0; i-- {
		w.writeBit(((value >> i) & 1) != 0)
	}
}

func (w *bitWriter) flushWord() {
	w.bytes = append(w.bytes,
		byte(w.current>>24),
		byte(w.current>>16),
		byte(w.current>>8),
		byte(w.current),
	)
	w.current = 0
	w.bitCount = 0
}

func (w *bitWriter) finish() []byte {
	if w.bitCount > 0 {
		w.flushWord()
	}
	return w.bytes
}

// dwopCompressor is the inverse of dwopDecompressor.
type dwopCompressor struct{}

func newDWOPCompressor() *dwopCompressor {
	return &dwopCompressor{}
}

func (c *dwopCompressor) minAverageIndex(a [5]uint32) int {
	idx := 0
	for i := 1; i < 5; i++ {
		if a[i] < a[idx] {
			idx = i
		}
	}
	return idx
}

func (c *dwopCompressor) predictorResidual(idx int, sample2x int32, d [5]int32) int32 {
	switch idx {
	case 0:
		return sample2x
	case 1:
		return subInt32(sample2x, d[0])
	case 2:
		return subInt32(subInt32(sample2x, d[0]), d[1])
	case 3:
		return subInt32(subInt32(subInt32(sample2x, d[0]), d[1]), d[2])
	case 4:
		return subInt32(subInt32(subInt32(subInt32(sample2x, d[0]), d[1]), d[2]), d[3])
	default:
		return sample2x
	}
}

func toCodeValue(signed2x int32) uint32 {
	if signed2x >= 0 {
		return uint32(signed2x)
	}
	return uint32(-signed2x - 1)
}

func (c *dwopCompressor) encodeRemainder(raw uint32, step, j uint32, rbits int) (remBits uint32, hasExtra, extraBit bool, ok bool) {
	if rbits < 0 || rbits > 31 {
		return 0, false, false, false
	}
	var limit uint32
	if rbits == 31 {
		limit = 0x80000000
	} else {
		limit = 1 << uint(rbits)
	}
	thresh := int32(j) - int32(step)
	if thresh < 0 {
		return 0, false, false, false
	}

	if raw < uint32(thresh) {
		if raw >= limit {
			return 0, false, false, false
		}
		return raw, false, false, true
	}

	folded := raw + uint32(thresh)
	remBits = folded >> 1
	hasExtra = true
	extraBit = (folded & 1) != 0
	if remBits < uint32(thresh) || remBits >= limit {
		return 0, false, false, false
	}
	return remBits, true, extraBit, true
}

func (c *dwopCompressor) adjustJRbits(step uint32, j *uint32, rbits *int) {
	if step < *j {
		for jt := *j >> 1; step < jt; jt >>= 1 {
			*j = jt
			*rbits--
		}
	} else {
		for step >= *j {
			prev := *j
			*j <<= 1
			*rbits++
			if *j <= prev {
				*j = prev
				break
			}
		}
	}
}

func (c *dwopCompressor) writeCodeValue(codeVal, baseStep uint32, j *uint32, rbits *int, bw *bitWriter) {
	var prefixSum uint32
	step := baseStep
	zerosWin := 7

	for zeros := uint32(0); zeros < 0x100000; zeros++ {
		if codeVal >= prefixSum {
			trialJ := *j
			trialRbits := *rbits
			c.adjustJRbits(step, &trialJ, &trialRbits)
			remBits, hasExtra, extraBit, ok := c.encodeRemainder(codeVal-prefixSum, step, trialJ, trialRbits)
			if ok {
				for i := uint32(0); i < zeros; i++ {
					bw.writeBit(false)
				}
				bw.writeBit(true)
				if trialRbits > 0 {
					bw.writeBits(remBits, trialRbits)
				}
				if hasExtra {
					bw.writeBit(extraBit)
				}
				*j = trialJ
				*rbits = trialRbits
				return
			}
		}
		if baseStep == 0 {
			break
		}
		if ^uint32(0)-prefixSum < step {
			break
		}
		prefixSum += step
		if prefixSum > codeVal && step != 0 {
			break
		}
		zerosWin--
		if zerosWin == 0 {
			step <<= 2
			zerosWin = 7
		}
	}

	bw.writeBit(true)
}

func (c *dwopCompressor) encodeChannel(sample2x int32, d *[5]int32, a *[5]uint32, j *uint32, rbits *int, bw *bitWriter) {
	idx := c.minAverageIndex(*a)
	baseStep := ((a[idx] * 3) + 36) >> 7
	residual := c.predictorResidual(idx, sample2x, *d)
	c.writeCodeValue(toCodeValue(residual), baseStep, j, rbits, bw)

	applyPredictor(idx, residual, &d[0], &d[1], &d[2], &d[3], &d[4])
	updateAverages(&a[0], &a[1], &a[2], &a[3], &a[4], d[0], d[1], d[2], d[3], d[4])
}

func (c *dwopCompressor) compressMono(in []int32) []byte {
	bw := &bitWriter{}
	var d [5]int32
	var a = [5]uint32{2560, 2560, 2560, 2560, 2560}
	j := uint32(2)
	rbits := 0

	for f := 0; f < len(in); f++ {
		c.encodeChannel(in[f]*2, &d, &a, &j, &rbits, bw)
	}
	return bw.finish()
}

func (c *dwopCompressor) compressStereo(in []int32) []byte {
	bw := &bitWriter{}
	var d [2][5]int32
	var a = [2][5]uint32{
		{2560, 2560, 2560, 2560, 2560},
		{2560, 2560, 2560, 2560, 2560},
	}
	var j = [2]uint32{2, 2}
	var rbits [2]int

	for f := 0; f < len(in)/2; f++ {
		left2x := in[f*2] * 2
		right2x := in[f*2+1] * 2
		c.encodeChannel(left2x, &d[0], &a[0], &j[0], &rbits[0], bw)
		c.encodeChannel(right2x-left2x, &d[1], &a[1], &j[1], &rbits[1], bw)
	}
	return bw.finish()
}

// iffWriter builds an IFF container with big-endian chunk structure.
type iffWriter struct {
	data []byte
}

func (w *iffWriter) beginChunk(id string) int {
	start := len(w.data)
	w.data = append(w.data, []byte(id[:4])...)
	w.data = append(w.data, 0, 0, 0, 0)
	return start
}

func (w *iffWriter) beginCat(formType string) int {
	start := w.beginChunk("CAT ")
	w.data = append(w.data, []byte(formType[:4])...)
	return start
}

func (w *iffWriter) endChunk(start int) {
	payloadStart := start + 8
	size := uint32(len(w.data) - payloadStart)
	w.data[start+4] = byte(size >> 24)
	w.data[start+5] = byte(size >> 16)
	w.data[start+6] = byte(size >> 8)
	w.data[start+7] = byte(size)
	if len(w.data)&1 != 0 {
		w.data = append(w.data, 0)
	}
}

func (w *iffWriter) put8(v uint8) {
	w.data = append(w.data, v)
}

func (w *iffWriter) put16(v uint16) {
	w.data = append(w.data, byte(v>>8), byte(v))
}

func (w *iffWriter) put32(v uint32) {
	w.data = append(w.data, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func (w *iffWriter) put64(v uint64) {
	w.data = append(w.data,
		byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32),
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v),
	)
}

func (w *iffWriter) putBytes(p []byte) {
	w.data = append(w.data, p...)
}

func putString(w *iffWriter, s string) {
	n := len(s)
	if n > 255 {
		n = 255
	}
	w.put32(uint32(n))
	if n > 0 {
		w.putBytes([]byte(s[:n]))
	}
}

// Encode creates a complete REX2 binary from PCM data and metadata.
func Encode(pcm []int32, info FileInfo, slices []SliceInfo, creator CreatorInfo) ([]byte, error) {
	frameCount := info.TotalFrames
	if frameCount == 0 && len(pcm) > 0 {
		frameCount = len(pcm) / info.Channels
	}
	if frameCount <= 0 {
		return nil, ErrInvalidSize
	}
	if info.Channels != 1 && info.Channels != 2 {
		info.Channels = 1
	}

	w := &iffWriter{}

	catStart := w.beginCat("REX2")

	// HEAD chunk
	hStart := w.beginChunk("HEAD")
	w.put32(0x490cf18d)
	w.put8(0xbc)
	w.put8(0x00) // version
	w.endChunk(hStart)

	// CREI chunk (optional)
	if creator.Name != "" || creator.Copyright != "" || creator.URL != "" ||
		creator.Email != "" || creator.FreeText != "" {
		cStart := w.beginChunk("CREI")
		putString(w, creator.Name)
		putString(w, creator.Copyright)
		putString(w, creator.URL)
		putString(w, creator.Email)
		putString(w, creator.FreeText)
		w.endChunk(cStart)
	}

	// GLOB chunk
	timeSigNum := info.TimeSigNum
	if timeSigNum <= 0 {
		timeSigNum = 4
	}
	timeSigDen := info.TimeSigDen
	if timeSigDen <= 0 {
		timeSigDen = 4
	}
	ppqTotal := info.PPQLength
	if ppqTotal <= 0 {
		ppqTotal = int(kREXPPQ) * 4 * timeSigNum / 4
	}
	totalBeats := ppqTotal / kREXPPQ
	bars := totalBeats / timeSigNum
	beats := totalBeats % timeSigNum

	// Ensure valid tempo (REX requires 20-450 BPM, store as BPM*1000)
	const minTempo = 20000  // 20 BPM
	const maxTempo = 450000 // 450 BPM
	const defaultTempo = 120000 // 120 BPM

	tempo := info.Tempo
	if tempo < minTempo {
		tempo = defaultTempo
	}
	if tempo > maxTempo {
		tempo = maxTempo
	}

	gStart := w.beginChunk("GLOB")
	w.put32(uint32(len(slices)))
	w.put16(uint16(bars))
	w.put8(uint8(beats))
	w.put8(uint8(timeSigNum))
	w.put8(uint8(timeSigDen))
	w.put8(0) // analysisSensitivity
	w.put16(0) // gateSensitivity
	w.put16(1000) // processingGain
	w.put16(0) // reserved
	w.put32(uint32(tempo))
	w.put16(0) // silenceSelected (2 bytes to match original)
	w.endChunk(gStart)

	// RECY chunk (tempo info) - skip for now, these values are obscure
	// if info.Tempo > 0 {
	// 	rStart := w.beginChunk("RECY")
	// 	w.putBytes([]byte{0xbc, 0x03, 0x00, 0x00}) // marker BE
	// 	w.put32(0x00000001) // ?
	// 	w.put16(uint16(info.Tempo / 2000)) // tempo / 2
	// 	w.put32(0x08000000) // ?
	// 	w.put8(0x3d)        // ?
	// 	w.endChunk(rStart)
	// }

	// RCYX chunk (flags) - skip for now
	// rxStart := w.beginChunk("RCYX")
	// w.putBytes([]byte{0xbc, 0x01, 0x00, 0x00}) // marker BE
	// w.putBytes([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) // zeros
	// w.put8(0x01)
	// w.put8(0x01)
	// w.put8(0x01)
	// w.put8(0x00)
	// w.put8(0x00)
	// w.put8(0x04)
	// w.put8(0x29)
	// w.put32(0x02260100) // ?
	// w.endChunk(rxStart)

	// DEVL container (TRSH, EQ, COMP) - just skip for now, not required
	// devStart := w.beginCat("DEVL")
	// trStart := w.beginChunk("TRSH")
	// w.put32(0x01001503)
	// w.put16(0xff00)
	// w.put8(0x64)
	// w.endChunk(trStart)
	// eqStart := w.beginChunk("EQ   ")
	// w.put32(0x00000f00)
	// w.put16(0x6400)
	// w.put32(0x000003e8)
	// w.put16(0x09c4)
	// w.put32(0x000003e8)
	// w.put16(0x4e20)
	// w.endChunk(eqStart)
	// coStart := w.beginChunk("COMP")
	// w.put16(0x0000)
	// w.put16(0x4d00)
	// w.put16(0x2700)
	// w.put8(0xc1)
	// w.put8(0x01)
	// w.put16(0x5e)
	// w.endChunk(coStart)
	// w.endChunk(devStart)

	// SLCE chunks inside nested CAT container
	if len(slices) > 0 {
		slCatStart := w.beginCat("SLCL")
		for _, s := range slices {
			slStart := w.beginChunk("SLCE")
			w.put32(uint32(s.SampleStart))
			w.put32(uint32(s.SampleLength))
			w.put16(0x7fff)
			w.put8(0) // flags
			w.endChunk(slStart)
		}
		w.endChunk(slCatStart)
	}

	// SINF chunk (after SLCE per REX2 spec)
	siStart := w.beginChunk("SINF")
	w.put8(uint8(info.Channels))
	w.put8(bitDepthCode(info.BitDepth))
	w.put32(uint32(info.SampleRate))
	w.put32(uint32(frameCount))
	w.put32(uint32(info.LoopStart))
	w.put32(uint32(info.LoopEnd))
	w.endChunk(siStart)

	// SDAT chunk (DWOP compressed audio)
	dStart := w.beginChunk("SDAT")
	comp := newDWOPCompressor()
	var dwopData []byte
	if info.Channels == 1 {
		dwopData = comp.compressMono(pcm)
	} else {
		dwopData = comp.compressStereo(pcm)
	}
	w.putBytes(dwopData)
	w.endChunk(dStart)

	w.endChunk(catStart)

	return w.data, nil
}

// encodeChannel is an alias for use in compression tests
var _ = binary.BigEndian
