//go:build !cgo

package engine

import "fmt"

// HasREX reports whether the REX SDK is available on this platform.
func HasREX() bool { return false }

func RenderLoopPreview(fileData []byte, targetSampleRate, tempo int) (*SliceExtraction, error) {
	return nil, fmt.Errorf("REX format not supported on this platform (REX SDK is proprietary, macOS/Windows only)")
}

func RenderSlicesPreview(fileData []byte, targetSampleRate, tempo int) ([]SliceExtraction, error) {
	return nil, fmt.Errorf("REX format not supported on this platform (REX SDK is proprietary, macOS/Windows only)")
}

func InitEngine(verbose bool) error {
	if verbose {
		fmt.Println("REX SDK not available on this platform")
	}
	return nil
}

func CloseEngine() error {
	return nil
}
