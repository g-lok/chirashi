package engine

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var printMu sync.Mutex

type AudioProperties struct {
	Channels        int     `json:"channels" yaml:"channels"`
	SampleRate      int     `json:"sample_rate_hz" yaml:"sample_rate_hz"`
	BitDepth        int     `json:"bit_depth" yaml:"bit_depth"`
	TotalFrames     int     `json:"total_frames" yaml:"total_frames"`
	DurationSeconds float64 `json:"duration_seconds" yaml:"duration_seconds"`
	Duration        string  `json:"duration" yaml:"duration"`
}

type TempoProperties struct {
	BPM           float64 `json:"bpm" yaml:"bpm"`
	OriginalBPM   float64 `json:"original_bpm,omitempty" yaml:"original_bpm,omitempty"`
	TimeSignature string  `json:"time_signature,omitempty" yaml:"time_signature,omitempty"`
	PPQLength     int     `json:"ppq_length,omitempty" yaml:"ppq_length,omitempty"`
}

type SliceRegion struct {
	SliceID      int    `json:"slice_id" yaml:"slice_id"`
	StartFrame   int    `json:"start_frame" yaml:"start_frame"`
	EndFrame     int    `json:"end_frame" yaml:"end_frame"`
	LengthFrames int    `json:"length_frames" yaml:"length_frames"`
	TimeRange    string `json:"time_range" yaml:"time_range"`
}

type SliceSummary struct {
	TotalCount int           `json:"total_count" yaml:"total_count"`
	Type       string        `json:"type" yaml:"type"`
	List       []SliceRegion `json:"list,omitempty" yaml:"list,omitempty"`
}

type FileDiagnostic struct {
	FilePath            string                 `json:"file_path" yaml:"file_path"`
	Format              string                 `json:"format" yaml:"format"`
	FileSizeBytes       int64                  `json:"file_size_bytes" yaml:"file_size_bytes"`
	Audio               AudioProperties        `json:"audio" yaml:"audio"`
	Tempo               TempoProperties        `json:"tempo" yaml:"tempo"`
	SliceRegions        SliceSummary           `json:"slice_regions" yaml:"slice_regions"`
	FormatDetails       map[string]interface{} `json:"format_details,omitempty" yaml:"format_details,omitempty"`
	QuirksAndLimit      []string               `json:"quirks_and_limitations,omitempty" yaml:"quirks_and_limitations,omitempty"`
	SampleReferences    []map[string]interface{} `json:"sample_references,omitempty" yaml:"sample_references,omitempty"`
}

type SplitPlan struct {
	FileIndex  int    `json:"file_index" yaml:"file_index"`
	OutputPath string `json:"output_path" yaml:"output_path"`
	SliceCount int    `json:"slice_count" yaml:"slice_count"`
	SliceRange string `json:"slice_range" yaml:"slice_range"`
}

type OutputPlan struct {
	TargetFormat    string      `json:"target_format" yaml:"target_format"`
	OutputChannels  int         `json:"output_channels" yaml:"output_channels"`
	ChannelAction   string      `json:"channel_action" yaml:"channel_action"`
	OutputSampleRate int         `json:"output_sample_rate_hz" yaml:"output_sample_rate_hz"`
	SampleRateAction string      `json:"sample_rate_action" yaml:"sample_rate_action"`
	OutputBitDepth  int         `json:"output_bit_depth" yaml:"output_bit_depth"`
	TotalOutputFiles int        `json:"total_output_files" yaml:"total_output_files"`
	Splits          []SplitPlan `json:"splits,omitempty" yaml:"splits,omitempty"`
	Warnings        []string    `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type DryRunReport struct {
	Input      FileDiagnostic `json:"input" yaml:"input"`
	OutputPlan OutputPlan     `json:"output_plan" yaml:"output_plan"`
}

func FormatTimestamp(frameOffset, sampleRate int) string {
	if sampleRate <= 0 {
		sampleRate = 44100
	}
	totalMs := int64(float64(frameOffset) * 1000.0 / float64(sampleRate))
	ms := totalMs % 1000
	totalSec := totalMs / 1000
	sec := totalSec % 60
	totalMin := totalSec / 60
	min := totalMin % 60
	hour := totalMin / 60

	return fmt.Sprintf("%02d:%02d:%02d:%03d", hour, min, sec, ms)
}

func BuildFileDiagnostic(fileData []byte, sourcePath string, cfg PipelineConfig, extraction []SliceExtraction) FileDiagnostic {
	ext := strings.ToLower(filepath.Ext(sourcePath))
	
	diag := FileDiagnostic{
		FilePath:      sourcePath,
		Format:        detectFormatName(ext, fileData),
		FileSizeBytes: int64(len(fileData)),
		FormatDetails: make(map[string]interface{}),
	}

	if len(extraction) == 0 {
		return diag
	}

	// Audio Properties
	channels := 0
	sampleRate := 0
	bitDepth := 0
	totalFrames := 0
	for _, s := range extraction {
		totalFrames += s.TotalFrames
		if channels == 0 {
			channels = s.Metadata.Channels
			sampleRate = s.Metadata.SampleRate
			bitDepth = s.Metadata.BitDepth
		}
	}

	durationSec := float64(totalFrames) / float64(sampleRate)
	diag.Audio = AudioProperties{
		Channels:        channels,
		SampleRate:      sampleRate,
		BitDepth:        bitDepth,
		TotalFrames:     totalFrames,
		DurationSeconds: durationSec,
		Duration:        FormatTimestamp(totalFrames, sampleRate),
	}

	// Tempo
	diag.Tempo = TempoProperties{
		BPM:           extraction[0].Metadata.Tempo,
		OriginalBPM:   extraction[0].Metadata.OriginalTempo,
		PPQLength:     extraction[0].Metadata.PPQLength,
	}
	if extraction[0].Metadata.TimeSignNom > 0 {
		diag.Tempo.TimeSignature = fmt.Sprintf("%d/%d", extraction[0].Metadata.TimeSignNom, extraction[0].Metadata.TimeSignDenom)
	}

	// Slices
	diag.SliceRegions.TotalCount = len(extraction[0].CuePoints)
	diag.SliceRegions.Type = "embedded_single_file"
	
	// If multi-slice extraction, it might be a container
	if len(extraction) > 1 {
		diag.SliceRegions.TotalCount = len(extraction)
		diag.SliceRegions.Type = "fragmented_slices"
	}

	// Build slice list
	if len(extraction) == 1 {
		m := extraction[0]
		for i, cp := range m.CuePoints {
			start := int(cp.Position)
			end := m.TotalFrames
			if i+1 < len(m.CuePoints) {
				end = int(m.CuePoints[i+1].Position)
			}
			
			diag.SliceRegions.List = append(diag.SliceRegions.List, SliceRegion{
				SliceID:      i + 1,
				StartFrame:   start,
				EndFrame:     end,
				LengthFrames: end - start,
				TimeRange:    fmt.Sprintf("%s - %s", FormatTimestamp(start, m.Metadata.SampleRate), FormatTimestamp(end, m.Metadata.SampleRate)),
			})
		}
	} else {
		offset := 0
		sr := extraction[0].Metadata.SampleRate
		for i, s := range extraction {
			end := offset + s.TotalFrames
			diag.SliceRegions.List = append(diag.SliceRegions.List, SliceRegion{
				SliceID:      i + 1,
				StartFrame:   offset,
				EndFrame:     end,
				LengthFrames: s.TotalFrames,
				TimeRange:    fmt.Sprintf("%s - %s", FormatTimestamp(offset, sr), FormatTimestamp(end, sr)),
			})
			offset = end
		}
	}

	// Format-specific details
	reader := DetectReader(ext)
	if reader != nil {
		diag.FormatDetails = reader.Inspect(fileData)
		// Extract sample references for top-level display if present
		if refs, ok := diag.FormatDetails["sample_references"]; ok {
			if strRefs, ok := refs.([]string); ok {
				for _, r := range strRefs {
					diag.SampleReferences = append(diag.SampleReferences, map[string]interface{}{
						"path": r,
					})
				}
				delete(diag.FormatDetails, "sample_references")
			}
		}
	}

	// Override/Add specifics for REX formats which are handled specially in runner
	switch ext {
	case ".rx2", ".rex", ".rcy":
		diag.FormatDetails["ppq_length"] = extraction[0].Metadata.PPQLength
		diag.FormatDetails["transient_sensitivity"] = extraction[0].Metadata.RexSensitivity
	}

	if extraction[0].Metadata.CreatorName != "" {
		diag.FormatDetails["creator_name"] = extraction[0].Metadata.CreatorName
	}
	if extraction[0].Metadata.Copyright != "" {
		diag.FormatDetails["copyright"] = extraction[0].Metadata.Copyright
	}

	// Add quirks/limitations
	if cfg.Format == "pti" && diag.SliceRegions.TotalCount > 48 {
		diag.QuirksAndLimit = append(diag.QuirksAndLimit, "PTI hardware limit is 48 slices; output will be split")
	}
	if cfg.Format == "aif-op1" && diag.SliceRegions.TotalCount > 24 {
		diag.QuirksAndLimit = append(diag.QuirksAndLimit, "OP-1 drum kit limit is 24 slices; output will be split")
	}

	return diag
}

func detectFormatName(ext string, data []byte) string {
	switch ext {
	case ".rx2":
		return "REX2"
	case ".rex":
		return "REX1"
	case ".rcy":
		return "ReCycle Document"
	case ".wav":
		return "WAV"
	case ".aif", ".aiff":
		return "AIFF"
	case ".caf":
		return "Apple Loop CAF"
	case ".xrni":
		return "Renoise Instrument"
	case ".adv":
		return "Ableton Simpler ADV"
	case ".als":
		return "Ableton Live Set ALS"
	case ".adg":
		return "Ableton Drum Rack ADG"
	case ".dt2pst":
		return "Digitakt II Preset"
	case ".pti":
		return "Polyend Tracker Sample"
	case ".ot":
		return "Octatrack Sample"
	case ".multisample":
		return "Bitwig Multisample"
	case ".pgm":
		return "Akai Legacy MPC Program"
	case ".sxt", ".nnxt":
		return "Reason NN-XT Patch"
	case ".mod":
		return "ProTracker Module"
	case ".8svx", ".16sv":
		return "Amiga IFF Audio"
	case ".txw", ".w01", ".w02", ".w03":
		return "Yamaha TX16W Wave"
	}
	return "Unknown"
}

func PrintReport(report interface{}, jsonMode bool) {
	printMu.Lock()
	defer printMu.Unlock()

	if jsonMode {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Println("---")
	// Manual YAML-like formatting for better readability
	printYAML(report, 0)
}

func printYAML(v interface{}, indent int) {
	prefix := strings.Repeat("  ", indent)
	
	switch val := v.(type) {
	case DryRunReport:
		fmt.Println("input:")
		printYAML(val.Input, indent+1)
		fmt.Println("output_plan:")
		printYAML(val.OutputPlan, indent+1)
	case FileDiagnostic:
		fmt.Printf("%sfile_path: %s\n", prefix, val.FilePath)
		fmt.Printf("%sformat: %s\n", prefix, val.Format)
		fmt.Printf("%sfile_size_bytes: %d\n", prefix, val.FileSizeBytes)
		fmt.Printf("%saudio:\n", prefix)
		printYAML(val.Audio, indent+1)
		fmt.Printf("%stempo:\n", prefix)
		printYAML(val.Tempo, indent+1)
		fmt.Printf("%sslice_regions:\n", prefix)
		printYAML(val.SliceRegions, indent+1)
		if len(val.FormatDetails) > 0 {
			fmt.Printf("%sformat_details:\n", prefix)
			printYAML(val.FormatDetails, indent+1)
		}
		if len(val.QuirksAndLimit) > 0 {
			fmt.Printf("%squirks_and_limitations:\n", prefix)
			for _, q := range val.QuirksAndLimit {
				fmt.Printf("%s  - %q\n", prefix, q)
			}
		}
		if len(val.SampleReferences) > 0 {
			fmt.Printf("%ssample_references:\n", prefix)
			for _, ref := range val.SampleReferences {
				fmt.Printf("%s  - path: %v\n", prefix, ref["path"])
				if t, ok := ref["type"]; ok {
					fmt.Printf("%s    type: %v\n", prefix, t)
				}
				if rs, ok := ref["resolution_status"]; ok {
					fmt.Printf("%s    resolution_status: %v\n", prefix, rs)
				}
				if rp, ok := ref["resolved_path"]; ok {
					fmt.Printf("%s    resolved_path: %v\n", prefix, rp)
				}
			}
		}
	case AudioProperties:
		fmt.Printf("%schannels: %d\n", prefix, val.Channels)
		fmt.Printf("%ssample_rate_hz: %d\n", prefix, val.SampleRate)
		fmt.Printf("%sbit_depth: %d\n", prefix, val.BitDepth)
		fmt.Printf("%stotal_frames: %d\n", prefix, val.TotalFrames)
		fmt.Printf("%sduration: %q\n", prefix, val.Duration)
	case TempoProperties:
		fmt.Printf("%sbpm: %.1f\n", prefix, val.BPM)
		if val.OriginalBPM > 0 {
			fmt.Printf("%soriginal_bpm: %.1f\n", prefix, val.OriginalBPM)
		}
		if val.TimeSignature != "" {
			fmt.Printf("%stime_signature: %q\n", prefix, val.TimeSignature)
		}
		if val.PPQLength > 0 {
			fmt.Printf("%sppq_length: %d\n", prefix, val.PPQLength)
		}
	case SliceSummary:
		fmt.Printf("%stotal_count: %d\n", prefix, val.TotalCount)
		fmt.Printf("%stype: %s\n", prefix, val.Type)
		if len(val.List) > 0 {
			fmt.Printf("%slist:\n", prefix)
			for _, s := range val.List {
				fmt.Printf("%s  - slice_id: %d\n", prefix, s.SliceID)
				fmt.Printf("%s    start_frame: %d\n", prefix, s.StartFrame)
				fmt.Printf("%s    end_frame: %d\n", prefix, s.EndFrame)
				fmt.Printf("%s    length_frames: %d\n", prefix, s.LengthFrames)
				fmt.Printf("%s    time_range: %q\n", prefix, s.TimeRange)
			}
		}
	case OutputPlan:
		fmt.Printf("%starget_format: %s\n", prefix, val.TargetFormat)
		fmt.Printf("%soutput_channels: %d\n", prefix, val.OutputChannels)
		fmt.Printf("%schannel_action: %q\n", prefix, val.ChannelAction)
		fmt.Printf("%soutput_sample_rate_hz: %d\n", prefix, val.OutputSampleRate)
		fmt.Printf("%ssample_rate_action: %q\n", prefix, val.SampleRateAction)
		fmt.Printf("%soutput_bit_depth: %d\n", prefix, val.OutputBitDepth)
		fmt.Printf("%stotal_output_files: %d\n", prefix, val.TotalOutputFiles)
		if len(val.Splits) > 0 {
			fmt.Printf("%ssplits:\n", prefix)
			for _, s := range val.Splits {
				fmt.Printf("%s  - file_index: %d\n", prefix, s.FileIndex)
				fmt.Printf("%s    output_path: %s\n", prefix, s.OutputPath)
				fmt.Printf("%s    slice_count: %d\n", prefix, s.SliceCount)
				fmt.Printf("%s    slice_range: %q\n", prefix, s.SliceRange)
			}
		}
		if len(val.Warnings) > 0 {
			fmt.Printf("%swarnings:\n", prefix)
			for _, w := range val.Warnings {
				fmt.Printf("%s  - %q\n", prefix, w)
			}
		}
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s%s: %v\n", prefix, k, val[k])
		}
	}
}

func BuildOutputPlan(sourcePath string, cfg PipelineConfig, chunks []SliceExtraction) OutputPlan {
	plan := OutputPlan{
		TargetFormat:     cfg.Format,
		OutputSampleRate: cfg.SampleRate,
		OutputBitDepth:   cfg.BitRate,
		TotalOutputFiles: len(chunks),
		Warnings:         []string{},
	}

	if len(chunks) == 0 {
		return plan
	}

	first := chunks[0]
	plan.OutputChannels = first.Metadata.Channels
	plan.OutputSampleRate = first.Metadata.SampleRate
	plan.OutputBitDepth = first.Metadata.BitDepth
	if cfg.BitRate > 0 {
		plan.OutputBitDepth = cfg.BitRate
	}

	// Channel action
	if cfg.Mono || cfg.Format == "pti" {
		plan.ChannelAction = "downmix to mono"
		if cfg.MonoMode != "" {
			plan.ChannelAction += fmt.Sprintf(" (%s)", cfg.MonoMode)
		}
	} else {
		plan.ChannelAction = fmt.Sprintf("pass-through (%d channels)", first.Metadata.Channels)
	}

	// Sample rate action
	if cfg.SampleRate > 0 {
		plan.SampleRateAction = fmt.Sprintf("resample to %d Hz", cfg.SampleRate)
	} else {
		plan.SampleRateAction = fmt.Sprintf("pass-through (%d Hz)", first.Metadata.SampleRate)
	}

	// Name resolution for splits
	nameLimit := fileNameLimit(cfg.Format)
	bpmPrefixStr := ""
	if cfg.BpmPrefix {
		bpm, _ := resolveBPM(first.Metadata, sourcePath, cfg.Tempo)
		bpmPrefixStr = formatBPMPrefix(bpm)
	}

	startSlice := 1
	for i, c := range chunks {
		suffix := splitSuffix(i, len(chunks), cfg.Format, nameLimit)
		baseName := outputBaseName(sourcePath, cfg, suffix, cfg.Format, bpmPrefixStr)
		
		ext := "." + cfg.Format
		if cfg.Format == "aif-op1" {
			ext = ".aif"
		} else if cfg.Format == "xy" {
			ext = ".preset.zip"
		} else if cfg.Format == "nnxt" {
			ext = ".sxt"
		} else if cfg.Format == "sp404mk2" {
			ext = ".wav"
		}

		endSlice := startSlice + len(c.CuePoints) - 1
		if len(c.CuePoints) == 0 {
			endSlice = startSlice
		}

		plan.Splits = append(plan.Splits, SplitPlan{
			FileIndex:  i + 1,
			OutputPath: baseName + ext,
			SliceCount: len(c.CuePoints),
			SliceRange: fmt.Sprintf("%d..%d", startSlice, endSlice),
		})
		startSlice = endSlice + 1
	}

	return plan
}
