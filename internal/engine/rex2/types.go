package rex2

const (
	kREXPPQ    = 15360
	kMaxSlices = 1024
)

type FileInfo struct {
	Channels      int
	SampleRate    int
	SliceCount    int
	Tempo         int // BPM * 1000
	OriginalTempo int
	PPQLength     int
	TimeSigNum    int
	TimeSigDen    int
	BitDepth      int
	TotalFrames   int
	LoopStart     int
	LoopEnd       int
}

type SliceInfo struct {
	PPQPos       int
	SampleStart  int
	SampleLength int
}

type CreatorInfo struct {
	Name      string
	Copyright string
	URL       string
	Email     string
	FreeText  string
}

func bitDepthCode(depth int) uint8 {
	switch depth {
	case 8:
		return 1
	case 16:
		return 3
	case 24:
		return 5
	case 32:
		return 7
	default:
		return 3
	}
}

func bitDepthFromCode(code uint8) int {
	switch code {
	case 1:
		return 8
	case 3:
		return 16
	case 5:
		return 24
	case 7:
		return 32
	default:
		return 16
	}
}

func PCMToFloat32(s int32, bitDepth int) float32 {
	switch bitDepth {
	case 16:
		return float32(s) / 32768.0
	case 24:
		return float32(s) / 8388608.0
	default:
		return float32(s) / 32768.0
	}
}

func Float32ToPCM(s float32, bitDepth int) int32 {
	switch bitDepth {
	case 16:
		if s >= 0 {
			return int32(s * 32767.0)
		}
		return int32(s * 32768.0)
	case 24:
		if s >= 0 {
			return int32(s * 8388607.0)
		}
		return int32(s * 8388608.0)
	default:
		if s >= 0 {
			return int32(s * 32767.0)
		}
		return int32(s * 32768.0)
	}
}

func clampSample(v int64, bitDepth int) int32 {
	var maxS, minS int64
	switch bitDepth {
	case 16:
		maxS, minS = 32767, -32768
	case 24:
		maxS, minS = 8388607, -8388608
	default:
		maxS, minS = 32767, -32768
	}
	if v > maxS {
		return int32(maxS)
	}
	if v < minS {
		return int32(minS)
	}
	return int32(v)
}
