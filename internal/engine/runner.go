package engine

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type deviceSpec struct {
	maxSlices int
}

var deviceMaxSlices = map[string]int{
	"wav":     0, // unlimited
	"pti":     0, // single instrument, unlimited slices via playback modes
	"ot":      64,
	"aif":     0,
	"aif-op1": 24,
	"xy":      24,
	"el":      64,
	"d2pst":   64,
	"xrni":    128,
	"adv":     0,
	"als":     0,
	"adg":     128,
}

func ExecuteConversionPipeline(cfg PipelineConfig) error {
	return runPipeline(cfg)
}

func runPipeline(cfg PipelineConfig) error {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, os.Stdin); err != nil {
			return fmt.Errorf("failed reading stream from stdin buffer: %w", err)
		}
		return processFileBuffer(buf.Bytes(), "stdin", cfg)
	}

	var targets []string
	if cfg.InputDir != "" {
		var err error
		targets, err = scanDirectory(cfg.InputDir, cfg.Recursive)
		if err != nil {
			return err
		}
	} else {
		targets = cfg.InputFiles
	}

	if len(targets) == 0 {
		return nil
	}

	if cfg.LibraryPath != "" {
		SetLibraryPath(cfg.LibraryPath)
	}

	numWorkers := runtime.NumCPU()
	guard := make(chan struct{}, numWorkers)
	errCh := make(chan error, len(targets))

	for _, target := range targets {
		go func(t string) {
			guard <- struct{}{}
			defer func() { <-guard }()

			data, err := os.ReadFile(t)
			if err != nil {
				errCh <- fmt.Errorf("skipping unreadable target %s: %v", t, err)
				return
			}

			err = processFileBuffer(data, t, cfg)

			errCh <- err
		}(target)
	}

	for range targets {
		if err := <-errCh; err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}

	return nil
}

func processFileBuffer(fileData []byte, sourcePath string, cfg PipelineConfig) error {
	ext := strings.ToLower(filepath.Ext(sourcePath))

	var slices []SliceExtraction
	var err error

	switch ext {
	case ".rex", ".rx2", ".rcy":
		if !HasREX() {
			return fmt.Errorf("REX format not supported on this platform (REX SDK is proprietary, macOS/Windows only)")
		}
		sdkTempo := 0
		if cfg.Tempo > 0 {
			sdkTempo = cfg.Tempo * 1000
		}
		if cfg.NoSlices {
			loop, loopErr := RenderLoopPreview(fileData, cfg.SampleRate, sdkTempo)
			if loopErr != nil {
				return fmt.Errorf("loop render failed: %w", loopErr)
			}
			loop.CuePoints = nil
			slices = []SliceExtraction{*loop}
		} else {
			slices, err = RenderSlicesPreview(fileData, cfg.SampleRate, sdkTempo)
			if err != nil {
				return fmt.Errorf("slices render failed: %w", err)
			}
		}

	case ".ot":
		wavData, wavErr := findCompanionWAV(sourcePath)
		if wavErr != nil {
			return wavErr
		}
		slices, err = ReadOTWithWAV(fileData, wavData, cfg.SampleRate)
		if err != nil {
			return fmt.Errorf("ot read failed: %w", err)
		}

	default:
		reader := DetectReader(ext)
		if reader == nil {
			return fmt.Errorf("unsupported input format: %s", ext)
		}
		slices, err = reader.Read(fileData, cfg.SampleRate)
		if err != nil {
			return fmt.Errorf("read %s failed: %w", ext, err)
		}
	}

	if len(slices) == 0 {
		return nil
	}

	channels := slices[0].Metadata.Channels
	applyMono := cfg.Mono || cfg.Format == "pti"

	if applyMono && channels > 1 {
		for i := range slices {
			mono, err := DownmixToMono(slices[i].Interleaved, channels, cfg.MonoMode)
			if err != nil {
				return err
			}
			slices[i].Interleaved = mono
			slices[i].TotalFrames = len(mono)
		}
		slices[0].Metadata.Channels = 1
	}

	var chunks []SliceExtraction
	sliceLimit := cfg.SliceLimit
	maxSlices := deviceMaxSlices[cfg.Format]
	if maxSlices > 0 {
		if sliceLimit == 0 || sliceLimit > maxSlices {
			sliceLimit = maxSlices
		}
	}
	if sliceLimit > 0 {
		chunks = groupSlices(slices, sliceLimit, cfg.NormalizeSplits)
	} else {
		chunks = buildSingleOutput(slices)
	}

	// Apply format-specific forced specs
	for i := range chunks {
		switch cfg.Format {
		case "pti":
			if err := ForcePTISpec(&chunks[i]); err != nil {
				return err
			}
		case "ot":
			if err := ForceOTSpec(&chunks[i]); err != nil {
				return err
			}
		case "aif", "aif-op1":
			if err := Force44100Spec(&chunks[i]); err != nil {
				return err
			}
		case "d2pst":
			if err := Force48kSpec(&chunks[i]); err != nil {
				return err
			}
		}
	}

	totalFiles := len(chunks)
	nameLimit := fileNameLimit(cfg.Format)

	var wg sync.WaitGroup
	errCh := make(chan error, totalFiles)

	for idx, c := range chunks {
		wg.Add(1)
		go func(idx int, c SliceExtraction) {
			defer wg.Done()

			suffix := splitSuffix(idx, totalFiles, cfg.Format, nameLimit)
			baseName := outputBaseName(sourcePath, cfg, suffix, cfg.Format)

			if err := writeOutputFiles(baseName, &c, cfg, idx, totalFiles); err != nil {
				errCh <- fmt.Errorf("failed encoding for %s: %w", baseName, err)
				return
			}

			if !cfg.Quiet {
				fmt.Printf("Converting: %s -> %s | Slices: %d | Channels: %d | Rate: %dHz | Tempo: %.1f BPM\n",
					filepath.Base(sourcePath), filepath.Base(baseName), len(c.CuePoints),
					c.Metadata.Channels, c.Metadata.SampleRate, c.Metadata.OriginalTempo)
			}
		}(idx, c)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		return err
	}

	return nil
}

func writeOutputFiles(basePath string, extraction *SliceExtraction, cfg PipelineConfig, idx, totalFiles int) error {
	switch cfg.Format {
	case "wav":
		path := basePath + ".wav"
		outDir := filepath.Dir(path)
		if outDir != "." && outDir != "" {
			_ = os.MkdirAll(outDir, 0o755)
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return EncodeWavContainer(f, extraction, cfg.BitRate)

	case "pti":
		path := basePath + ".pti"
		outDir := filepath.Dir(path)
		if outDir != "." && outDir != "" {
			_ = os.MkdirAll(outDir, 0o755)
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return EncodePTI(f, extraction)

	case "ot":
		wavPath := basePath + ".wav"
		otPath := basePath + ".ot"
		outDir := filepath.Dir(wavPath)
		if outDir != "." && outDir != "" {
			_ = os.MkdirAll(outDir, 0o755)
		}
		fw, err := os.Create(wavPath)
		if err != nil {
			return err
		}
		if err := EncodeWavContainer(fw, extraction, cfg.BitRate); err != nil {
			fw.Close()
			return err
		}
		fw.Close()

		bpm := extraction.Metadata.OriginalTempo
		if bpm == 0 {
			bpm = extraction.Metadata.Tempo
		}
		fo, err := os.Create(otPath)
		if err != nil {
			return err
		}
		defer fo.Close()
		return EncodeOT(fo, extraction, bpm)

	case "aif":
		path := basePath + ".aif"
		outDir := filepath.Dir(path)
		if outDir != "." && outDir != "" {
			_ = os.MkdirAll(outDir, 0o755)
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return EncodeAIFF(f, extraction)

	case "aif-op1":
		path := basePath + ".aif"
		outDir := filepath.Dir(path)
		if outDir != "." && outDir != "" {
			_ = os.MkdirAll(outDir, 0o755)
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return EncodeOP1AIF(f, extraction)

	case "xy":
		path := basePath + ".preset.zip"
		outDir := filepath.Dir(path)
		if outDir != "." && outDir != "" {
			_ = os.MkdirAll(outDir, 0o755)
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return EncodeXYPreset(f, extraction)

	case "el":
		wavPath := basePath + ".wav"
		txtPath := basePath + "_slices.txt"
		outDir := filepath.Dir(wavPath)
		if outDir != "." && outDir != "" {
			_ = os.MkdirAll(outDir, 0o755)
		}
		fw, err := os.Create(wavPath)
		if err != nil {
			return err
		}
		if err := EncodeWavContainer(fw, extraction, cfg.BitRate); err != nil {
			fw.Close()
			return err
		}
		fw.Close()

		ft, err := os.Create(txtPath)
		if err != nil {
			return err
		}
		defer ft.Close()
		return EncodeEL(ft, extraction)

	case "d2pst":
		path := basePath + ".dt2pst"
		outDir := filepath.Dir(path)
		if outDir != "." && outDir != "" {
			_ = os.MkdirAll(outDir, 0o755)
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		name := filepath.Base(basePath)
		return EncodeDT2Preset(f, extraction, name)

	case "xrni":
		path := basePath + ".xrni"
		outDir := filepath.Dir(path)
		if outDir != "." && outDir != "" {
			_ = os.MkdirAll(outDir, 0o755)
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		name := filepath.Base(basePath)
		return EncodeXRNI(f, extraction, name)

	case "adv", "als":
		isALS := cfg.Format == "als"

		baseDir := filepath.Dir(basePath)
		baseName := filepath.Base(basePath)

		wavDir := filepath.Join(baseDir, "Samples", "Imported")
		_ = os.MkdirAll(wavDir, 0o755)

		wavName := baseName + ".wav"
		relPath := filepath.ToSlash(filepath.Join("Samples", "Imported", wavName))

		wavPath := filepath.Join(wavDir, wavName)
		fw, err := os.Create(wavPath)
		if err != nil {
			return err
		}
		if err := EncodeWavContainer(fw, extraction, cfg.BitRate); err != nil {
			fw.Close()
			return err
		}
		fw.Close()

		metaPath := basePath + ".adv"
		if isALS {
			metaPath = basePath + ".als"
		}
		fm, err := os.Create(metaPath)
		if err != nil {
			return err
		}
		defer fm.Close()

		if isALS {
			return EncodeSimplerALS(fm, extraction, relPath)
		}
		return EncodeSimplerADV(fm, extraction, relPath)

	case "adg":
		slices := splitExtractionIntoSlices(extraction)

		baseDir := filepath.Dir(basePath)
		baseName := filepath.Base(basePath)
		baseNote := 36

		wavDir := filepath.Join(baseDir, "Samples", "Imported")
		_ = os.MkdirAll(wavDir, 0o755)

		samplePaths := make([]string, len(slices))
		for i, s := range slices {
			padName := fmt.Sprintf("%s_pad_%02d", baseName, i+1)
			wavName := padName + ".wav"
			relPath := filepath.ToSlash(filepath.Join("Samples", "Imported", wavName))
			samplePaths[i] = relPath

			wavPath := filepath.Join(wavDir, wavName)
			fw, err := os.Create(wavPath)
			if err != nil {
				return err
			}
			if err := EncodeWavContainer(fw, &s, cfg.BitRate); err != nil {
				fw.Close()
				return err
			}
			fw.Close()
		}

		path := basePath + ".adg"
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return EncodeDrumRackADG(f, slices, samplePaths, baseNote)

	default:
		path := basePath + ".wav"
		outDir := filepath.Dir(path)
		if outDir != "." && outDir != "" {
			_ = os.MkdirAll(outDir, 0o755)
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return EncodeWavContainer(f, extraction, cfg.BitRate)
	}
}

func fileNameLimit(format string) int {
	switch format {
	case "d2pst":
		return 12
	case "aif", "aif-op1":
		return 8
	case "pti", "xrni":
		return 31
	default:
		return 255
	}
}

func sanitizeName(name string, limit int) string {
	sanitized := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == ' ' || c == '-' || c == '_' || c == '+' || c == '@' {
			sanitized = append(sanitized, c)
		} else {
			sanitized = append(sanitized, '_')
		}
	}
	result := strings.TrimSpace(string(sanitized))
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	if result == "" {
		result = "output"
	}
	return result
}

func splitSuffix(idx, totalFiles int, format string, _ int) string {
	if totalFiles <= 1 {
		return ""
	}
	switch format {
	case "d2pst", "aif-op1":
		return fmt.Sprintf("_%d", idx+1)
	default:
		return fmt.Sprintf("_%02d", idx+1)
	}
}

func outputBaseName(sourcePath string, cfg PipelineConfig, suffix, format string) string {
	if cfg.OutputFile != "" && cfg.SliceLimit <= 0 {
		// User explicitly specified the output path AND no chunking.
		// Use it as-is (no sanitization, no suffix). Strip the extension;
		// writeOutputFiles adds it back.
		return strings.TrimSuffix(cfg.OutputFile, filepath.Ext(cfg.OutputFile))
	}

	if cfg.OutputFile != "" && cfg.SliceLimit > 0 {
		// Chunking requested with -o: treat the path as a base name pattern
		// and append chunk suffixes (_01, _02, ...). Strip extension so
		// writeOutputFiles can add format-specific extension.
		baseName := strings.TrimSuffix(cfg.OutputFile, filepath.Ext(cfg.OutputFile))
		nameLimit := fileNameLimit(format)
		if nameLimit > 0 && len(baseName)+len(suffix) > nameLimit {
			baseName = baseName[:nameLimit-len(suffix)]
		}
		return baseName + suffix
	}

	if sourcePath == "stdin" {
		baseName := "output"
		nameLimit := fileNameLimit(format)
		baseName = sanitizeName(baseName+suffix, nameLimit)
		if cfg.OutputDir != "" {
			return filepath.Join(cfg.OutputDir, baseName)
		}
		return baseName
	}

	baseName := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))

	nameLimit := fileNameLimit(format)
	suffixed := sanitizeName(baseName, nameLimit-len(suffix))
	suffixed += suffix

	if cfg.OutputDir != "" {
		if cfg.Preserve && cfg.InputDir != "" {
			relDir, err := filepath.Rel(cfg.InputDir, filepath.Dir(sourcePath))
			if err == nil && relDir != "." && relDir != "" {
				return filepath.Join(cfg.OutputDir, relDir, suffixed)
			}
		}
		return filepath.Join(cfg.OutputDir, suffixed)
	}
	return suffixed
}

func groupSlices(slices []SliceExtraction, maxSlices int, normalize bool) []SliceExtraction {
	total := len(slices)
	if total == 0 || maxSlices <= 0 {
		return buildSingleOutput(slices)
	}

	var groupSizes []int
	if normalize && maxSlices > 1 && total > maxSlices {
		numFiles := int(math.Ceil(float64(total) / float64(maxSlices)))
		baseSize := total / numFiles
		remainder := total % numFiles
		for i := 0; i < numFiles; i++ {
			size := baseSize
			if i < remainder {
				size++
			}
			groupSizes = append(groupSizes, size)
		}
	} else {
		remaining := total
		for remaining > 0 {
			take := maxSlices
			if remaining < maxSlices {
				take = remaining
			}
			groupSizes = append(groupSizes, take)
			remaining -= take
		}
	}

	ch := slices[0].Metadata.Channels
	var results []SliceExtraction
	sliceIdx := 0

	for _, size := range groupSizes {
		if sliceIdx+size > total {
			size = total - sliceIdx
		}

		totalFrames := 0
		for j := 0; j < size; j++ {
			totalFrames += slices[sliceIdx+j].TotalFrames
		}

		pcm := make([]float32, totalFrames*ch)
		offset := 0
		for j := 0; j < size; j++ {
			s := slices[sliceIdx+j]
			copy(pcm[offset:offset+len(s.Interleaved)], s.Interleaved)
			offset += len(s.Interleaved)
		}

		cues := make([]WavCueMarker, size)
		frameOffset := 0
		for j := 0; j < size; j++ {
			cues[j] = WavCueMarker{
				SliceID:  j,
				Position: uint32(frameOffset),
				Label:    fmt.Sprintf("Slice %02d", sliceIdx+j+1),
			}
			frameOffset += slices[sliceIdx+j].TotalFrames
		}

		results = append(results, SliceExtraction{
			Metadata:    slices[0].Metadata,
			CuePoints:   cues,
			Interleaved: pcm,
			TotalFrames: totalFrames,
		})

		sliceIdx += size
	}

	return results
}

func buildSingleOutput(slices []SliceExtraction) []SliceExtraction {
	if len(slices) == 0 {
		return nil
	}
	if len(slices) == 1 {
		return slices
	}

	ch := slices[0].Metadata.Channels
	totalFrames := 0
	for _, s := range slices {
		totalFrames += s.TotalFrames
	}

	pcm := make([]float32, totalFrames*ch)
	cues := make([]WavCueMarker, len(slices))
	frameOffset := 0

	for i, s := range slices {
		copy(pcm[frameOffset*ch:], s.Interleaved)
		cues[i] = WavCueMarker{
			SliceID:  i,
			Position: uint32(frameOffset),
			Label:    fmt.Sprintf("Slice %02d", i+1),
		}
		frameOffset += s.TotalFrames
	}

	return []SliceExtraction{
		{
			Metadata:    slices[0].Metadata,
			CuePoints:   cues,
			Interleaved: pcm,
			TotalFrames: totalFrames,
		},
	}
}

var inputExtensions = []string{".rex", ".rx2", ".rcy", ".xrni", ".als", ".adv", ".adg", ".wav", ".aif", ".aiff", ".pti", ".ot", ".xy", ".d2pst"}

func isSupportedInput(ext string) bool {
	ext = strings.ToLower(ext)
	for _, e := range inputExtensions {
		if ext == e {
			return true
		}
	}
	return false
}

func scanDirectory(dir string, recursive bool) ([]string, error) {
	var matches []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !recursive && info.IsDir() && path != dir {
			return filepath.SkipDir
		}
		if !info.IsDir() && isSupportedInput(filepath.Ext(path)) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}
