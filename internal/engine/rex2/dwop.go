package rex2

type bitReader struct {
	data     []uint32
	pos      int
	current  uint32
	bitsLeft int
	eof      bool
}

func newBitReader(data []byte) *bitReader {
	words := len(data) / 4
	buf := make([]uint32, words)
	for i := 0; i < words; i++ {
		b := i * 4
		buf[i] = (uint32(data[b]) << 24) | (uint32(data[b+1]) << 16) | (uint32(data[b+2]) << 8) | uint32(data[b+3])
	}
	return &bitReader{
		data: buf,
		pos:  0,
	}
}

func (r *bitReader) readBit() bool {
	r.bitsLeft--
	if r.bitsLeft < 0 {
		if r.pos >= len(r.data) {
			r.eof = true
			return false
		}
		r.current = r.data[r.pos]
		r.pos++
		r.bitsLeft = 31
	}
	bit := (int32(r.current) < 0)
	r.current <<= 1
	return bit
}

func (r *bitReader) readBits(n int) uint32 {
	if n <= 0 || n > 31 {
		r.eof = true
		return 0
	}
	result := r.current >> (32 - n)
	r.current <<= n
	blBefore := r.bitsLeft
	r.bitsLeft -= n
	if blBefore-n < 0 {
		if r.pos >= len(r.data) {
			r.eof = true
			return result
		}
		next := r.data[r.pos]
		r.pos++
		r.bitsLeft += 32
		result |= next >> r.bitsLeft
		r.current = next << (32 - r.bitsLeft)
	}
	return result
}

type dwopDecompressor struct {
	ch [2]struct {
		deltas   [5]int32
		averages [5]uint32
	}
	br *bitReader
}

func newDWOPDecompressor(data []byte) *dwopDecompressor {
	d := &dwopDecompressor{br: newBitReader(data)}
	for c := 0; c < 2; c++ {
		for i := 0; i < 5; i++ {
			d.ch[c].averages[i] = 2560
		}
	}
	return d
}

func mag(v int32) uint32 {
	return uint32(v ^ (v >> 31))
}

func addInt32(a, b int32) int32 {
	return int32(uint32(a) + uint32(b))
}

func subInt32(a, b int32) int32 {
	return int32(uint32(a) - uint32(b))
}

func (d *dwopDecompressor) adjustJRbits(step uint32, j *uint32, rbits *int) {
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

func applyPredictor(idx int, s2x int32, d0, d1, d2, d3, d4 *int32) int32 {
	switch idx {
	case 0:
		t0 := subInt32(s2x, *d0)
		t1 := subInt32(t0, *d1)
		t2 := subInt32(t1, *d2)
		*d4 = subInt32(t2, *d3)
		*d3 = t2
		*d2 = t1
		*d1 = t0
		*d0 = s2x
		return s2x
	case 1:
		t1 := subInt32(s2x, *d1)
		t2 := subInt32(t1, *d2)
		nd0 := addInt32(*d0, s2x)
		*d4 = subInt32(t2, *d3)
		*d3 = t2
		*d2 = t1
		*d1 = s2x
		*d0 = nd0
		return nd0
	case 2:
		nd1 := addInt32(*d1, s2x)
		nd0 := addInt32(*d0, nd1)
		t := subInt32(s2x, *d2)
		*d4 = subInt32(t, *d3)
		*d3 = t
		*d2 = s2x
		*d1 = nd1
		*d0 = nd0
		return nd0
	case 3:
		nd2 := addInt32(*d2, s2x)
		nd1 := addInt32(*d1, nd2)
		nd0 := addInt32(*d0, nd1)
		*d4 = subInt32(s2x, *d3)
		*d3 = s2x
		*d2 = nd2
		*d1 = nd1
		*d0 = nd0
		return nd0
	case 4:
		nd3 := addInt32(*d3, s2x)
		nd2 := addInt32(*d2, nd3)
		nd1 := addInt32(*d1, nd2)
		nd0 := addInt32(*d0, nd1)
		*d4 = s2x
		*d3 = nd3
		*d2 = nd2
		*d1 = nd1
		*d0 = nd0
		return nd0
	default:
		return *d0
	}
}

func updateAverages(a0, a1, a2, a3, a4 *uint32, d0, d1, d2, d3, d4 int32) {
	*a0 = *a0 + mag(d0) - (*a0 >> 5)
	*a1 = *a1 + mag(d1) - (*a1 >> 5)
	*a2 = *a2 + mag(d2) - (*a2 >> 5)
	*a3 = *a3 + mag(d3) - (*a3 >> 5)
	*a4 = *a4 + mag(d4) - (*a4 >> 5)
}

func (d *dwopDecompressor) decodeFrame(cIdx int, j *uint32, rbits *int,
	d0, d1, d2, d3, d4 *int32,
	a0, a1, a2, a3, a4 *uint32) (int32, bool) {

	r := d.br
	minAvg := *a0
	minIdx := 0
	if *a1 < minAvg {
		minAvg = *a1
		minIdx = 1
	}
	if *a2 < minAvg {
		minAvg = *a2
		minIdx = 2
	}
	if *a3 < minAvg {
		minAvg = *a3
		minIdx = 3
	}
	if *a4 < minAvg {
		minAvg = *a4
		minIdx = 4
	}

	step := ((minAvg * 3) + 36) >> 7
	var prefixSum uint32
	zerosWin := 7

	for {
		bit := r.readBit()
		if r.eof {
			return 0, false
		}
		if bit {
			break
		}
		if step > 0 && prefixSum > 0xFFFFFFFF-step {
			r.eof = true
			return 0, false
		}
		prefixSum += step
		zerosWin--
		if zerosWin == 0 {
			step <<= 2
			zerosWin = 7
		}
	}

	if r.eof {
		return 0, false
	}

	d.adjustJRbits(step, j, rbits)

	var rem uint32
	if *rbits > 0 {
		rem = r.readBits(*rbits)
		if r.eof {
			return 0, false
		}
	}

	thresh := int64(*j) - int64(step)
	if int64(rem)-thresh >= 0 {
		extra := r.readBits(1)
		if r.eof {
			return 0, false
		}
		rem = rem*2 - uint32(thresh) + extra
	}

	codeVal := rem + prefixSum
	signed2x := -(int32)(codeVal&1) ^ int32(codeVal)
	s2x := applyPredictor(minIdx, signed2x, d0, d1, d2, d3, d4)
	updateAverages(a0, a1, a2, a3, a4, *d0, *d1, *d2, *d3, *d4)
	return s2x, true
}

func (d *dwopDecompressor) DecompressMono(frameCount int, bitDepth int) ([]int32, bool) {
	out := make([]int32, frameCount)
	d0 := d.ch[0].deltas[0] * 2
	d1 := d.ch[0].deltas[1] * 2
	d2 := d.ch[0].deltas[2] * 2
	d3 := d.ch[0].deltas[3] * 2
	d4 := d.ch[0].deltas[4] * 2
	a0 := d.ch[0].averages[0]
	a1 := d.ch[0].averages[1]
	a2 := d.ch[0].averages[2]
	a3 := d.ch[0].averages[3]
	a4 := d.ch[0].averages[4]
	j := uint32(2)
	rbits := 0

	for f := 0; f < frameCount; f++ {
		s2x, ok := d.decodeFrame(0, &j, &rbits, &d0, &d1, &d2, &d3, &d4, &a0, &a1, &a2, &a3, &a4)
		if !ok {
			return out[:f], false
		}
		out[f] = clampSample(int64(s2x)>>1, bitDepth)
	}

	d.ch[0].deltas[0] = d0 >> 1
	d.ch[0].deltas[1] = d1 >> 1
	d.ch[0].deltas[2] = d2 >> 1
	d.ch[0].deltas[3] = d3 >> 1
	d.ch[0].deltas[4] = d4 >> 1
	d.ch[0].averages[0] = a0
	d.ch[0].averages[1] = a1
	d.ch[0].averages[2] = a2
	d.ch[0].averages[3] = a3
	d.ch[0].averages[4] = a4
	return out, true
}

func (d *dwopDecompressor) DecompressStereo(frameCount int, bitDepth int) ([]int32, bool) {
	out := make([]int32, frameCount*2)
	var dArr [2][5]int32
	var aArr [2][5]uint32
	for c := 0; c < 2; c++ {
		for i := 0; i < 5; i++ {
			dArr[c][i] = d.ch[c].deltas[i] * 2
			aArr[c][i] = d.ch[c].averages[i]
		}
	}
	j := [2]uint32{2, 2}
	rbits := [2]int{0, 0}

	for f := 0; f < frameCount; f++ {
		var ch2x [2]int32
		for c := 0; c < 2; c++ {
			s2x, ok := d.decodeFrame(c,
				&j[c], &rbits[c],
				&dArr[c][0], &dArr[c][1], &dArr[c][2], &dArr[c][3], &dArr[c][4],
				&aArr[c][0], &aArr[c][1], &aArr[c][2], &aArr[c][3], &aArr[c][4],
			)
			if !ok {
				return out[:f*2], false
			}
			ch2x[c] = s2x
		}
		out[f*2+0] = clampSample(int64(ch2x[0])>>1, bitDepth)
		out[f*2+1] = clampSample((int64(ch2x[0])+int64(ch2x[1]))>>1, bitDepth)
	}

	for c := 0; c < 2; c++ {
		for i := 0; i < 5; i++ {
			d.ch[c].deltas[i] = dArr[c][i] >> 1
			d.ch[c].averages[i] = aArr[c][i]
		}
	}
	return out, true
}
