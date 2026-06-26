package engine

import (
	"math"
	"testing"
)

func TestExtractBPMFromName(t *testing.T) {
	tests := []struct {
		name  string
		want  float64
		label string
	}{
		// Pattern 1: explicit "bpm" keyword suffix
		{"SM101_brk_Bongo Bar_110bpm", 110, "suffix _NNNbpm"},
		{"SM101_brk_animal break_140bpm", 140, "suffix _NNNbpm with spaces"},
		{"cool_128bpm_loop", 128, "bpm keyword in middle"},
		{"128bpm_loop", 128, "bpm keyword at start"},
		{"loop_128bpm", 128, "bpm keyword at end"},

		// Pattern 1: with decimal
		{"loop_140.5bpm", 140.5, "fractional bpm suffix"},
		{"120.0bpm_test", 120, "decimal .0 bpm"},

		// Pattern 1: various separators
		{"loop-128bpm-thing", 128, "hyphen separator before bpm"},
		{"loop 128bpm thing", 128, "space separator before bpm"},
		{"loop(128bpm)", 128, "paren separator before bpm"},

		// Pattern 2: numeric prefix
		{"120Stereo", 120, "3-digit prefix"},
		{"100HasCreatorInfo", 100, "3-digit prefix + text"},
		{"120FourBeats", 120, "3-digit prefix + CamelCase"},
		{"240FiveHundredSlices", 240, "3-digit prefix (240)"},

		// Pattern 2: 3-digit prefix with separator
		{"128_loop", 128, "3-digit prefix with underscore"},
		{"128-loop", 128, "3-digit prefix with hyphen"},
		{"128 loop", 128, "3-digit prefix with space"},

		// No match cases
		{"Slice_001", 0, "leading zeros not BPM"},
		{"PreviewRender_Tempo120000", 0, "Tempo prefix not match"},
		{"sample", 0, "no BPM"},
		{"output", 0, "no BPM in output"},
		{"", 0, "empty string"},
		{"12short", 0, "2-digit not BPM"},
		{"1000long", 0, "4-digit prefix out of range"},

		// Edge cases
		{"808State", 0, "808 out of range (>500)"},
		{"30bpm", 30, "30 BPM at lower boundary"},
		{"500bpm", 500, "500 BPM at upper boundary"},
		{"501bpm", 0, "501 BPM out of range"},
		{"29bpm", 0, "29 BPM out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_"+tt.label, func(t *testing.T) {
			got := extractBPMFromName(tt.name)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("extractBPMFromName(%q) = %v, want %v (%s)", tt.name, got, tt.want, tt.label)
			}
		})
	}
}

func TestFormatBPMPrefix(t *testing.T) {
	tests := []struct {
		bpm  float64
		want string
	}{
		{128, "128-"},
		{128.0, "128-"},
		{140.5, "140.5-"},
		{90, "90-"},
		{0, ""},
		{-1, ""},
		{120.333, "120.3-"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatBPMPrefix(tt.bpm)
			if got != tt.want {
				t.Errorf("formatBPMPrefix(%v) = %q, want %q", tt.bpm, got, tt.want)
			}
		})
	}
}

func TestResolveBPM(t *testing.T) {
	type args struct {
		meta          RexMetadata
		sourceName    string
		tempoOverride int
	}

	tests := []struct {
		name          string
		args          args
		wantBPM       float64
		wantFromMeta  bool
	}{
		{
			name: "metadata OriginalTempo takes priority",
			args: args{
				meta:          RexMetadata{OriginalTempo: 120.0, Tempo: 140.0},
				sourceName:    "test.rx2",
				tempoOverride: 130,
			},
			wantBPM:      120.0,
			wantFromMeta: true,
		},
		{
			name: "metadata Tempo fallback",
			args: args{
				meta:          RexMetadata{Tempo: 140.0},
				sourceName:    "test.rx2",
				tempoOverride: 0,
			},
			wantBPM:      140.0,
			wantFromMeta: true,
		},
		{
			name: "filename BPM when no metadata",
			args: args{
				meta:          RexMetadata{},
				sourceName:    "/some/path/120Stereo.rx2",
				tempoOverride: 0,
			},
			wantBPM:      120.0,
			wantFromMeta: false,
		},
		{
			name: "filename _NNNbpm pattern",
			args: args{
				meta:          RexMetadata{},
				sourceName:    "/some/path/SM101_brk_Bongo Bar_110bpm.xrni",
				tempoOverride: 0,
			},
			wantBPM:      110.0,
			wantFromMeta: false,
		},
		{
			name: "--tempo override when no metadata or filename BPM",
			args: args{
				meta:          RexMetadata{},
				sourceName:    "unknown.wav",
				tempoOverride: 130,
			},
			wantBPM:      130.0,
			wantFromMeta: false,
		},
		{
			name: "no BPM available",
			args: args{
				meta:          RexMetadata{},
				sourceName:    "unknown.wav",
				tempoOverride: 0,
			},
			wantBPM:      0,
			wantFromMeta: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBPM, gotFromMeta := resolveBPM(tt.args.meta, tt.args.sourceName, tt.args.tempoOverride)
			if math.Abs(gotBPM-tt.wantBPM) > 0.001 {
				t.Errorf("resolveBPM() got BPM = %v, want %v", gotBPM, tt.wantBPM)
			}
			if gotFromMeta != tt.wantFromMeta {
				t.Errorf("resolveBPM() got fromMeta = %v, want %v", gotFromMeta, tt.wantFromMeta)
			}
		})
	}
}

func TestFormatBPMPrefix_NameLimitInteraction(t *testing.T) {
	prefix := formatBPMPrefix(128)
	if prefix != "128-" {
		t.Fatalf("formatBPMPrefix(128) = %q, want %q", prefix, "128-")
	}
	// The prefix is always 4 chars for 3-digit BPM: "NNN-"
	if len("128-") != 4 {
		t.Errorf("expected prefix length 4, got %d", len("128-"))
	}
	// 2-digit BPM prefix is shorter
	prefix90 := formatBPMPrefix(90)
	if len(prefix90) != 3 {
		t.Errorf("expected 2-digit prefix length 3, got %d", len(prefix90))
	}
}
