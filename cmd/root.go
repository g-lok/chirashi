package cmd

import (
	"fmt"
	"os"

	"github.com/g-lok/chirashi/internal/engine"
	"github.com/spf13/cobra"
)

// Set at build time via -ldflags -X
var version = "dev"

var (
	inputFiles      []string
	inputDir        string
	outputFile      string
	outputDir       string
	recursive       bool
	bitRate         int
	sampleRate      int
	mono            bool
	sliceLimit      int
	normalizeSplits bool
	tempo           int
	quiet           bool
	preserve        bool
	verbose         bool
	outputFormat    string
	noSlices        bool
	monoMode        string
	libraryPath     string
	inputFormat     string
	samplePathMode  string
)

var rootCmd = &cobra.Command{
	Use:     "chirashi [INPUT_FILES...]",
	Short:   "Cross-format sliced instrument converter for hardware samplers and DAWs",
	Version: version,
	Long: `Converts sliced instruments (REX, RX2, RCY, XRNI, ADV, ALS, ADG) into hardware sampler
formats: WAV, PTI (Polyend Tracker), OT (Octatrack), OP-1 AIFF, XY preset (OP-XY),
Elektron multi-sample text, DT2 preset (Digitakt II). Supports concurrency, resampling,
mono downmix, and slice grouping.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		hasFlagInputs := len(inputFiles) > 0
		hasPositionalInputs := len(args) > 0
		hasDirInput := inputDir != ""

		stat, _ := os.Stdin.Stat()
		hasStdin := (stat.Mode() & os.ModeCharDevice) == 0

		if hasDirInput && (hasFlagInputs || hasPositionalInputs || hasStdin) {
			return fmt.Errorf("error: cannot mix --input-dir with explicit file targets or stdin")
		}

		if !hasDirInput && !hasFlagInputs && !hasPositionalInputs && !hasStdin {
			return fmt.Errorf("error: missing input targets. Provide positional args, --input-file, or --input-dir")
		}

		if outputFile != "" && outputDir != "" {
			return fmt.Errorf("error: cannot combine --output-file with --output-dir")
		}

		if outputFile != "" {
			totalInputs := len(inputFiles) + len(args)
			if totalInputs > 1 || hasDirInput {
				return fmt.Errorf("error: --output-file cannot be used with multiple input files; use --output-dir (-e) instead")
			}
		}

		if recursive && !hasDirInput {
			return fmt.Errorf("error: --recursive requires --input-dir")
		}

		if preserve && !hasDirInput {
			return fmt.Errorf("error: --preserve requires --input-dir")
		}

		if normalizeSplits && sliceLimit <= 0 {
			return fmt.Errorf("error: --normalize-splits requires --slice-limit")
		}

		if bitRate != 0 && bitRate != 8 && bitRate != 16 && bitRate != 24 {
			return fmt.Errorf("error: unsupported bit-rate %d. Supported: 8, 16, or 24", bitRate)
		}

		if hasPositionalInputs {
			inputFiles = append(inputFiles, args...)
		}

		if engine.HasREX() {
			if err := engine.InitEngine(verbose); err != nil {
				return fmt.Errorf("failed to initialize REX SDK: %w", err)
			}
			defer engine.CloseEngine()
		} else if verbose {
			fmt.Println("REX SDK not available on this platform — REX/RX2/RCY input disabled")
		}

		pipelineConfig := engine.PipelineConfig{
			InputFiles:      inputFiles,
			InputDir:        inputDir,
			OutputFile:      outputFile,
			OutputDir:       outputDir,
			Recursive:       recursive,
			BitRate:         bitRate,
			SampleRate:      sampleRate,
			Mono:            mono,
			SliceLimit:      sliceLimit,
			NormalizeSplits: normalizeSplits,
			Tempo:           tempo,
			Quiet:           quiet,
			Preserve:        preserve,
			Verbose:         verbose,
			Format:          outputFormat,
			NoSlices:        noSlices,
			MonoMode:        monoMode,
			LibraryPath:     libraryPath,
			InputFormat:     inputFormat,
			SamplePathMode:  samplePathMode,
		}

		return engine.ExecuteConversionPipeline(pipelineConfig)
	},
}

func Execute() {
	if rootCmd == nil {
		return
	}
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringSliceVarP(&inputFiles, "input-file", "i", []string{}, "Target ReCycle input file(s)")
	rootCmd.Flags().StringVarP(&inputDir, "input-dir", "d", "", "Scan directory for .rex/.rx2 files")
	rootCmd.Flags().StringVarP(&outputFile, "output-file", "o", "", "Output WAV path (single input only)")
	rootCmd.Flags().StringVarP(&outputDir, "output-dir", "e", "", "Output directory for batch conversions")
	rootCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recurse subdirectories (requires --input-dir)")
	rootCmd.Flags().BoolVarP(&preserve, "preserve", "p", false, "Preserve directory structure in output (requires --input-dir)")
	rootCmd.Flags().IntVarP(&bitRate, "bit-rate", "b", 0, "Bit depth: 8, 16, or 24")
	rootCmd.Flags().IntVarP(&sampleRate, "sample-rate", "s", 0, "Output sample rate in Hz")
	rootCmd.Flags().BoolVarP(&mono, "mono", "m", false, "Downmix to mono")
	rootCmd.Flags().IntVarP(&tempo, "tempo", "t", 0, "Override loop tempo in BPM (default: original)")
	rootCmd.Flags().IntVarP(&sliceLimit, "slice-limit", "l", 0, "Max slices per output file")
	rootCmd.Flags().BoolVar(&normalizeSplits, "normalize-splits", false, "Balance slices evenly across splits")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress output")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Debug output (Zig struct diagnostics)")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "f", "wav", "Output format: wav, pti, ot, aif, aif-op1, xy, el, d2pst, xrni, adv, als, adg")
	rootCmd.Flags().BoolVarP(&noSlices, "no-slices", "n", false, "Ignore REX cue points, render plain unsliced output")
	rootCmd.Flags().StringVar(&monoMode, "mono-mode", "sum", "Mono downmix strategy: sum, left, right, difference, dual-detect")
	rootCmd.Flags().StringVar(&libraryPath, "library-path", "", "Ableton User Library path for sample resolution")
	rootCmd.Flags().StringVar(&inputFormat, "input-format", "", "Force input format (auto-detect by extension if unset)")
	rootCmd.Flags().StringVar(&samplePathMode, "sample-path-mode", "relative", "Sample path style in XML output: relative, absolute, library")
}
