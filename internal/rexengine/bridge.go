package rexengine

/*
#cgo CFLAGS: -I.
#include <stdlib.h>

typedef struct {
    int channels;
    int sample_rate;
    double tempo;
    double original_tempo;
    int time_sign_nom;
    int time_sign_denom;
    int bit_depth;
    int ppq_length;
} ZigMetadata;

typedef struct {
    int ppq_pos;
} ZigLoopSliceInfo;

typedef struct {
    ZigMetadata metadata;
    int tempo;
    int frame_length;
    int slice_count;
    ZigLoopSliceInfo* slice_info;
    float* pcm_data;
} ZigLoopRenderResult;

typedef struct {
    int frame_length;
    float* pcm_data;
    int ppq_pos;
    int sample_pos;
} ZigPerSliceResult;

typedef struct {
    ZigMetadata metadata;
    int tempo;
    int total_frames;
    int slice_count;
    ZigPerSliceResult* slices;
} ZigSlicesRenderResult;

int Zig_InitEngine(void);
void Zig_CloseEngine(void);
void Zig_Diagnostic(void);

void* Zig_RenderLoopPreview(const unsigned char* file_bytes, int byte_len, int target_sample_rate, int tempo_bpm);
void Zig_FreeLoopRenderResult(void* result);

void* Zig_RenderSlicesPreview(const unsigned char* file_bytes, int byte_len, int target_sample_rate, int tempo_bpm);
void Zig_FreeSlicesRenderResult(void* result);
*/
import "C"

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"unsafe"
)

// rexMutex serializes access to the REX SDK, which is NOT thread-safe.
// All CGo calls into Zig REX functions must hold this lock.
var rexMutex sync.Mutex

// HasREX reports whether the REX SDK is available on this platform.
func HasREX() bool { return true }

// InitEngine initializes the REX SDK framework. Called once at startup.
func InitEngine(verbose bool) error {
	if rc := C.Zig_InitEngine(); rc != 0 {
		return fmt.Errorf("Zig_InitEngine failed with code %d", rc)
	}
	if verbose {
		C.Zig_Diagnostic()
	}
	return nil
}

// CloseEngine shuts down the REX SDK framework. Called once at shutdown.
func CloseEngine() error {
	C.Zig_CloseEngine()
	return nil
}

// RenderLoopPreview renders the full REX loop at given tempo using SDK preview API.
// tempo: BPM * 1000 (e.g. 120000 for 120 BPM). Pass 0 to use original tempo.
func RenderLoopPreview(fileData []byte, targetSampleRate, tempo int) (*SliceExtraction, error) {
	if len(fileData) == 0 {
		return nil, errors.New("empty file data buffer")
	}

	cBytes := C.CBytes(fileData)
	defer C.free(cBytes)

	rexMutex.Lock()
	opaquePtr := C.Zig_RenderLoopPreview((*C.uchar)(cBytes), C.int(len(fileData)), C.int(targetSampleRate), C.int(tempo))
	rexMutex.Unlock()
	if opaquePtr == nil {
		return nil, errors.New("Zig loop render failed")
	}
	defer C.Zig_FreeLoopRenderResult(opaquePtr)

	cRes := (*C.ZigLoopRenderResult)(opaquePtr)

	meta := RexMetadata{
		Channels:      int(cRes.metadata.channels),
		SampleRate:    int(cRes.metadata.sample_rate),
		Tempo:         float64(cRes.metadata.tempo),
		OriginalTempo: float64(cRes.metadata.original_tempo),
		TimeSignNom:   int(cRes.metadata.time_sign_nom),
		TimeSignDenom: int(cRes.metadata.time_sign_denom),
		BitDepth:      int(cRes.metadata.bit_depth),
		PPQLength:     int(cRes.metadata.ppq_length),
	}

	loopFrames := int(cRes.frame_length)
	sliceCount := int(cRes.slice_count)
	totalSamples := loopFrames * meta.Channels
	actualTempo := int(cRes.tempo)

	interleaved := make([]float32, totalSamples)
	if totalSamples > 0 && cRes.pcm_data != nil {
		var cPCM []C.float
		pcmHeader := (*reflect.SliceHeader)(unsafe.Pointer(&cPCM))
		pcmHeader.Data = uintptr(unsafe.Pointer(cRes.pcm_data))
		pcmHeader.Len = totalSamples
		pcmHeader.Cap = totalSamples
		for s := 0; s < totalSamples; s++ {
			interleaved[s] = float32(cPCM[s])
		}
	}

	var cSliceInfo []C.ZigLoopSliceInfo
	infoHeader := (*reflect.SliceHeader)(unsafe.Pointer(&cSliceInfo))
	infoHeader.Data = uintptr(unsafe.Pointer(cRes.slice_info))
	infoHeader.Len = sliceCount
	infoHeader.Cap = sliceCount

	// framePos = sampleRate * 1000 * ppqPos / (tempo * 256)
	cuePoints := make([]WavCueMarker, sliceCount)
	for i := 0; i < sliceCount; i++ {
		ppqPos := int(cSliceInfo[i].ppq_pos)
		framePos := int(float64(meta.SampleRate) * 1000.0 * float64(ppqPos) / (float64(actualTempo) * 256.0))
		if framePos > loopFrames {
			framePos = loopFrames
		}
		cuePoints[i] = WavCueMarker{
			SliceID:  i,
			Position: uint32(framePos),
			Label:    fmt.Sprintf("Slice %02d", i+1),
		}
	}

	return &SliceExtraction{
		Metadata:    meta,
		CuePoints:   cuePoints,
		Interleaved: interleaved,
		TotalFrames: loopFrames,
	}, nil
}

// RenderSlicesPreview renders all slices into individual PCM buffers using SDK preview API.
// Returns one SliceExtraction per slice, with exact frame positions from Zig.
// tempo: BPM * 1000 (e.g. 120000 for 120 BPM). Pass 0 to use original tempo.
func RenderSlicesPreview(fileData []byte, targetSampleRate, tempo int) ([]SliceExtraction, error) {
	if len(fileData) == 0 {
		return nil, errors.New("empty file data buffer")
	}

	cBytes := C.CBytes(fileData)
	defer C.free(cBytes)

	rexMutex.Lock()
	opaquePtr := C.Zig_RenderSlicesPreview((*C.uchar)(cBytes), C.int(len(fileData)), C.int(targetSampleRate), C.int(tempo))
	rexMutex.Unlock()
	if opaquePtr == nil {
		return nil, errors.New("Zig slices render failed")
	}
	defer C.Zig_FreeSlicesRenderResult(opaquePtr)

	cRes := (*C.ZigSlicesRenderResult)(opaquePtr)

	meta := RexMetadata{
		Channels:      int(cRes.metadata.channels),
		SampleRate:    int(cRes.metadata.sample_rate),
		Tempo:         float64(cRes.metadata.tempo),
		OriginalTempo: float64(cRes.metadata.original_tempo),
		TimeSignNom:   int(cRes.metadata.time_sign_nom),
		TimeSignDenom: int(cRes.metadata.time_sign_denom),
		BitDepth:      int(cRes.metadata.bit_depth),
		PPQLength:     int(cRes.metadata.ppq_length),
	}

	sliceCount := int(cRes.slice_count)
	result := make([]SliceExtraction, sliceCount)

	var cSlices []C.ZigPerSliceResult
	slicesHeader := (*reflect.SliceHeader)(unsafe.Pointer(&cSlices))
	slicesHeader.Data = uintptr(unsafe.Pointer(cRes.slices))
	slicesHeader.Len = sliceCount
	slicesHeader.Cap = sliceCount

	for i := 0; i < sliceCount; i++ {
		frameLen := int(cSlices[i].frame_length)
		totalSamples := frameLen * meta.Channels

		pcm := make([]float32, totalSamples)
		if totalSamples > 0 && cSlices[i].pcm_data != nil {
			var cPCM []C.float
			pcmHeader := (*reflect.SliceHeader)(unsafe.Pointer(&cPCM))
			pcmHeader.Data = uintptr(unsafe.Pointer(cSlices[i].pcm_data))
			pcmHeader.Len = totalSamples
			pcmHeader.Cap = totalSamples
			for s := 0; s < totalSamples; s++ {
				pcm[s] = float32(cPCM[s])
			}
		}

		cuePoints := []WavCueMarker{
			{
				SliceID:  i,
				Position: 0,
				Label:    fmt.Sprintf("Slice %02d", i+1),
			},
		}

		result[i] = SliceExtraction{
			Metadata:    meta,
			CuePoints:   cuePoints,
			Interleaved: pcm,
			TotalFrames: frameLen,
		}
	}

	return result, nil
}
