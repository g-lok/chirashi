package engine

// RexMetadata holds file-level attributes from the REX SDK.
type RexMetadata struct {
	Channels      int
	SampleRate    int
	Tempo         float64 // From SDK as BPM*1000 (e.g. 123456 → 123.456)
	OriginalTempo float64
	TimeSignNom   int
	TimeSignDenom int
	BitDepth      int
	PPQLength     int     // Loop length in PPQ ticks
	CreatorName   string
	Copyright     string
}

// WavCueMarker maps a slice boundary to a sample position in the output WAV.
type WavCueMarker struct {
	SliceID  int
	Position uint32 // Frame offset within the output file
	Label    string
}

// SliceExtraction holds rendered PCM data and metadata for one or more slices.
type SliceExtraction struct {
	Metadata    RexMetadata
	CuePoints   []WavCueMarker
	Interleaved []float32 // Channel-interleaved float32 PCM
	TotalFrames int       // Number of audio frames
}

// PipelineConfig holds all CLI-driven settings for batch conversion.
type PipelineConfig struct {
	InputDir        string
	InputFiles      []string
	OutputDir       string
	OutputFile      string
	SampleRate      int
	BitRate         int
	Mono            bool
	Recursive       bool
	SliceLimit      int
	NormalizeSplits bool
	Tempo           int   // BPM override (0 = use original)
	Quiet           bool  // Suppress "Converting:" progress lines
	Preserve        bool  // Mirror input directory structure in output
	Verbose         bool
	Format          string // Output format: wav, pti, ot, aif-op1, xy, el, dt2pst
	NoSlices        bool   // Ignore REX cue positions, render single unsliced output
	MonoMode        string // Mono downmix strategy: sum, left, right, difference, dual-detect
	LibraryPath     string // Ableton User Library path for sample resolution
	InputFormat     string // Force input format (auto-detect by extension if unset)
	SamplePathMode  string // Sample path style in XML output: relative, absolute, library
	BpmPrefix       bool   // Prepend detected BPM to output filename (e.g., "128-SourceName.wav")
}
