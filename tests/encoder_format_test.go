package chirashi_test

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func hasBinary() bool {
	_, err := os.Stat(binaryPath)
	return err == nil
}

func runChirashi(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func findTestREX() string {
	if runtime.GOOS == "linux" {
		return ""
	}
	candidates := []string{
		"tests/testdata/Baby Huey & The Babysitters - Listen to Me.rx2",
		"testdata/Baby Huey & The Babysitters - Listen to Me.rx2",
		"tests/testdata/120Stereo.rx2",
		"testdata/120Stereo.rx2",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	dir := "tests/testdata"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".rx2") || strings.HasSuffix(e.Name(), ".rex")) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

func TestFormat_FLAG_PTI(t *testing.T) {
	if !hasBinary() {
		t.Skip("binary not found; build with `mise run build` first")
	}
	rexFile := findTestREX()
	if rexFile == "" {
		t.Skip("no test REX file found in testdata/")
	}

	dir := t.TempDir()
	output, err := runChirashi(t,
		"--format", "pti",
		"--output-dir", dir,
		rexFile,
	)
	if err != nil {
		t.Fatalf("convert failed: %v\n%s", err, output)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	ptiFound := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pti") {
			ptiFound = true
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if string(data[0:2]) != "TI" {
				t.Fatalf("bad PTI magic: %q", data[0:2])
			}
			_ = binary.LittleEndian.Uint32(data[60:64])
		}
	}
	if !ptiFound {
		t.Fatal("no .pti output file found")
	}
}

func TestFormat_FLAG_OT(t *testing.T) {
	if !hasBinary() {
		t.Skip("binary not found")
	}
	rexFile := findTestREX()
	if rexFile == "" {
		t.Skip("no test REX file found")
	}

	dir := t.TempDir()
	output, err := runChirashi(t,
		"--format", "ot",
		"--output-dir", dir,
		rexFile,
	)
	if err != nil {
		t.Fatalf("convert failed: %v\n%s", err, output)
	}

	otFound := false
	wavFound := false
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ot") {
			otFound = true
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if len(data) != 0x340 {
				t.Fatalf("OT size: expected 0x340, got 0x%x", len(data))
			}
			if string(data[0:4]) != "FORM" {
				t.Fatalf("bad OT magic: %q", data[0:4])
			}
			var checksum uint16
			for i := 0x10; i < 0x33E; i++ {
				checksum += uint16(data[i])
			}
			storedChecksum := binary.BigEndian.Uint16(data[0x33E:0x340])
			if checksum != storedChecksum {
				t.Fatalf("OT checksum mismatch")
			}
		}
		if strings.HasSuffix(e.Name(), ".wav") {
			wavFound = true
		}
	}
	if !otFound {
		t.Fatal("no .ot output file found")
	}
	if !wavFound {
		t.Fatal("no .wav output file found (required alongside .ot)")
	}
}

func TestFormat_FLAG_OP1(t *testing.T) {
	if !hasBinary() {
		t.Skip("binary not found")
	}
	rexFile := findTestREX()
	if rexFile == "" {
		t.Skip("no test REX file found")
	}

	dir := t.TempDir()
	output, err := runChirashi(t,
		"--format", "aif-op1",
		"--output-dir", dir,
		rexFile,
	)
	if err != nil {
		t.Fatalf("convert failed: %v\n%s", err, output)
	}

	aifFound := false
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".aif") {
			aifFound = true
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if string(data[0:4]) != "FORM" {
				t.Fatalf("bad AIFF magic: %q", data[0:4])
			}
			if string(data[8:12]) != "AIFF" {
				t.Fatalf("bad AIFF type: %q", data[8:12])
			}
		}
	}
	if !aifFound {
		t.Fatal("no .aif output file found")
	}
}

func TestFormat_FLAG_XY(t *testing.T) {
	if !hasBinary() {
		t.Skip("binary not found")
	}
	rexFile := findTestREX()
	if rexFile == "" {
		t.Skip("no test REX file found")
	}

	dir := t.TempDir()
	output, err := runChirashi(t,
		"--format", "xy",
		"--output-dir", dir,
		rexFile,
	)
	if err != nil {
		t.Fatalf("convert failed: %v\n%s", err, output)
	}

	zipFound := false
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".preset.zip") {
			zipFound = true
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatalf("bad ZIP: %v", err)
			}
			hasJSON := false
			hasWAV := false
			for _, f := range zr.File {
				if f.Name == "patch.json" {
					hasJSON = true
				}
				if strings.HasPrefix(f.Name, "slice_") && strings.HasSuffix(f.Name, ".wav") {
					hasWAV = true
				}
			}
			if !hasJSON {
				t.Fatal("ZIP missing patch.json")
			}
			if !hasWAV {
				t.Fatal("ZIP missing slice WAVs")
			}
		}
	}
	if !zipFound {
		t.Fatal("no .preset.zip output file found")
	}
}

func TestFormat_FLAG_EL(t *testing.T) {
	if !hasBinary() {
		t.Skip("binary not found")
	}
	rexFile := findTestREX()
	if rexFile == "" {
		t.Skip("no test REX file found")
	}

	dir := t.TempDir()
	output, err := runChirashi(t,
		"--format", "el",
		"--output-dir", dir,
		rexFile,
	)
	if err != nil {
		t.Fatalf("convert failed: %v\n%s", err, output)
	}

	txtFound := false
	wavFound := false
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_slices.txt") {
			txtFound = true
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "ELEKTRON MULTI-SAMPLE") {
				t.Fatal("bad EL header")
			}
			if !strings.Contains(string(data), "[[key-zones]]") {
				t.Fatal("missing key-zones")
			}
		}
		if strings.HasSuffix(e.Name(), ".wav") {
			wavFound = true
		}
	}
	if !txtFound {
		t.Fatal("no _slices.txt output file found")
	}
	if !wavFound {
		t.Fatal("no .wav output file found (required alongside .txt)")
	}
}

func TestFormat_FLAG_DT2(t *testing.T) {
	if !hasBinary() {
		t.Skip("binary not found")
	}
	rexFile := findTestREX()
	if rexFile == "" {
		t.Skip("no test REX file found")
	}

	dir := t.TempDir()
	output, err := runChirashi(t,
		"--format", "d2pst",
		"--output-dir", dir,
		rexFile,
	)
	if err != nil {
		t.Fatalf("convert failed: %v\n%s", err, output)
	}

	dt2Found := false
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".dt2pst") {
			dt2Found = true
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatalf("bad ZIP: %v", err)
			}
			if len(zr.File) != 3 {
				t.Fatalf("expected 3 ZIP entries, got %d", len(zr.File))
			}
			for _, f := range zr.File {
				if f.Name == "manifest.json" {
					continue
				}
				if strings.HasSuffix(f.Name, ".wav") {
					continue
				}
				// third entry is the preset binary (no extension)
			}
		}
	}
	if !dt2Found {
		t.Fatal("no .dt2pst output file found")
	}
}

func TestFormat_NoSlices(t *testing.T) {
	if !hasBinary() {
		t.Skip("binary not found")
	}
	rexFile := findTestREX()
	if rexFile == "" {
		t.Skip("no test REX file found")
	}

	dir := t.TempDir()
	output, err := runChirashi(t,
		"--format", "wav",
		"--no-slices",
		"--output-dir", dir,
		rexFile,
	)
	if err != nil {
		t.Fatalf("convert failed: %v\n%s", err, output)
	}

	entries, _ := os.ReadDir(dir)
	wavCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".wav") {
			wavCount++
		}
	}
	if wavCount == 0 {
		t.Fatal("no WAV output with --no-slices")
	}
}

func TestFormat_MonoMode_Sum(t *testing.T) {
	if !hasBinary() {
		t.Skip("binary not found")
	}
	rexFile := findTestREX()
	if rexFile == "" {
		t.Skip("no test REX file found")
	}

	dir := t.TempDir()
	_, err := runChirashi(t,
		"--format", "wav",
		"--mono",
		"--mono-mode", "sum",
		"--output-dir", dir,
		rexFile,
	)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
}

func TestFormat_MonoMode_Left(t *testing.T) {
	if !hasBinary() {
		t.Skip("binary not found")
	}
	rexFile := findTestREX()
	if rexFile == "" {
		t.Skip("no test REX file found")
	}

	dir := t.TempDir()
	_, err := runChirashi(t,
		"--format", "wav",
		"--mono",
		"--mono-mode", "left",
		"--output-dir", dir,
		rexFile,
	)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
}

func TestFormat_NoSlices_PTI(t *testing.T) {
	if !hasBinary() {
		t.Skip("binary not found")
	}
	rexFile := findTestREX()
	if rexFile == "" {
		t.Skip("no test REX file found")
	}

	dir := t.TempDir()
	output, err := runChirashi(t,
		"--format", "pti",
		"--no-slices",
		"--output-dir", dir,
		rexFile,
	)
	if err != nil {
		t.Fatalf("convert failed: %v\n%s", err, output)
	}
}

func TestFormat_UnknownFormatDefaultsToWAV(t *testing.T) {
	if !hasBinary() {
		t.Skip("binary not found")
	}
	// Unknown format name gracefully falls through to WAV encoder
	wavPath := filepath.Join(t.TempDir(), "test.wav")
	wavData := buildMinimalWAV()
	if err := os.WriteFile(wavPath, wavData, 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := runChirashi(t,
		"--format", "bogus",
		wavPath,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, output)
	}
	if !strings.Contains(output, ".wav") {
		t.Fatalf("expected WAV output for unknown format, got: %s", output)
	}
}
