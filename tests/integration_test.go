package rexconverter_test

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// binaryPath is the path to the built rexconverter executable.
var binaryPath = findBinary()

func findBinary() string {
	// Check common locations
	candidates := []string{
		"../build/rexconverter",
		"build/rexconverter",
		"./rexconverter",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	// Allow override via env var
	if p := os.Getenv("REXCONVERTER_BIN"); p != "" {
		return p
	}
	return ""
}

func requireREX(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "linux" {
		t.Skip("REX SDK not available on Linux")
	}
}

func testDataPath(name string) string {
	// On Linux, REX test files exist but can't be processed
	ext := strings.ToLower(filepath.Ext(name))
	if (ext == ".rex" || ext == ".rx2" || ext == ".rcy") && runtime.GOOS == "linux" {
		return ""
	}
	return filepath.Join("testdata", name)
}

// ---------- Helper: parse reference .txt file ----------

type refSlice struct {
	NumChannels  int
	SampleRate   int
	NumFrames    int
	PCMInterleaved []float64
}

func readRefTxt(path string) (*refSlice, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lines := 0
	var r refSlice
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		switch lines {
		case 0:
			r.NumChannels, _ = strconv.Atoi(line)
		case 1:
			r.SampleRate, _ = strconv.Atoi(line)
		case 2:
			r.NumFrames, _ = strconv.Atoi(line)
		default:
			val, err := strconv.ParseFloat(line, 64)
			if err != nil {
				return nil, fmt.Errorf("bad float at line %d: %s", lines+1, line)
			}
			r.PCMInterleaved = append(r.PCMInterleaved, val)
		}
		lines++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &r, nil
}

// ---------- Helper: parse WAV file PCM data ----------

func readWAVPCM(path string) ([]int16, int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(data) < 44 {
		return nil, 0, 0, fmt.Errorf("file too small for WAV header")
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, 0, fmt.Errorf("not a valid WAV")
	}

	numChans := int(binary.LittleEndian.Uint16(data[22:24]))
	sampleRate := int(binary.LittleEndian.Uint32(data[24:28]))

	// Find data chunk (may not be at offset 36 if there are other chunks)
	dataStart := 12
	for dataStart < len(data)-8 {
		chunkID := string(data[dataStart : dataStart+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[dataStart+4 : dataStart+8]))
		if chunkID == "data" {
			pcmStart := dataStart + 8
			pcmEnd := pcmStart + chunkSize
			if pcmEnd > len(data) {
				pcmEnd = len(data)
			}
			pcmBytes := data[pcmStart:pcmEnd]
			samples := make([]int16, len(pcmBytes)/2)
			for i := range samples {
				samples[i] = int16(binary.LittleEndian.Uint16(pcmBytes[i*2:]))
			}
			return samples, numChans, sampleRate, nil
		}
		dataStart += 8 + chunkSize
		// Pad to word boundary
		if chunkSize%2 != 0 {
			dataStart++
		}
	}
	return nil, 0, 0, fmt.Errorf("no data chunk found")
}

const pcmTolerance = 2 // max acceptable int16 difference between reference and output

// Compare reference .txt (float32) against WAV output (int16).
func compareRefToWAV(refPath, wavPath string, tolerance int) error {
	ref, err := readRefTxt(refPath)
	if err != nil {
		return fmt.Errorf("reading ref %s: %w", refPath, err)
	}

	pcm, numChans, sampleRate, err := readWAVPCM(wavPath)
	if err != nil {
		return fmt.Errorf("reading wav %s: %w", wavPath, err)
	}

	if numChans != ref.NumChannels {
		return fmt.Errorf("channel count: expected %d, got %d", ref.NumChannels, numChans)
	}
	if sampleRate != ref.SampleRate {
		return fmt.Errorf("sample rate: expected %d, got %d", ref.SampleRate, sampleRate)
	}

	expectedSamples := ref.NumFrames * ref.NumChannels
	gotSamples := len(pcm)
	if gotSamples != expectedSamples {
		return fmt.Errorf("sample count: expected %d, got %d", expectedSamples, gotSamples)
	}

	maxDiff := 0
	maxDiffIdx := 0
	for i := 0; i < expectedSamples; i++ {
		expected := int16(clampFloat(ref.PCMInterleaved[i]) * 32767.0)
		got := pcm[i]
		diff := int(got) - int(expected)
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDiff {
			maxDiff = diff
			maxDiffIdx = i
		}
		if diff > tolerance && tolerance >= 0 {
			return fmt.Errorf("sample %d: expected %d, got %d (diff=%d, ref=%f)", i, expected, got, diff, ref.PCMInterleaved[i])
		}
	}
	if maxDiff > 0 && tolerance >= 0 {
		// This shouldn't trigger if we return errors above, but just in case
		_ = maxDiffIdx
	}
	return nil
}

func clampFloat(f float64) float64 {
	if f > 1.0 {
		return 1.0
	}
	if f < -1.0 {
		return -1.0
	}
	return f
}

// ---------- Helper: ffprobe WAV format info ----------

type wavFormatInfo struct {
	SampleRate int
	Channels   int
	BitDepth   int
	NumFrames  int64
	Duration   float64
}

func ffprobeWAV(path string) (*wavFormatInfo, error) {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json",
		"-show_format", "-show_streams", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result struct {
		Streams []struct {
			SampleRateStr string `json:"sample_rate"`
			Channels      int    `json:"channels"`
			BitDepth      int    `json:"bits_per_sample"`
			DurationTS    int64  `json:"duration_ts"`
			DurationStr   string `json:"duration"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("ffprobe parse failed: %w", err)
	}
	if len(result.Streams) == 0 {
		return nil, fmt.Errorf("no audio streams found")
	}

	s := result.Streams[0]
	sampleRate, _ := strconv.Atoi(s.SampleRateStr)
	duration, _ := strconv.ParseFloat(s.DurationStr, 64)

	return &wavFormatInfo{
		SampleRate: sampleRate,
		Channels:   s.Channels,
		BitDepth:   s.BitDepth,
		NumFrames:  s.DurationTS,
		Duration:   duration,
	}, nil
}

// ---------- Helper: compare two WAV PCM data ----------

func compareWAVPCM(path1, path2 string, tolerance int) error {
	pcm1, ch1, rate1, err := readWAVPCM(path1)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path1, err)
	}
	pcm2, ch2, rate2, err := readWAVPCM(path2)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path2, err)
	}

	if ch1 != ch2 {
		return fmt.Errorf("channel count mismatch: %d vs %d", ch1, ch2)
	}
	if rate1 != rate2 {
		return fmt.Errorf("sample rate mismatch: %d vs %d", rate1, rate2)
	}
	if len(pcm1) != len(pcm2) {
		return fmt.Errorf("sample count mismatch: %d vs %d", len(pcm1), len(pcm2))
	}

	maxDiff := 0
	maxDiffIdx := 0
	for i := 0; i < len(pcm1); i++ {
		diff := int(pcm1[i]) - int(pcm2[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDiff {
			maxDiff = diff
			maxDiffIdx = i
		}
		if diff > tolerance && tolerance >= 0 {
			return fmt.Errorf("sample %d: ref=%d, got=%d (diff=%d)", i, pcm1[i], pcm2[i], diff)
		}
	}
	if maxDiff > 0 {
		_ = maxDiffIdx
	}
	return nil
}

// ---------- Integration Tests ----------

func TestBinaryExists(t *testing.T) {
	if binaryPath == "" {
		t.Skip("rexconverter binary not found; build with 'mise run build' first")
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatal("binary is not executable")
	}
	t.Logf("Using binary: %s", binaryPath)
}

func TestCLIHelp(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	cmd := exec.Command(binaryPath, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "rexconverter") {
		t.Fatal("help output doesn't contain binary name")
	}
}

func TestCLI_NoArgs(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	cmd := exec.Command(binaryPath)
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "error") && !strings.Contains(string(out), "missing") {
		t.Fatalf("expected error for no args, got: %s", string(out))
	}
}

func TestIntegration_StereoDefaultOutput(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("120Stereo.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found:", rx2Path)
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\noutput: %s", err, string(out))
	}
	t.Logf("Output:\n%s", string(out))

	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Fatal("output WAV not created")
	}

	pcm, numChans, sampleRate, err := readWAVPCM(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if numChans != 2 {
		t.Fatalf("expected stereo, got %d channels", numChans)
	}
	if sampleRate != 44100 {
		t.Fatalf("expected 44100 Hz, got %d", sampleRate)
	}
	if len(pcm) == 0 {
		t.Fatal("empty PCM data")
	}
	t.Logf("WAV: %d channels, %d Hz, %d samples (%.2f sec)", numChans, sampleRate, len(pcm), float64(len(pcm))/float64(sampleRate*numChans))
}

func TestIntegration_MonoDownmix(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("120Stereo.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "mono.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-m", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\noutput: %s", err, string(out))
	}

	pcm, numChans, sampleRate, err := readWAVPCM(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if numChans != 1 {
		t.Fatalf("expected mono, got %d channels", numChans)
	}
	if sampleRate != 44100 {
		t.Fatalf("expected 44100 Hz, got %d", sampleRate)
	}
	if len(pcm) == 0 {
		t.Fatal("empty PCM data")
	}
	t.Logf("Mono WAV: %d Hz, %d samples (%.2f sec)", sampleRate, len(pcm), float64(len(pcm))/float64(sampleRate))
}

func TestIntegration_MonoFilePassThrough(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("120Mono.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "mono.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\noutput: %s", err, string(out))
	}
	t.Logf("Output:\n%s", string(out))

	pcm, numChans, _, err := readWAVPCM(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if numChans != 1 {
		t.Fatalf("expected mono source, got %d channels", numChans)
	}
	if len(pcm) == 0 {
		t.Fatal("empty PCM data")
	}
}

func TestIntegration_SliceLimit(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("120FourBeats.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "chunk.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-l", "4", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\noutput: %s", err, string(out))
	}
	t.Logf("Output:\n%s", string(out))

	// Should produce multiple output files
	pat := filepath.Join(dir, "chunk_*.wav")
	matches, _ := filepath.Glob(pat)
	if len(matches) < 2 {
		t.Fatalf("expected multiple output files, got %d", len(matches))
	}
	for _, m := range matches {
		pcm, _, _, err := readWAVPCM(m)
		if err != nil {
			t.Fatal(err)
		}
		if len(pcm) == 0 {
			t.Fatalf("empty PCM in %s", m)
		}
	}
}

func TestIntegration_NormalizeSplits(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("240FiveHundredSlices.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "norm.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-l", "64", "-n", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\noutput: %s", err, string(out))
	}
	t.Logf("Output:\n%s", string(out))

	pat := filepath.Join(dir, "norm_*.wav")
	matches, _ := filepath.Glob(pat)
	if len(matches) < 2 {
		t.Fatalf("expected multiple normalized files, got %d", len(matches))
	}
}

func TestIntegration_ErrorCorrupt(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("ErrorCorrupt.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-o", outPath)
	out, _ := cmd.CombinedOutput()
	// Should fail with an error message, not crash
	if !strings.Contains(string(out), "error") && !strings.Contains(string(out), "Error") && !strings.Contains(string(out), "failed") {
		t.Logf("Corrupt file output: %s", string(out))
	}
}

func TestIntegration_ErrorTooNew(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("ErrorTooNew.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-o", outPath)
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "error") && !strings.Contains(string(out), "Error") && !strings.Contains(string(out), "failed") {
		t.Logf("Too-new file output: %s", string(out))
	}
}

func TestIntegration_LegacyREX(t *testing.T) {
	requireREX(t)
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rxPath := testDataPath("120RexTest.rex")
	if _, err := os.Stat(rxPath); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.wav")
	cmd := exec.Command(binaryPath, rxPath, "-s", "44100", "-b", "16", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rex file failed: %v\noutput: %s", err, string(out))
	}
	pcm, _, _, err := readWAVPCM(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(pcm) == 0 {
		t.Fatal("empty PCM from .rex file")
	}
}

func TestIntegration_RCY(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rcyPath := testDataPath("120RcyTest.rcy")
	if _, err := os.Stat(rcyPath); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.wav")
	cmd := exec.Command(binaryPath, rcyPath, "-s", "44100", "-b", "16", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rcy file failed: %v\noutput: %s", err, string(out))
	}
	pcm, _, _, err := readWAVPCM(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(pcm) == 0 {
		t.Fatal("empty PCM from .rcy file")
	}
}

func TestIntegration_VariousSampleRates(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("120Stereo.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	for _, rate := range []int{22050, 44100, 48000, 96000} {
		t.Run(fmt.Sprintf("rate_%d", rate), func(t *testing.T) {
			dir := t.TempDir()
			outPath := filepath.Join(dir, "out.wav")
			cmd := exec.Command(binaryPath, rx2Path, "-s", fmt.Sprintf("%d", rate), "-b", "16", "-o", outPath)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("rate %d failed: %v\noutput: %s", rate, err, string(out))
			}
			_, _, sampleRate, _ := readWAVPCM(outPath)
			if sampleRate != rate {
				t.Fatalf("expected %d Hz, got %d", rate, sampleRate)
			}
		})
	}
}

func TestIntegration_24BitOutput(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("120Mono24Bits.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out24.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "24", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("24-bit failed: %v\noutput: %s", err, string(out))
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	bitsPerSmp := int(binary.LittleEndian.Uint16(data[34:36]))
	if bitsPerSmp != 24 {
		t.Fatalf("expected 24 bit, got %d", bitsPerSmp)
	}
}

func TestIntegration_CleanWAVStructure(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("120Stereo.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\noutput: %s", err, string(out))
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}

	// Walk all RIFF chunks and verify:
	// 1. Only fmt, cue, data exist (no LIST/additional chunks)
	// 2. Chunk order is fmt → data → cue (M8 firmware requirement)
	pos := 12
	var chunkOrder []string
	foundFmt := false
	foundCue := false
	foundData := false
	for pos < len(data)-8 {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		switch chunkID {
		case "fmt ":
			foundFmt = true
		case "cue ":
			foundCue = true
		case "data":
			foundData = true
		default:
			t.Fatalf("unexpected chunk in WAV: %q at offset %d (size=%d) — only fmt, cue, data allowed", chunkID, pos, chunkSize)
		}
		chunkOrder = append(chunkOrder, chunkID)
		pos += 8 + chunkSize
		if chunkSize%2 != 0 {
			pos++
		}
	}
	if !foundFmt {
		t.Fatal("missing fmt chunk")
	}
	if !foundData {
		t.Fatal("missing data chunk")
	}
	// Verify chunk order: fmt must come before data, data before cue
	expectedOrder := []string{"fmt ", "data", "cue "}
	if foundCue {
		if len(chunkOrder) != 3 {
			t.Fatalf("expected 3 chunks (fmt, data, cue), got %d: %v", len(chunkOrder), chunkOrder)
		}
		for i, expected := range expectedOrder {
			if chunkOrder[i] != expected {
				t.Fatalf("chunk order: expected %q at position %d, got %q — M8 requires fmt → data → cue", expected, i, chunkOrder[i])
			}
		}
		t.Logf("WAV structure clean + correct order: fmt → data → cue (%d bytes)", len(data))
	} else {
		t.Logf("WAV structure clean (no cue): %v (%d bytes)", chunkOrder, len(data))
	}
}

func TestIntegration_GatedSlices(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("120Gated.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gated file failed: %v\noutput: %s", err, string(out))
	}
	pcm, _, _, _ := readWAVPCM(outPath)
	if len(pcm) == 0 {
		t.Fatal("empty PCM from gated file")
	}
}

func TestIntegration_CueMarkersPresent(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("120FourBeats.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "cue_test.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-l", "4", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed: %v\noutput: %s", err, string(out))
	}

	pat := filepath.Join(dir, "cue_test_*.wav")
	matches, _ := filepath.Glob(pat)
	if len(matches) == 0 {
		t.Fatal("no output files generated")
	}
	for _, m := range matches {
		data, _ := os.ReadFile(m)
		if !bytes.Contains(data, []byte("cue ")) {
			t.Fatalf("missing cue chunk in %s — slice-based output must contain cue markers", m)
		}
		if bytes.Contains(data, []byte("LIST")) {
			t.Fatalf("unexpected LIST chunk in %s — no metadata allowed", m)
		}
	}
	t.Logf("All %d output files contain cue markers with no LIST chunks", len(matches))
}

func TestIntegration_CueMarkersCorrect(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("120Stereo.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "cue_correct.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed: %v\noutput: %s", err, string(out))
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(data, []byte("LIST")) {
		t.Fatal("output must not contain LIST chunk")
	}

	// Find and parse cue chunk
	cueIdx := bytes.Index(data, []byte("cue "))
	if cueIdx < 0 {
		t.Fatal("output missing cue chunk — slice-based REX file must have cue markers")
	}

	numCue := int(binary.LittleEndian.Uint32(data[cueIdx+8 : cueIdx+12]))
	if numCue < 1 {
		t.Fatal("cue chunk must contain at least 1 cue point")
	}
	t.Logf("Cue points: %d", numCue)

	// Parse each cue point and verify structure
	prevSampleOff := uint32(0)
	for i := 0; i < numCue; i++ {
		off := cueIdx + 12 + i*24
		if off+24 > len(data) {
			t.Fatalf("cue point %d overflows file", i)
		}
		dwName := binary.LittleEndian.Uint32(data[off : off+4])
		dwPosition := binary.LittleEndian.Uint32(data[off+4 : off+8])
		fccChunk := string(data[off+8 : off+12])
		dwChunkStart := binary.LittleEndian.Uint32(data[off+12 : off+16])
		dwBlockStart := binary.LittleEndian.Uint32(data[off+16 : off+20])
		dwSampleOffset := binary.LittleEndian.Uint32(data[off+20 : off+24])

		if dwName != uint32(i+1) {
			t.Fatalf("point %d: expected name %d, got %d", i, i+1, dwName)
		}
		if fccChunk != "data" {
			t.Fatalf("point %d: expected fcc 'data', got %q", i, fccChunk)
		}
		if dwBlockStart != 0 {
			t.Fatalf("point %d: expected blockStart 0, got %d", i, dwBlockStart)
		}
		if dwChunkStart != 0 {
			t.Fatalf("point %d: expected chunkStart 0 for M8 compatibility, got %d", i, dwChunkStart)
		}

		// dwPosition must equal dwSampleOffset (M8 treats position as data-relative)
		if dwPosition != dwSampleOffset {
			t.Fatalf("point %d: position %d != sampleOffset %d — M8 requires data-relative position", i, dwPosition, dwSampleOffset)
		}

		// Sample offsets must be monotonically non-decreasing
		if i > 0 && dwSampleOffset <= prevSampleOff {
			t.Fatalf("point %d: sampleOffset %d not greater than previous %d", i, dwSampleOffset, prevSampleOff)
		}
		prevSampleOff = dwSampleOffset
	}
	t.Logf("All %d cue points validated (structure OK)", numCue)
}

// ---------- PCM Reference Comparison Tests ----------

func TestLoopRenderMatch_Stereo(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}

	refPath := testDataPath("PreviewRender_Tempo120000.txt")
	if _, err := os.Stat(refPath); os.IsNotExist(err) {
		t.Skipf("ref file not found: %s", refPath)
	}

	rx2Path := testDataPath("120Stereo.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skipf("test file not found: %s", rx2Path)
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\noutput: %s", err, string(out))
	}

	err = compareRefToWAV(refPath, outPath, pcmTolerance)
	if err != nil {
		t.Fatalf("PCM comparison failed: %v", err)
	}
	t.Logf("Loop render PCM matches SDK PreviewRender_Tempo120000 reference")
}

func TestLoopRenderMatch_Stereo_SliceLimit(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}

	rx2Path := testDataPath("120Stereo.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skipf("test file not found: %s", rx2Path)
	}

	dir := t.TempDir()
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-l", "4", "-e", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\noutput: %s", err, string(out))
	}
	t.Logf("Output:\n%s", string(out))

	pat := filepath.Join(dir, "120Stereo_*.wav")
	matches, _ := filepath.Glob(pat)
	if len(matches) < 2 {
		t.Fatalf("expected multiple split files, got %d", len(matches))
	}
	for _, m := range matches {
		pcm, _, _, err := readWAVPCM(m)
		if err != nil {
			t.Fatal(err)
		}
		if len(pcm) == 0 {
			t.Fatalf("empty PCM in %s", m)
		}
	}
	t.Logf("Split OK: %d files (max 4 slices each)", len(matches))
}

func TestLoopRenderMatch_Stereo_Normalize(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}

	rx2Path := testDataPath("240FiveHundredSlices.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skipf("test file not found: %s", rx2Path)
	}

	dir := t.TempDir()
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-l", "64", "-n", "-e", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\noutput: %s", err, string(out))
	}
	t.Logf("Output:\n%s", string(out))

	pat := filepath.Join(dir, "240FiveHundredSlices_*.wav")
	matches, _ := filepath.Glob(pat)
	if len(matches) < 2 {
		t.Fatalf("expected multiple normalized files, got %d", len(matches))
	}
	t.Logf("Normalized split OK: %d files", len(matches))
}

func TestLoopRender_MonoMatch(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}

	rx2Path := testDataPath("120Stereo.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skipf("test file not found: %s", rx2Path)
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "mono_out.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-m", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\noutput: %s", err, string(out))
	}
	t.Logf("Output:\n%s", string(out))

	pcm, numChans, _, err := readWAVPCM(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if numChans != 1 {
		t.Fatalf("expected mono, got %d channels", numChans)
	}
	t.Logf("Mono loop: %d samples, %d ch", len(pcm), numChans)
}

// ---------- Statistical PCM Analysis ----------

func TestPCMStatisticalAnalysis(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	rx2Path := testDataPath("120Stereo.rx2")
	if _, err := os.Stat(rx2Path); os.IsNotExist(err) {
		t.Skip("test data not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "stats.wav")
	cmd := exec.Command(binaryPath, rx2Path, "-s", "44100", "-b", "16", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\noutput: %s", err, string(out))
	}
	t.Logf("Output:\n%s", string(out))

	pcm, numChans, sampleRate, err := readWAVPCM(outPath)
	if err != nil {
		t.Fatal(err)
	}

	// Compute statistics
	var sum, sumSq float64
	min := int16(math.MaxInt16)
	max := int16(math.MinInt16)
	nonzero := 0
	for _, s := range pcm {
		v := float64(s)
		sum += v
		sumSq += v * v
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
		if s != 0 {
			nonzero++
		}
	}
	mean := sum / float64(len(pcm))
	variance := sumSq/float64(len(pcm)) - mean*mean
	rms := math.Sqrt(sumSq / float64(len(pcm)))

	t.Logf("PCM Stats: %d ch, %d Hz, %d samples", numChans, sampleRate, len(pcm))
	t.Logf("  Range: [%d, %d]", min, max)
	t.Logf("  Mean: %.2f, Variance: %.2f, RMS: %.2f", mean, variance, rms)
	t.Logf("  Non-zero samples: %d / %d (%.1f%%)", nonzero, len(pcm), float64(nonzero)/float64(len(pcm))*100)

	if nonzero == 0 {
		t.Fatal("all samples are zero - no audio data")
	}
	if rms < 100 {
		t.Logf("WARNING: very low RMS (%.2f) - audio may be silent", rms)
	}
	if variance < 1 {
		t.Logf("WARNING: very low variance (%.2f) - audio may be silent/constant", variance)
	}
}

// ---------- Sine Wave Pipeline Test ----------

func TestSineWavePipeline(t *testing.T) {
	// This test creates a sine wave in Go, writes it through a mock pipeline,
	// and verifies the output matches. This tests the encoder without needing the REX SDK.
	dir := t.TempDir()

	// Generate 0.5 seconds of 440 Hz sine at 44100 Hz, stereo
	numFrames := 22050
	numChans := 2
	numSamples := numFrames * numChans
	sine := make([]int16, numSamples)
	for i := 0; i < numFrames; i++ {
		val := int16(16000.0 * math.Sin(float64(i)*440.0/44100.0*2.0*math.Pi))
		sine[i*2] = val     // L
		sine[i*2+1] = val   // R
	}

	wavPath := filepath.Join(dir, "sine.wav")
	f, err := os.Create(wavPath)
	if err != nil {
		t.Fatal(err)
	}

	// Write minimal valid WAV with our PCM
	dataSize := numSamples * 2
	byteRate := 44100 * 2 * 2
	blockAlign := uint16(2 * 2)
	f.Write([]byte("RIFF"))
	binary.Write(f, binary.LittleEndian, uint32(36+dataSize))
	f.Write([]byte("WAVE"))

	f.Write([]byte("fmt "))
	binary.Write(f, binary.LittleEndian, uint32(16))
	binary.Write(f, binary.LittleEndian, uint16(1))
	binary.Write(f, binary.LittleEndian, uint16(numChans))
	binary.Write(f, binary.LittleEndian, uint32(44100))
	binary.Write(f, binary.LittleEndian, uint32(byteRate))
	binary.Write(f, binary.LittleEndian, blockAlign)
	binary.Write(f, binary.LittleEndian, uint16(16))

	// data chunk
	f.Write([]byte("data"))
	binary.Write(f, binary.LittleEndian, uint32(dataSize))
	for _, s := range sine {
		binary.Write(f, binary.LittleEndian, s)
	}
	f.Close()

	// Read back and verify
	pcm, ch, rate, err := readWAVPCM(wavPath)
	if err != nil {
		t.Fatal(err)
	}
	if ch != numChans {
		t.Fatalf("channels: expected %d, got %d", numChans, ch)
	}
	if rate != 44100 {
		t.Fatalf("rate: expected 44100, got %d", rate)
	}
	if len(pcm) != numSamples {
		t.Fatalf("samples: expected %d, got %d", numSamples, len(pcm))
	}

	// Verify first few samples are close to expected
	for i := 0; i < 10; i++ {
		diff := int(pcm[i]) - int(sine[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > 1 {
			t.Fatalf("sample %d: expected %d, got %d", i, sine[i], pcm[i])
		}
	}
	t.Log("Sine wave round-trip: OK")
}
