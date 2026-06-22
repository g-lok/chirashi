# Phase 2: Cross-Format Input Readers

Turn rexconverter from REX-only input into general-purpose (any)→(any but REX) converter.

## Architecture

Insert `InputReader` interface between file loading and pipeline. `processFileBuffer()` detects format by extension → dispatches to reader → produces `[]SliceExtraction` → existing encoder pipeline unchanged.

```
file bytes → ext dispatch → InputReader.Read()
  ├─ .rex/.rx2/.rcy  → REX SDK CGo (existing, mutex-guarded)
  ├─ .xrni           → reader_xrni.go  (ZIP+XML+FLAC)
  ├─ .adv/.als       → reader_simpler.go  (GZip XML, Simpler with slices)
  ├─ .adg            → reader_drumrack.go  (GZip XML, Drum Rack pads)
  ├─ .wav            → reader_wav.go  (RIFF WAV, cue markers for slicing)
  ├─ .aif/.aiff      → reader_aiff.go  (AIFF/AIFC, instrument chunks)
  ├─ .pti            → reader_pti.go  (Polyend Tracker instrument)
  ├─ .ot             → reader_ot.go  (Octatrack project + WAV sidecar)
  ├─ .xy             → reader_xy.go  (OP-XY preset ZIP: patch.json + WAVs)
  └─ .d2pst          → reader_d2pst.go  (Digitakt II preset ZIP: manifest.json + WAV + TLV)
```

## Concurrency Model

| Path | Thread-safe? | Strategy |
|------|-------------|----------|
| REX SDK (.rex/.rx2/.rcy) | ❌ | Serialized via mutex (existing) |
| XRNI reader (.xrni) | ✅ | Full goroutine per file. No shared state. |
| Simpler reader (.adv/.als) | ✅ | Full goroutine per file. Disk I/O bound. |
| Drum Rack reader (.adg) | ✅ | Per-file goroutine. Pad decodes: parallel via errgroup within file. |
| WAV reader (.wav) | ✅ | Stateless, goroutine per file. RIFF chunk parsing only. |
| AIFF reader (.aif/.aiff) | ✅ | Stateless, goroutine per file. FORM chunk parsing only. |
| PTI reader (.pti) | ✅ | Stateless, goroutine per file. Header parse + raw PCM. |
| OT reader (.ot) | ✅ | Stateless, goroutine per file. Sidecar parse + WAV inside. |
| XY reader (.xy) | ✅ | Stateless, goroutine per file. ZIP parse + JSON + WAVs. |
| DT2 reader (.d2pst) | ✅ | Stateless, goroutine per file. ZIP parse + TLV + WAV. |

`runPipeline()` worker pool already spawns up to `runtime.NumCPU()` goroutines — new readers drop into same pool without mutex.

## InputReader Interface

```go
type InputReader interface {
    Probe(data []byte) (*AudioMetadata, error)
    Read(data []byte, targetSampleRate int) ([]SliceExtraction, error)
    SupportedExtensions() []string
}
```

Registry:
```go
var readers []InputReader

func RegisterReader(r InputReader)    // called from init()
func DetectReader(ext string) InputReader  // finds reader by extension
func ProbeInput(data []byte) (InputReader, error) // magic-byte detection
```

## Files to Create

### 1. `internal/rexengine/reader.go` — Interface + registry

- `InputReader` interface
- `AudioMetadata` struct (may differ from `RexMetadata` — source-format agnostic)
- Global registry
- `DetectReader(ext)` dispatch
- `ProbeInput(data)` for magic-byte detection

### 2. `internal/rexengine/reader_xrni.go` — Renoise XRNI

**Format**: ZIP archive.
- `Instrument.xml` — metadata + slice markers + sample refs
- `SampleData/Sample00 (NAME).flac` — embedded audio (FLAC or WAV)

**Dep**: `github.com/mewkiz/flac` (MIT)

**Extraction**:
1. Open ZIP, find `Instrument.xml` + audio file
2. Parse XML:
   - `<SliceMarkers>/<SliceMarker>/<SamplePosition>` → sample offsets
   - N markers = N+1 slices (last goes to end)
   - `<FileName>` → audio path inside ZIP
   - Alias samples: `<IsAlias>true</IsAlias>`, each with `<Mapping>/<BaseNote>` = MIDI note
   - Slice duration from alias `<LoopEnd>` or `next_pos - current_pos`
3. Decode FLAC → float32 PCM
4. Split at slice positions → `[]SliceExtraction` (one per slice)
5. Per-slice: `WavCueMarker{Position: frameOffset}`

### 3. `internal/rexengine/reader_simpler.go` — Ableton Simpler (ADV, ALS)

**Format**: GZip XML, root `<Ableton>`.

**Sources**:
- `.adv` = standalone Simpler preset
- `.als` = full Live Set — find tracks with `<OriginalSimpler>` or `<Simpler>`

**Extraction**:
1. Decompress gzip
2. Find `<OriginalSimpler>` (ADV) or Live Set track with Simpler device (ALS)
3. From `MultiSamplePart`:
   - `<SampleRef>/<FileRef>/<Path>` → source audio path
   - `<DefaultSampleRate>` → source rate
   - `<DefaultDuration>` → sample count
4. From `<InitialSlicePointsFromOnsets>`:
   - `<SlicePoint TimeInSeconds="...">` → slice boundaries in seconds
   - Convert to frames: `pos = int(TimeInSeconds * sampleRate)`
5. Read source audio file (resolve path — relative to project dir, or absolute)
6. Decode WAV/AIFF → float32 PCM
7. Split at slice positions → `[]SliceExtraction`

**Path resolution** (Ableton hell):
- ALS: paths relative to `.als` project dir (`Samples/Imported/`, `Samples/Recorded/`)
- ADV: absolute paths or relative to Ableton User Library
- Fallback search: `--library-path` flag, or scan known locations
- On failure: clear error message with attempted paths

### 4. `internal/rexengine/reader_drumrack.go` — Ableton Drum Rack (ADG)

**Format**: GZip XML, root `<Ableton>`, contains `<DrumGroupDevice>`.

**Extraction**:
1. Decompress gzip
2. Find `<DrumGroupDevice>/<DrumPadsListWrapper>/<Pads>/<Pad>` elements
3. Each `<Pad>`:
   - `<MidiNote Value="N">` → key zone (0-127)
   - `<Chain>/<DeviceChain>/.../<Simpler>/<SampleRef>/<FileRef>/<Path>` → sample
4. Decode each pad's sample independently → float32 PCM
5. Return `[]SliceExtraction` (one per pad). Each slice:
   - PCM = entire decoded sample
   - `WavCueMarker{Position: 0, Label: "MIDI N"}`
   - Metadata tracks MIDI note for zone mapping

**Concurrency**: Pad decodes run in parallel via `errgroup` (N pads, goroutine per decode).

### 5. `internal/rexengine/reader_wav.go` — WAV with cue markers

**Format**: RIFF WAV. Slices come from `cue` chunk markers.

**Extraction**:
1. Parse RIFF header, find `fmt `, `data`, `cue ` chunks
2. `fmt ` → sample rate, bit depth, channels
3. `data` → PCM samples (supports 8/16/24/32-bit, float)
4. `cue ` → `struct CuePoint{ dwName, dwPosition, fccChunk, dwChunkStart, dwBlockStart, dwSampleOffset }`
   - `dwSampleOffset` = sample frame offset for each cue point
5. N cue points = N+1 slices (last slice to end)
6. Per-slice: `WavCueMarker{Position: dwSampleOffset}`
7. Return `[]SliceExtraction` (PCM chunks at cue boundaries)

**Edge cases**:
- WAV without `cue ` chunk → single monolithic slice (no slicing)
- WAV with `smpl` chunk → optional sample loop points (not primary slice source)
- Multiple `data` chunks → concatenate (rare, but valid RIFF)
- 24-bit PCM → expand to 32-bit int, normalize to float32

### 6. `internal/rexengine/reader_aiff.go` — AIFF/AIFC

**Format**: IFF FORM/AIFF or FORM/AIFC.

**Extraction**:
1. Parse FORM chunks: `COMM` → sample rate, bit depth, channels, frame count
2. `SSND` → PCM data (offset past header)
3. `INST` → instrument base note + envelope (optional, for mapping)
4. `MARK` → markers with positions + names (slice candidates)
5. No native cue chunk — use `MARK` chunks as slice points if present
6. AIFC: compressed formats only if `FL32`/`FL64`/`twos`/`sowt` (common), else error
7. OP-1 AIFF: `APPL("op-1")` chunk with JSON → skip JSON, use MARK/SSND for audio

**Edge cases**:
- AIFF without MARK chunks → single monolithic slice
- AIFC with uncommon compression → error with clear message
- OP-1 APPL JSON → read but ignore for audio extraction

### 7. `internal/rexengine/reader_pti.go` — Polyend Tracker Instrument

**Format**: 392-byte header + 44.1k/16-bit mono PCM.

**Header** (from `encoder_pti.go`):
```
Offset  Size  Field
0       4     "PTI\x00" magic
4       28    name (null-terminated)
32      1     version
33      1     root_note
34      1     fine_tune
35      1     volume
36      1     pan
37      1     sample_rate_index
38      1     loop_mode
39      4     loop_start (bytes)
43      4     loop_end (bytes)
47      4     sample_length (bytes)
51      1     midi_low
52      1     midi_high
53      1     midi_root
54      338   reserved (zeros)
392+          PCM data
```

**Extraction**:
1. Validate magic `"PTI\x00"`
2. Parse header fields
3. Read PCM data from offset 392 to end
4. Single slice: entire sample at `WavCueMarker{Position: 0}`
5. Metadata: root note, MIDI range, volume, pan, loop points
6. Return `[]SliceExtraction` with one entry

### 8. `internal/rexengine/reader_ot.go` — Elektron Octatrack

**Format**: 0x340-byte big-endian binary sidecar file. Actual audio in companion WAV.

**Sidecar structure** (from `encoder_ot.go`):
```
Offset  Size  Field
0       4     "OT\x00\x00" magic
4       2     reserved
6       2     slice_count
8       64*4  slice_start_bytes[]  (64 × uint32 big-endian)
264     64*4  slice_end_bytes[]    (64 × uint32 big-endian)
...
```

**Extraction**:
1. Read `.ot` sidecar — parse header + slice tables
2. Find companion WAV with same base name → read as WAV via `reader_wav.go`
3. Slice start/end values are byte offsets in companion WAV → convert to sample frames
4. Split PCM at slice boundaries → `[]SliceExtraction`
5. If no companion WAV found → error with expected path

**Design**: `ReadOT(data, companionWavPath string) → []SliceExtraction` or accept raw WAV bytes as second arg.

### 9. `internal/rexengine/reader_xy.go` — OP-XY Preset

**Format**: ZIP archive containing `patch.json` + per-slice WAV files.

**ZIP structure** (from `encoder_xy.go`):
```
preset.zip/
  ├── patch.json        # JSON with regions array
  └── slices/
      ├── slice_000.wav
      ├── slice_001.wav
      └── ...
```

**patch.json regions**:
```json
{
  "regions": [
    {"name": "Slice 1", "rootNote": 60, "sampleStart": 0, "sampleEnd": 44100},
    ...
  ]
}
```

**Extraction**:
1. Open ZIP, find `patch.json`
2. Parse JSON → regions array (name, rootNote per slice)
3. Find WAV files in `slices/` dir
4. Decode each WAV via WAV reader → PCM
5. Return `[]SliceExtraction` — one per region, with metadata

### 10. `internal/rexengine/reader_d2pst.go` — Digitakt II Preset

**Format**: ZIP archive with `manifest.json` + WAV + TLV preset binary.

**ZIP structure** (from `encoder_d2pst.go`):
```
preset.d2pst/
  ├── manifest.json     # preset metadata + sample info
  ├── sample.wav        # single WAV (all slices concatenated?)
  └── preset.tlv        # TLV-encoded preset binary
```

**Extraction**:
1. Open ZIP, find `manifest.json` + `sample.wav` + `preset.tlv`
2. Parse manifest → get sample rate, key zones
3. Parse TLV binary → extract slice positions (if TLV contains slice data)
4. Decode WAV → PCM
5. Split at slice positions → `[]SliceExtraction`
6. Map each slice to MIDI note from manifest/tlv

**Note**: DT2 format is complex. TLV binary format reverse-engineering needed. May produce 1 slice initially (no splitting) if TLV slice table format unknown.

## Pipeline Integration (`runner.go`)

### `processFileBuffer()` dispatch

```go
func processFileBuffer(fileData []byte, sourcePath string, cfg PipelineConfig) error {
    ext := strings.ToLower(filepath.Ext(sourcePath))

    var slices []SliceExtraction
    var err error

    switch ext {
    case ".rex", ".rx2", ".rcy":
        // existing REX SDK path — mutex serialized in bridge.go
        slices, err = renderREXSlices(fileData, cfg)
    default:
        reader := DetectReader(ext)
        if reader == nil {
            return fmt.Errorf("unsupported input format: %s", ext)
        }
        slices, err = reader.Read(fileData, cfg.SampleRate)
    }
    // ... rest of pipeline unchanged
}
```

### `scanDirectory()` update

```go
var supportedExts = []string{".rex", ".rx2", ".rcy", ".xrni", ".als", ".adv", ".adg", ".wav", ".aif", ".aiff", ".pti", ".ot", ".xy", ".d2pst"}
```

Switch from hardcoded `".rx2"` and `".rex"` filter to using `supportedExts` lookup.

## CLI Changes (`cmd/root.go`)

- Auto-detect by extension — no new flags required for basic use
- Optional `--input-format` flag for explicit format override
- Update `rootCmd.Short` and `RootCmd.Long` to reflect new scope
- Add `--library-path` flag for Ableton User Library path resolution

## REX SDK Mutex Clarification

REX SDK is only threaded through `bridge.go` → Zig CGo. The `rexMutex` in `bridge.go` serializes only the two CGo entry points (`Zig_RenderLoopPreview` and `Zig_RenderSlicesPreview`). All non-REX readers and encoder paths are lock-free and run fully concurrent across files.

## Implementation Order

| Step | File(s) | Description |
|------|---------|-------------|
| 1 | `reader.go` | Interface + registry |
| 2 | `reader_xrni.go` | XRNI reader (simplest — self-contained ZIP) |
| 3 | `reader_simpler.go` | ADV/ALS Simpler reader (needs path resolution) |
| 4 | `reader_drumrack.go` | ADG Drum Rack reader (multi-sample parallel decode) |
| 5 | `reader_wav.go` | WAV cue-marker reader (common interchange format) |
| 6 | `reader_aiff.go` | AIFF/AIFC MARK-chunk reader |
| 7 | `reader_pti.go` | Polyend Tracker instrument reader |
| 8 | `reader_ot.go` | Octatrack sidecar + companion WAV reader |
| 9 | `reader_xy.go` | OP-XY preset ZIP reader |
| 10 | `reader_d2pst.go` | Digitakt II preset ZIP reader |
| 11 | `runner.go`, `types.go` | Pipeline dispatch, metadata fields |
| 12 | `cmd/root.go` | Help text, `--library-path`, `--input-format` |
| 13 | Tests | Integration tests using example files in `ableton_kit/` and `*.xrni` |

## Edge Cases

- **Missing audio files** (sample not collected, cross-machine paths): Error with clear path list
- **XRNI silent slices** (gap between aliases): Zero-fill PCM
- **ADV without slicing** (Classic/1-Shot mode): 1 slice = whole file
- **Drum Rack nested Instrument Racks**: Follow chain to find Simpler, skip plugin-only
- **ADG missing chain preset refs**: Warning, skip pad
- **Sample rate mismatch**: Reader returns native rate, pipeline `ForceSampleRate` handles conversion
- **Ableton path is relative to User Library**: Try common paths, accept `--library-path`
- **WAV without `cue ` chunk**: Single monolithic slice (no slicing)
- **WAV 24-bit PCM**: Expand to 32-bit int, normalize to float32
- **AIFF without MARK chunks**: Single monolithic slice
- **AIFC with uncommon compression**: Error with supported-codec list
- **PTI with missing magic**: Error "not a PTI file"
- **OT without companion WAV**: Error listing expected paths
- **XY with missing patch.json**: Error listing ZIP contents
- **DT2 TLV unknown format**: Fall back to 1 slice = whole WAV

## Dependencies

| Package | License | Use |
|---------|---------|-----|
| `github.com/mewkiz/flac` | MIT | FLAC decode (XRNI) |
| `encoding/xml` | stdlib | XML parse (Simpler, Drum Rack) |
| `compress/gzip` | stdlib | Gunzip (Simpler, Drum Rack) |
| `archive/zip` | stdlib | ZIP read (XRNI, XY, DT2) |
| `encoding/json` | stdlib | JSON parse (XY patch.json, DT2 manifest.json) |
| `encoding/binary` | stdlib | BigEndian reads (OT sidecar, PTI header) |
| `bytes` | stdlib | RIFF/IFF chunk parsing (WAV, AIFF) |
| `io` | stdlib | Readers/writers |
| `compress/flac` (stdlib?) | — | **No** — Go stdlib has no FLAC decoder. Use `mewkiz/flac`. |
