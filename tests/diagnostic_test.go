package chirashi_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCLI_Info(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}
	t.Logf("Using binary: %s", binaryPath)

	cmd := exec.Command(binaryPath, "../tests/testdata/120Stereo.rx2", "--info")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("info failed: %v\noutput: %s", err, string(out))
	}

	sOut := string(out)
	expected := []string{
		"file_path: ../tests/testdata/120Stereo.rx2",
		"format: REX2",
		"channels: 2",
		"sample_rate_hz: 44100",
		"bpm: 120.0",
		"total_count: 10",
		"time_range: \"00:00:00:000 - 00:00:00:246\"",
	}

	for _, exp := range expected {
		if !strings.Contains(sOut, exp) {
			t.Errorf("missing expected output: %q", exp)
		}
	}
}

func TestCLI_DryRun(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}

	cmd := exec.Command(binaryPath, "../tests/testdata/120Stereo.rx2", "-f", "pti", "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run failed: %v\noutput: %s", err, string(out))
	}

	sOut := string(out)
	expected := []string{
		"target_format: pti",
		"channel_action: \"downmix to mono (sum)\"",
		"output_sample_rate_hz: 44100",
		"output_path:",
		"120Stereo.pti",
	}

	for _, exp := range expected {
		if !strings.Contains(sOut, exp) {
			t.Errorf("missing expected output: %q", exp)
		}
	}
}

func TestCLI_InfoJSON(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}

	cmd := exec.Command(binaryPath, "../tests/testdata/120Stereo.rx2", "--info", "--json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("info json failed: %v\noutput: %s", err, string(out))
	}

	sOut := string(out)
	if !strings.HasPrefix(sOut, "{") {
		t.Errorf("expected JSON output starting with '{', got: %s", sOut)
	}
	if !strings.Contains(sOut, "\"format\": \"REX2\"") {
		t.Errorf("missing format in JSON")
	}
}

func TestCLI_InfoBatch(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not found")
	}

	cmd := exec.Command(binaryPath, "../tests/testdata/120Stereo.rx2", "../tests/testdata/120Mono.rx2", "--info")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("batch info failed: %v\noutput: %s", err, string(out))
	}

	sOut := string(out)
	// Check for two documents
	docs := strings.Count(sOut, "---")
	if docs != 2 {
		t.Errorf("expected 2 YAML documents, got %d", docs)
	}
}
