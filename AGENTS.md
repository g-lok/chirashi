# rexconverter — Contributor Guide

Go + Zig hybrid CLI tool for cross-format sliced instrument conversion.
**Input** (14 extensions): REX/RX2/RCY (REX SDK), XRNI (Renoise), ADV/ALS/ADG
(Ableton), WAV, AIFF/AIFC, PTI (Polyend Tracker), OT (Octatrack), XY (OP-XY),
D2PST (Digitakt II). **Output** (12 formats): WAV, PTI, OT, AIFF (standard),
OP-1 AIFF, XY preset, EL text, DT2 preset, XRNI, ADV, ALS, ADG.
REX is the only format with no output encoder (REX SDK read-only).

## Architecture

### Phase 1 — REX → multi-format output (current)

**macOS:** Zig pipeline (Go c-archive linked into Zig binary):
```
mise run build  →  zig build (orchestrates Go → Zig link)
     └── Go compiled as static C archive (buildmode=c-archive)
     └── Zig compiles extractor.zig (main entrypoint, calls REX SDK C API)
     └── Go archive statically linked into Zig executable
     └── install_name_tool patches framework rpath at end
```

**Linux:** Pure Go standalone binary (no Zig, no REX SDK):
```
CGO_ENABLED=0 go build -o rexconverter main_linux.go
```
Cross-compile: set `GOOS=linux GOARCH=amd64|arm64|arm GOARM=7`.

### Phase 2 — Cross-format universal converter (current)
Any audio input → any output format. REX is the only read-only format
(no SDK write API).

### Language Roles

| Layer | Role | Key Files |
|-------|------|-----------|
| **Zig** | Main executable (macOS/Windows). Calls REX SDK via `b.addTranslateC()` (build-time C→Zig translation). Exports CGo functions. On Linux, a stub exists for CI compatibility but is unused. | `internal/rexengine/extractor.zig`, `internal/rexengine/extractor_stub.zig` |
| **Go** | CLI (cobra), file I/O, format encoding, cue marker calculation, input format parsing. Compiled as c-archive (macOS/Windows) or standalone binary (Linux). | `main.go`, `cmd/root.go`, `internal/rexengine/` |
| **REX SDK** | Proprietary C library from Reason Studios for reading/rendering REX files. Read-only — no write API. macOS/Windows only. | `internal/rexengine/REX.h`, `internal/rexengine/libs/macos/` |
| **Linux** | Pure Go standalone (`CGO_ENABLED=0`). No Zig, no REX SDK. All 10 non-REX input formats + 12 output formats work. | `main_linux.go` |

### Data Flow

```
Any input file bytes (REX, XRNI, ADV, ALS, ADG, WAV, AIFF, PTI, OT, XY, D2PST)
  → ext dispatch → InputReader.Read()
    ├─ .rex/.rx2/.rcy  → Zig CGo → REX SDK (mutex-guarded)
    ├─ .xrni           → reader_xrni.go  ZIP+XML → FLAC decode → split at markers
    ├─ .adv/.als       → reader_simpler.go  GZip XML → find sample + slice points
    ├─ .adg            → reader_drumrack.go  GZip XML → per-pad samples → parallel decode
    ├─ .wav            → reader_wav.go  RIFF → cue markers → split PCM
    ├─ .aif/.aiff      → reader_aiff.go  FORM → MARK chunks → split PCM
    ├─ .pti            → reader_pti.go  PTI header → raw PCM → single slice
    ├─ .ot             → reader_ot.go  OT sidecar → slice table → companion WAV
    ├─ .xy             → reader_xy.go  ZIP → patch.json → per-slice WAVs
    └─ .d2pst          → reader_d2pst.go  ZIP → manifest.json + TLV → WAV
  → Normalized []SliceExtraction (same type for all readers)
  → Go optionally downmixes/splits at cue boundaries
  → Go optionally resamples (linear interpolation), converts bit depth,
     downmixes to mono (5 strategies: sum/left/right/difference/dual-detect)
   → Go routes to selected encoder:
        wav   → EncodeWavContainer (fmt + data + cue)
        pti   → EncodePTI (392-byte header + 44.1k/16-bit mono PCM)
        ot    → EncodeWavContainer + EncodeOT (0x340-byte sidecar)
        aif   → EncodeAIFF (standard AIFF, no OP-1 metadata)
        aif-op1 → EncodeOP1AIF (AIFF + APPL "op-1" JSON chunk)
        xy    → EncodeXYPreset (ZIP with patch.json + per-slice WAVs)
        el    → EncodeWavContainer + EncodeEL (text sidecar)
        d2pst → EncodeDT2Preset (ZIP: manifest.json + 48k WAV + preset binary)
        xrni  → EncodeXRNI (ZIP: Instrument.xml + WAV + slice markers)
        adv   → EncodeSimplerADV (GZip XML + companion WAV)
        als   → EncodeSimplerALS (GZip XML Live Set wrapper + WAV)
        adg   → EncodeDrumRackADG (GZip XML + per-pad WAVs)
```

### Output-Only Formats (REX excluded)

REX SDK has no write API — REX/RX2/RCY cannot be produced. All other input
formats have corresponding output encoders for full round-trip conversion.

## Code Layout

```
├── main.go                  # C-archive entry, exports GoMainEntry()
├── cmd/root.go              # Cobra CLI flags + validation
├── internal/rexengine/
│   ├── bridge.go            # CGo bridge: calls Zig exported functions
│   ├── encoder.go           # Manual WAV encoder (no external libs)
│   ├── encoder_pti.go       # PTI format: 392-byte header + 44.1k/16-bit mono PCM
│   ├── encoder_ot.go        # OT sidecar: 0x340-byte big-endian binary w/ checksum
│   ├── encoder_op1.go       # OP-1 AIFF: FORM/AIFF/COMM/APPL(op-1 JSON)/SSND
│   ├── encoder_xy.go        # XY preset ZIP: patch.json + per-slice WAVs
│   ├── encoder_el.go        # EL text sidecar: key-zone mapping format
│   ├── encoder_d2pst.go     # DT2 preset ZIP: manifest.json + WAV + TLV preset bin
│   ├── encoder_aiff.go      # Standard AIFF: FORM/COMM/MARK/SSND (no OP-1 metadata)
│   ├── encoder_xrni.go      # XRNI ZIP: Instrument.xml + WAV + SliceMarkers
│   ├── encoder_simpler.go   # ADV/ALS: GZip XML Simpler preset + companion WAV
│   ├── encoder_adg.go       # ADG: GZip XML Drum Rack + per-pad WAVs
│   ├── resample.go          # ForceSampleRate, DownmixToMono (5 strategies), format force-helpers
│   ├── extractor.zig        # REX SDK interface via translate-c (Zig)
│   ├── runner.go            # Pipeline orchestrator
│   ├── types.go             # Go data types
│   ├── reader.go            # InputReader interface + registry
│   ├── reader_xrni.go       # Renoise XRNI (ZIP + XML + FLAC)
│   ├── reader_simpler.go    # Ableton Simpler (ADV/ALS, GZip XML)
│   ├── reader_drumrack.go   # Ableton Drum Rack (ADG, GZip XML)
│   ├── reader_wav.go        # WAV RIFF + cue markers
│   ├── reader_aiff.go       # AIFF/AIFC FORM + MARK chunks
│   ├── reader_pti.go        # Polyend Tracker instrument
│   ├── reader_ot.go         # Octatrack sidecar + companion WAV
│   ├── reader_xy.go         # OP-XY preset ZIP (patch.json + WAVs)
│   ├── reader_d2pst.go      # Digitakt II preset ZIP (manifest + TLV)
│   ├── REX.h                # REX SDK C header (patched for MinGW)
│   ├── rex/REX.c            # Windows DLL loader (outside CGo path)
│   └── libs/macos/          # macOS REX Shared Library.framework
├── build.zig                # Build coordinator
└── tests/
    ├── integration_test.go  # 30 integration tests
    ├── encoder_test.go      # WAV unit tests
    ├── encoder_format_test.go # Multi-format encoder tests (subprocess)
    ├── processor_test.go    # Slice partition tests
    └── testdata/            # Test REX files
```

## Building

### Prerequisites

- **Go** 1.26+ (managed via [mise](https://mise.jdx.dev) or manually)
- **Zig** 0.16.0+ (managed via mise or manually, macOS/Windows only)
- **REX SDK v1.9.2** — download from Reason Studios:
  - macOS: `REXSDK_Mac_1.9.2.zip` → place REX Shared Library.framework in `internal/rexengine/libs/macos/`
  - Windows: `REXSDK_Win_1.9.2.zip` → place `REX Shared Library.dll` + `.lib` alongside the built binary

### Commands

```bash
# macOS native build (Zig pipeline: Go c-archive → Zig executable)
mise run build
# Output: build/rexconverter

# macOS universal + Windows x86_64 release archives
mise run build-releases
# Output: build/releases/ (tar.gz + zip)

# Linux standalone build (pure Go, no Zig/REX SDK needed)
CGO_ENABLED=0 go build -o rexconverter main_linux.go

# Linux cross-compile targets
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o rexconverter-linux-amd64 main_linux.go
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o rexconverter-linux-arm64 main_linux.go
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -o rexconverter-linux-arm main_linux.go

# Manual macOS build
zig build -Dtarget=x86_64-macos -Doptimize=ReleaseSafe
```

### Windows Cross-Compile (from macOS)

```bash
CC="zig cc -target x86_64-windows-gnu" GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  go build -buildmode=c-archive -tags netgo -o build/go_engine_windows.a main.go
cd build && ar x go_engine_windows.a && zig ar rcs go_engine_windows.a *.o && rm -f *.o && cd ..
zig build -Dtarget=x86_64-windows-gnu -Doptimize=ReleaseSafe \
  "-Dgo-archive=build/go_engine_windows.a" "--prefix" build/zig-out-win
```

Note: The `ar x` + `zig ar rcs` dance fixes BSD-format archives (created by macOS `ar`) for
lld-link compatibility on Windows targets.

## Testing

```bash
go test ./tests/...
```

Tests run the built binary as a subprocess (`os/exec`), so build first.

Key test categories:

| Test | What it validates |
|------|------------------|
| `TestIntegration_StereoDefaultOutput` | Full pipeline: stereo REX → WAV output |
| `TestIntegration_SliceLimit` | `--slice-limit` splitting at cue boundaries |
| `TestIntegration_NormalizeSplits` | `--normalize-splits` balanced partitioning |
| `TestIntegration_CleanWAVStructure` | Only fmt/data/cue chunks present (no LIST/INFO) |
| `TestIntegration_CueMarkersCorrect` | Every cue point field validated |
| `TestLoopRenderMatch_Stereo` | PCM matches SDK PreviewRender (0/176k samples off by >2) |
| `TestFormatPTI` | PTI header byte validation + PCM length |
| `TestFormatOT` | OT sidecar checksum + structure + 64-slice table |
| `TestFormatOP1` | OP-1 AIFF form type + APPL chunk JSON |
| `TestFormatXY` | XY ZIP structure + patch.json regions |
| `TestFormatEL` | EL text sidecar key-zone sections |
| `TestFormat_FLAG_DT2` | DT2 ZIP manifest + WAV + TLV preset binary |
| `TestNoSlicesFlag` | `--no-slices` produces single monolithic WAV |
| `TestMonoModeFlags` | `--mono-mode` strategies produce correct channels |
| `TestInput_XRNI` | XRNI file → correct slice count + PCM |
| `TestInput_ADVSimpler` | ADV Simpler → correct slice count + positions |
| `TestInput_ADGDrumRack` | ADG Drum Rack → correct pad count + audio |
| `TestInput_ALS_Simpler` | ALS with Simpler → slice extraction |
| `TestEncodeAIFF` | AIFF FORM/COMM/MARK/SSND structure |
| `TestEncodeXRNI` | XRNI ZIP Instrument.xml + WAV |
| `TestEncodeXRNI_RoundTrip` | XRNI encode → decode → 2 slices |
| `TestEncodeSimplerADV` | ADV GZip XML Ableton-compatible |
| `TestEncodeSimplerALS` | ALS GZip XML Live Set structure |
| `TestEncodeDrumRackADG` | ADG DrumBranchPreset + DrumCell |


## Adding a New CLI Flag

1. Add field to `PipelineConfig` in `internal/rexengine/bridge.go`
2. Add flag + var in `cmd/root.go` `init()` function
3. Wire flag → PipelineConfig in `RunE`
4. Use `pipelineConfig.Field` in `runner.go`
5. Add test in `tests/integration_test.go`

## Adding a New Output Format

1. Create `internal/rexengine/encoder_<format>.go` with `Encode<Format>(...)` function
   - Simple (WAV/PTI/AIFF): single PCM blob → writer
   - Sidecar (OT/EL): WAV + companion metadata file
   - Composite (XRNI/XY/DT2): ZIP or directory with audio + metadata
   - DAW Preset (ADV/ALS/ADG): GZip XML + companion WAV(s)
2. Create `internal/rexengine/resample.go` helpers if format forces specific sample rate/channels
3. Add format case to the switch in `runner.go` `writeOutputFiles()`
4. Add entry to `deviceMaxSlices` map in `runner.go` (0 = unlimited)
5. Add format to `fileNameLimit()` in `runner.go` (0 = default 255)
6. Add format to `--format` help string in `cmd/root.go`
7. Add tests in `tests/encoder_format_test.go` (or `tests/encoder_<format>_test.go`)

## Adding a New Input Format (Phase 2)

1. Create `internal/rexengine/reader_<format>.go` implementing `InputReader` interface
2. Return `[]SliceExtraction` from PCM + positional data
3. Input readers should be stateless (no shared data) for thread safety
4. For WAV/AIFF: hand-rolled RIFF/FORM parsing (follow existing encoder patterns)
5. For MP3/FLAC/OGG: wrap pure Go library (`go-mp3`, `go-flac`, `oggvorbis`) — all permissive license, no CGo
6. For unsupported formats: optional ffmpeg subprocess fallback, runtime-detected (not a hard dependency)

### Existing Input Readers

| Format | File | Container | Audio Source | Slice Source | Deps |
|--------|------|-----------|--------------|--------------|------|
| REX/RX2 | `bridge.go` (Zig CGo) | Raw bytes → SDK | REX SDK render | SDK PPQ positions | REX SDK (proprietary) |
| XRNI | `reader_xrni.go` | ZIP | Embedded FLAC/WAV | `<SliceMarkers><SamplePosition>` | `mewkiz/flac` |
| ADV | `reader_simpler.go` | GZip XML | External WAV via `<FileRef>/<Path>` | `<InitialSlicePointsFromOnsets><SlicePoint TimeInSeconds>` | stdlib only |
| ALS | `reader_simpler.go` | GZip XML | External WAV via `<FileRef>/<Path>` | Same Simpler structure as ADV | stdlib only |
| ADG | `reader_drumrack.go` | GZip XML | External WAV per pad via `<FileRef>/<Path>` | Each pad = 1 slice (MIDI note zone) | stdlib only |
| WAV | `reader_wav.go` | RIFF | `data` chunk PCM | `cue ` chunk `dwSampleOffset` | stdlib only |
| AIFF | `reader_aiff.go` | FORM IFF | `SSND` chunk PCM | `MARK` chunk positions | stdlib only |
| PTI | `reader_pti.go` | Raw binary | Header + PCM at offset 392 | Single slice (no splitting) | stdlib only |
| OT | `reader_ot.go` | Raw binary | Companion WAV (same basename) | Sidecar slice table (64× start/end) | stdlib only |
| XY | `reader_xy.go` | ZIP | Per-slice WAVs in `slices/` | `patch.json` regions array | stdlib only |
| D2PST | `reader_d2pst.go` | ZIP | Embedded WAV | TLV binary or monolithic fallback | stdlib only |

### Key XML Structures

**Renoise XRNI** (`Instrument.xml`):
```xml
<RenoiseInstrument doc_version="34">
  <Name>Kit Name</Name>
  <SampleGenerator><Samples>
    <Sample IsAlias="false">
      <FileName>//File:/path/sample.flac</FileName>
      <SliceMarkers>
        <SliceMarker><SamplePosition>9433</SamplePosition></SliceMarker>
        <!-- N markers = N+1 slices -->
      </SliceMarkers>
      <Mapping><BaseNote>36</BaseNote></Mapping>
    </Sample>
    <Sample IsAlias="true">
      <!-- alias per slice, each with BaseNote, no FileName -->
      <Mapping><BaseNote>37</BaseNote></Mapping>
      <LoopEnd>9427</LoopEnd>
    </Sample>
  </Samples></SampleGenerator>
</RenoiseInstrument>
```

**Ableton Simpler** (ADV/ALS):
```xml
<OriginalSimpler>
  <MultiSampleMap><SampleParts>
    <MultiSamplePart HasImportedSlicePoints="true">
      <Name>Slice Name</Name>
      <SampleRef>
        <FileRef><Path>Samples/Imported/sample.wav</Path></FileRef>
        <DefaultDuration Value="1024916"/>
        <DefaultSampleRate Value="44100"/>
      </SampleRef>
      <SlicingBeatGrid Value="4"/>
      <SlicingRegions Value="8"/>
      <InitialSlicePointsFromOnsets>
        <SlicePoint TimeInSeconds="0" Rank="0"/>
        <SlicePoint TimeInSeconds="0.6026" Rank="0"/>
      </InitialSlicePointsFromOnsets>
    </MultiSamplePart>
  </SampleParts></MultiSampleMap>
</OriginalSimpler>
```

**Ableton Drum Rack** (ADG):
```xml
<DrumGroupDevice>
  <DrumPadsListWrapper><Pads>
    <Pad>
      <MidiNote Value="36"/>
      <Chain>
        <DeviceChain>
          <OriginalSimpler>
            <SampleRef>
              <FileRef><Path>kick.wav</Path></FileRef>
            </SampleRef>
          </OriginalSimpler>
        </DeviceChain>
      </Chain>
    </Pad>
    <!-- N pads, each a one-shot on its MIDI note -->
  </Pads></DrumPadsListWrapper>
</DrumGroupDevice>
```

### Concurrency Rules

`runPipeline()` spawns 1 goroutine per input file, throttled by `runtime.NumCPU()`. Each goroutine calls `processFileBuffer()` which runs the entire read→downmix→group→encode pipeline without locking. Only the REX SDK CGo calls inside `RenderLoopPreview()`/`RenderSlicesPreview()` hold `rexMutex` (defined in `bridge.go`). All other paths (readers, downmix, grouping, encoders) are lock-free.

| Path | Thread-safe? | Strategy |
|------|-------------|----------|
| REX SDK (CGo/Zig) | ❌ (except RenderPreviewBatch) | `rexMutex` in bridge.go guards RenderLoopPreview / RenderSlicesPreview |
| All non-REX readers | ✅ | Stateless, no locking needed |
| Drum Rack reader | ✅ | Per-pad decode parallel via sync.WaitGroup |
| Downmix/resample | ✅ | Local data, no shared state |
| Group/build output | ✅ | Local data, no shared state |
| All output encoders | ✅ | Each chunk written independently in parallel goroutines |

## SDK Notes

- The REX SDK is **not thread-safe** (except `REXRenderPreviewBatch`)
- `REX.h` is patched at line 84: `#elif defined(__GNUC__)` (was `__GNUC__ && REX_MAC`) for MinGW support
- Output WAV uses `fmt → data → cue` chunk order, `dwPosition = dwSampleOffset` (sample offset, not byte offset), `dwChunkStart = 0`
- `REX.h` is translated via `b.addTranslateC()` in `build.zig` with target-specific `defineCMacro` calls for `REX_MAC`/`REX_WINDOWS`
- `REXGetInfoFromBuffer()` is available but unused — enables fast metadata scanning without `REXCreate`
- `REXGetCreatorInfo()` is never called — creator metadata not extracted

## REX SDK License

This project links against the Reason Studios REX SDK, which has specific license terms:

- **Royalty-free** for study, amendment, and use
- **Cannot be used** with copyleft-licensed open source software
- The SDK license and copyright notice must be distributed with all copies
- The SDK is **read-only** — there is no API to produce REX/RX2 files
- **Phase 2** input readers (pure Go decoders, optional ffmpeg subprocess) do NOT link against the REX SDK and have no license restrictions

See `REX_SDK_LICENSE.txt` and `NOTICE.md` for full details.
