# rexconverter

[![CI](https://github.com/g-lok/rexconverter/actions/workflows/ci.yml/badge.svg)](https://github.com/g-lok/rexconverter/actions/workflows/ci.yml)

Cross-format sliced instrument converter. **14 input → 12 output formats.**
Only REX is read-only (REX SDK has no write API).

**Input:** REX/RX2/RCY, XRNI (Renoise), ADV/ALS/ADG (Ableton), WAV, AIFF/AIFC,
PTI (Polyend Tracker), OT (Octatrack), XY (OP-XY), D2PST (Digitakt II)

**Output:** WAV, PTI, OT, AIFF (standard), OP-1 AIFF, XY preset, EL text,
DT2 preset, XRNI, ADV, ALS, ADG

## Features

- **Cross-format conversion** — any input → any output (14→12)
- **Multi-format output** (`--format`) — wav, pti, ot, aif, aif-op1, xy, el, d2pst, xrni, adv, als, adg
- **9 pure Go input readers** (11 extensions) — no CGo, no external deps (except FLAC for XRNI)
- **REX SDK input** — REX/RX2/RCY via Reason Studios REX SDK (macOS/Windows only)
- **RIFF cue markers** — each slice gets a proper cue point
- **Batch conversion** — convert entire directories recursively
- **Slice splitting** (`--slice-limit`) — split renders at cue boundaries
- **Mono downmix** (5 strategies) — sum, left, right, difference, dual-detect
- **Sample rate/bit depth conversion, tempo override**
- **Ableton library path resolution** (`--library-path`) — find samples across machines
- **Cross-platform** — macOS (native, arm64+x86_64), Windows (cross-compiled), Linux (standalone)

## System Requirements

| OS          | Architecture                                          |
| ----------- | ----------------------------------------------------- |
| macOS 11+   | Intel (x86_64) and Apple Silicon (arm64)              |
| Windows 10+ | x86_64                                                |
| Linux       | x86_64, arm64, arm (standalone, no Zig/REX SDK needed) |

Building from source: Go 1.26+. macOS/Windows REX builds additionally need Zig 0.16.0+ and REX SDK v1.9.2. Linux builds are pure Go (`CGO_ENABLED=0`).

## Quick Start

```bash
# REX → WAV (default)
rexconverter loop.rx2 -o output.wav

# REX → Polyend Tracker
rexconverter loop.rx2 --format pti -o loop.pti

# Renoise XRNI → Ableton Simpler preset
rexconverter kit.xrni --format adv -o kit.adv

# Ableton Drum Rack → Octatrack
rexconverter kit.adg --format ot -o kit.wav

# WAV with cue markers → Digitakt II preset
rexconverter sliced.wav --format d2pst -o kit.d2pst

# Batch convert a directory
rexconverter --input-dir ./samples --output-dir ./converted
```

## Installation

### macOS

[Homebrew](https://brew.sh/):
```bash
brew install g-lok/tap/rexconverter
```

Manually: download `.tar.gz` from [Releases](https://github.com/g-lok/rexconverter/releases).
`Frameworks/` folder must be in the same directory as the binary.

```bash
tar xzf rexconverter-<version>-macos.tar.gz
cd rexconverter-<version>-macos
./rexconverter --help
```

### Linux

Download standalone binary from [Releases](https://github.com/g-lok/rexconverter/releases):

```bash
chmod +x rexconverter-linux-*
./rexconverter-linux-amd64 --help
```

No REX SDK or Zig needed on Linux. REX/RX2/RCY input is unavailable on Linux
(REX SDK is proprietary, macOS/Windows only). All other 11 input formats work.

### Windows

[Scoop](https://scoop.sh/):
```powershell
scoop bucket add g-lok https://github.com/g-lok/scoop-bucket
scoop install rexconverter
```

Manually: download `.zip` from [Releases](https://github.com/g-lok/rexconverter/releases).
Keep `REX Shared Library.dll` alongside `rexconverter.exe`.

### Build from Source

**Linux (standalone, no REX):**
```bash
CGO_ENABLED=0 go build -o rexconverter main_linux.go
```

**macOS/Windows (with REX support):** Go 1.26+, Zig 0.16.0+, REX SDK v1.9.2
```bash
mise install    # install Go + Zig
mise run build  # build binary
```

REX SDK: download from [Reason Studios](https://developer.reasonstudios.com/downloads/other-products).
- **macOS**: `REX Shared Library.framework` → `internal/rexengine/libs/macos/`
- **Windows**: `REX Shared Library.dll` alongside the built binary

## Usage

```text
rexconverter [INPUT_FILES...] [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| Flag | Short | Description |
|------|-------|-------------|
| `--input-file` | `-i` | Target input file(s) |
| `--input-dir` | `-d` | Scan directory for supported files |
| `--input-format` | | Force input format (auto-detect by ext if unset) |
| `--output-file` | `-o` | Output path (single input only) |
| `--output-dir` | `-e` | Output directory for batch conversions |
| `--recursive` | `-r` | Recurse subdirectories (requires --input-dir) |
| `--preserve` | `-p` | Preserve directory structure in output |
| `--bit-rate` | `-b` | Bit depth: 8, 16, or 24 |
| `--sample-rate` | `-s` | Output sample rate in Hz |
| `--mono` | `-m` | Downmix to mono |
| `--tempo` | `-t` | Override tempo in BPM (0 = original) |
| `--slice-limit` | `-l` | Max slices per output file |
| `--normalize-splits` | | Balance slices evenly across splits |
| `--no-slices` | `-n` | Render as single unsliced sample (no cue markers) |
| `--mono-mode` | | Mono downmix: sum, left, right, difference, dual-detect |
| `--format` | `-f` | Output format (see above) |
| `--library-path` | | Ableton User Library path for sample resolution |
| `--sample-path-mode` | | Sample path style in XML: relative, absolute, library |
| `--quiet` | `-q` | Suppress progress output |
| `--verbose` | `-v` | Debug output |
| `--version` | | Print version |

### Format-Specific Output Extensions

| Format | Extension | Notes |
|--------|-----------|-------|
| `wav` | `.wav` | RIFF with fmt+data+cue chunks |
| `pti` | `.pti` | 392-byte header + 44.1k/16 mono PCM |
| `ot` | `.wav` + `.ot` | Octatrack sidecar + companion WAV |
| `aif` | `.aif` | Standard AIFF (COMM/SSND/MARK) |
| `aif-op1` | `.aif` | OP-1 AIFF with APPL(op-1 JSON) |
| `xy` | `.preset.zip` | OP-XY ZIP with patch.json + per-slice WAVs |
| `el` | `.wav` + `_slices.txt` | Elektron multi-sample text sidecar |
| `d2pst` | `.dt2pst` | Digitakt II ZIP + TLV preset |
| `xrni` | `.xrni` | Renoise ZIP with Instrument.xml + WAV |
| `adv` | `.adv` + `Samples/Imported/` WAV | Ableton Simpler preset (GZip XML) |
| `als` | `.als` + `Samples/Imported/` WAV | Ableton Live Set (GZip XML) |
| `adg` | `.adg` + `Samples/Imported/` WAVs | Ableton Drum Rack (GZip XML) |

### Examples

```bash
# Single output with cue markers
rexconverter loop.rx2 -o output.wav

# Split into files of up to 8 slices each
rexconverter loop.rx2 --slice-limit 8 -o split.wav

# Override tempo, suppress progress
rexconverter loop.rx2 --tempo 140 --quiet -o output.wav

# Batch directory, preserve structure
rexconverter --input-dir ./tracks --output-dir ./wavs --preserve

# Polyend Tracker instrument (forces 44.1kHz/16-bit/mono)
rexconverter loop.rx2 --format pti -o loop.pti

# Elektron Octatrack (WAV + .ot sidecar)
rexconverter loop.rx2 --format ot -o loop.wav

# Standard AIFF (no OP-1 metadata)
rexconverter loop.rx2 --format aif -o loop.aif

# OP-1 / OP-Z drum instrument
rexconverter loop.rx2 --format aif-op1 -o loop.aif

# OP-XY drum preset (ZIP with per-slice WAVs + patch.json)
rexconverter loop.rx2 --format xy -o loop.preset.zip

# Elektron multi-sample text sidecar (WAV + _slices.txt)
rexconverter loop.rx2 --format el -o loop.wav

# Digitakt II preset (ZIP with manifest.json + WAV + preset binary)
rexconverter loop.rx2 --format d2pst -o loop.dt2pst

# Renoise instrument (ZIP: WAV + slice markers)
rexconverter loop.rx2 --format xrni -o loop.xrni

# Ableton Simpler preset (GZip XML + companion WAV)
rexconverter loop.rx2 --format adv -o loop.adv

# Ableton Live Set (GZip XML + companion WAV)
rexconverter loop.rx2 --format als -o loop.als

# Ableton Drum Rack (GZip XML + per-pad WAVs)
rexconverter loop.rx2 --format adg -o loop.adg

# Cross-format: convert XRNI to ADV
rexconverter kit.xrni --format adv -o kit.adv

# Cross-format: convert WAV with cues to PTI
rexconverter sliced.wav --format pti -o kit.pti

# Cross-format: convert ADG to OT
rexconverter kit.adg --format ot -o kit.wav

# Cross-format: convert AIFF with MARK chunks to DT2
rexconverter kit.aif --format d2pst -o kit.dt2pst

# Ableton sample resolution via library path
rexconverter kit.adv --library-path "~/Music/Ableton/User Library"

# Mono downmix using left channel only
rexconverter loop.rx2 --mono --mono-mode left -o output.wav

# Render as single continuous sample (no slice cues)
rexconverter loop.rx2 --no-slices -o loop.wav
```

## How It Works

1. **Input** — bytes read from any supported file format
2. **Dispatch** — REX/RX2/RCY → REX SDK (CGo, mutex-guarded). All others → pure Go `InputReader`
3. **Normalize** — all readers return `[]SliceExtraction` (PCM + cue markers + metadata)
4. **Process** — optional: downmix, resample, bit-depth convert, slice group
5. **Encode** — route to selected format encoder

### Input Readers

| Format | Reader | Audio Source | Slice Source |
|--------|--------|--------------|--------------|
| REX/RX2/RCY | REX SDK (Zig CGo) | SDK render | SDK PPQ positions |
| XRNI | `reader_xrni.go` | ZIP → FLAC/WAV | `<SliceMarker><SamplePosition>` |
| ADV/ALS | `reader_simpler.go` | External WAV | `<SlicePoint TimeInSeconds>` |
| ADG | `reader_drumrack.go` | External per-pad WAVs | 1 per pad (MIDI note zone) |
| WAV | `reader_wav.go` | RIFF `data` chunk | `cue ` chunk `dwSampleOffset` |
| AIFF/AIFC | `reader_aiff.go` | FORM `SSND` chunk | `MARK` chunk positions |
| PTI | `reader_pti.go` | 392-byte header + PCM | Single slice |
| OT | `reader_ot.go` | Companion WAV | 64-slice sidecar table |
| XY | `reader_xy.go` | ZIP → per-slice WAVs | `patch.json` regions |
| D2PST | `reader_d2pst.go` | ZIP → embedded WAV | TLV binary |

### Output Encoders

| Format | Encoder | Strategy |
|--------|---------|----------|
| WAV | `encoder.go` | RIFF fmt+data+cue |
| PTI | `encoder_pti.go` | 392-byte header + PCM |
| OT | `encoder_ot.go` | 0x340-byte sidecar + WAV |
| AIFF | `encoder_aiff.go` | FORM/COMM/SSND/MARK |
| AIF-OP1 | `encoder_op1.go` | AIFF + APPL "op-1" JSON |
| XY | `encoder_xy.go` | ZIP: patch.json + WAVs |
| EL | `encoder_el.go` | WAV + text sidecar |
| DT2 | `encoder_d2pst.go` | ZIP: manifest + WAV + TLV |
| XRNI | `encoder_xrni.go` | ZIP: Instrument.xml + WAV |
| ADV | `encoder_simpler.go` | GZip XML + companion WAV |
| ALS | `encoder_simpler.go` | GZip XML Live Set + WAV |
| ADG | `encoder_adg.go` | GZip XML + per-pad WAVs |

### Concurrency

All input readers (except REX SDK) are stateless and goroutine-safe.
Only REX SDK CGo calls (`Zig_RenderLoopPreview`/`Zig_RenderSlicesPreview`)
hold `rexMutex` in `bridge.go` — all other operations (readers, downmix,
group, encode) are lock-free. File processing runs N workers parallel
(N = `runtime.NumCPU()`). ADG reader additionally parallelizes per-pad
audio decode via `sync.WaitGroup`.

## REX SDK Dependency

This project uses the Reason Studios REX SDK v1.9.2 (proprietary, royalty-free).
The SDK is **read-only** — no API to produce REX/RX2/RCY files.

All non-REX input readers are pure Go (no CGo, no linking). The REX SDK is
only needed for `.rex`/`.rx2`/`.rcy` input. Phase 2 readers have no license
conflict — they are permissively licensed pure Go decoders.

Release archives bundle the SDK framework binary for end-user convenience.
These binaries remain proprietary Reason Studios property (not MIT licensed).

See `REX_SDK_LICENSE.txt` and `NOTICE.md` for full terms.

## SHA256 Verification

Release artifacts include SHA256 checksums:

```bash
# macOS
shasum -a 256 rexconverter-<version>-macos.tar.gz

# Windows
shasum -a 256 rexconverter-<version>-windows.zip
```

## Contributing

See [`AGENTS.md`](AGENTS.md) for contributor guide, architecture, build instructions.

All contributions must pass the full test suite:

```bash
go test ./...
```

CI runs on every push (Linux standalone + macOS native). Ensure compatibility
with Go 1.26+. REX builds additionally need Zig 0.16.0+.

## License

MIT License (rexconverter source code). REX SDK components are proprietary Reason Studios property — `REX_SDK_LICENSE.txt`.
