# chirashi

![chirashi logo](assets/chirashi.png)

CLI tool for converting between sliced instrument formats used in hardware samplers and DAWs.

- **Cross-format** — convert between any supported input/output pair
- **Pure Go** — no external dependencies, no native SDKs required
- **REX/RX2/RCY** input via pure Go implementation
- Mono downmix, resampling, slice limiting, BPM override, BPM filename prefix

## Table of Contents

- [Installation](#installation)
- [Quick start](#quick-start)
- [Flags](#flags)
- [BPM prefix](#bpm-prefix)
- [Supported formats](#supported-formats)
  - [Input formats](#input-formats)
  - [Output formats](#output-formats)
- [Format details](#format-details)
- [WAV + cue markers](#wav--cue-markers)
- [AIFF / AIFF-C](#aiff--aiff-c)
- [CAF — Apple Loops](#caf--apple-loops)
- [Ableton Live presets](#ableton-live-presets)
- [Renoise XRNI](#renoise-xrni)
- [Polyend Tracker (PTI)](#polyend-tracker-pti)
- [Elektron Octatrack (OT)](#elektron-octatrack-ot)
- [Teenage Engineering OP-1](#teenage-engineering-op-1)
- [Teenage Engineering OP-XY (preset)](#teenage-engineering-op-xy-preset)
- [Elektron multi-sample (EL)](#elektron-multi-sample-el)
- [Elektron Digitakt II (DT2PST)](#elektron-digitakt-ii-dt2pst)
- [REX / RX2 / RCY](#rex--rx2--rcy)
- [Architecture](#architecture)
- [Building from source](#building-from-source)
- [Development](#development)
- [License](#license)

## Installation

### macOS / Linux (Homebrew)

```bash
brew install g-lok/tap/chirashi
```

### Windows (Scoop)

```powershell
scoop bucket add g-lok https://github.com/g-lok/scoop-bucket
scoop install chirashi
```

### Manual install (GitHub Releases)

Download the latest release for your platform from the [releases page](https://github.com/g-lok/chirashi/releases).

```bash
# macOS — universal binary (Apple Silicon + Intel)
VERSION=<latest tag>  # e.g. v0.5.0
curl -LO "https://github.com/g-lok/chirashi/releases/download/$VERSION/chirashi-$VERSION-macos.tar.gz"
tar xzf "chirashi-$VERSION-macos.tar.gz"
sudo mkdir -p /usr/local/opt
sudo mv chirashi-$VERSION-macos /usr/local/opt/chirashi
sudo ln -s /usr/local/opt/chirashi/chirashi /usr/local/bin/chirashi
```

```bash
# Linux amd64 — static binary
VERSION=<latest tag>
curl -LO "https://github.com/g-lok/chirashi/releases/download/$VERSION/chirashi-$VERSION-linux-amd64.tar.gz"
tar xzf "chirashi-$VERSION-linux-amd64.tar.gz"
sudo mv chirashi-$VERSION-linux-amd64 /usr/local/bin/chirashi
```

```powershell
# Windows (PowerShell)
$VERSION = "<latest tag>"
Invoke-WebRequest -Uri "https://github.com/g-lok/chirashi/releases/download/$VERSION/chirashi-$VERSION-windows.zip" -OutFile chirashi.zip
Expand-Archive chirashi.zip -DestinationPath chirashi
$installDir = "$env:LOCALAPPDATA\Programs\chirashi"
New-Item -ItemType Directory -Force -Path $installDir
Move-Item chirashi\* $installDir -Force
$env:Path += ";$installDir"
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$installDir", "User")
```

Verify checksums from `CHECKSUMS.txt` on the release.

## Quick start

```bash
chirashi [INPUT_FILES...] [flags]
```

Convert a Renoise XRNI to Polyend Tracker instrument:

```bash
chirashi kit.xrni -f pti -o pt_kit.pti
```

Convert anything to a Dirtywave M8-ready sliced WAV:

```bash
chirashi kit.pti -s 44100 -b 16 -e ./m8_output -f wav
```

Batch convert a directory of REX files to Ableton Drum Rack:

```bash
chirashi -d ./rex_files -e ./adg_output -f adg
```

Convert a REX2 to OP-XY preset (up to 24 slices):

```bash
chirashi loop.rx2 -l 24 -f xy -o xy_loop.preset.zip
```

Convert an Ableton Simpler preset to AIFF with slice markers:

```bash
chirashi simpler.adv -o output.aif
```

## Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--input-file` | `-i` | — | Input file(s) (repeatable) |
| `--input-dir` | `-d` | — | Scan directory for input files |
| `--output-file` | `-o` | — | Output path (single input only) |
| `--output-dir` | `-e` | — | Output directory for batch |
| `--format` | `-f` | `wav` | Output format (see below) |
| `--bit-rate` | `-b` | 16 | Bit depth: 8, 16, or 24 |
| `--sample-rate` | `-s` | source | Output sample rate in Hz (1k–1M) |
| `--mono` | `-m` | false | Downmix to mono |
| `--mono-mode` | — | `sum` | `sum`, `left`, `right`, `difference`, `dual-detect` |
| `--tempo` | `-t` | 0 | Override tempo in BPM (0 = use original, clamped to 20-450 range) |
| `--bpm-prefix` | — | false | Prepend BPM to filename (e.g. `128-Source.wav`). Sources: file metadata → filename patterns → `--tempo`. Ignored with `-o` (no `-l`) |
| `--slice-limit` | `-l` | 0 | Max slices per output file |
| `--no-slices` | `-n` | false | Ignore slice markers, render plain output |
| `--recursive` | `-r` | false | Recurse subdirs (with `--input-dir`) |
| `--preserve` | `-p` | false | Preserve directory structure (with `--input-dir`) |
| `--quiet` | `-q` | false | Suppress progress |
| `--verbose` | `-v` | false | Debug output |
| `--library-path` | — | — | Ableton User Library path |
| `--input-format` | — | auto | Force input format (override auto-detect) |
| `--sample-path-mode` | — | `relative` | Sample path style in XML (reserved) |

### BPM prefix

The `--bpm-prefix` flag prepends the detected BPM to output filenames (e.g., `128-SourceName.wav`).

**BPM resolution priority:**
1. File metadata `OriginalTempo` (REX, CAF Apple Loop)
2. File metadata `Tempo`
3. Filename patterns (`_NNNbpm` suffix anywhere in name, or `NNN` prefix)
4. `--tempo` CLI flag

**Conflict handling:** If `--tempo` is provided but differs from metadata BPM by more than 0.5, a warning is printed and metadata BPM is used for the prefix.

**Interaction with `-o` and `-l`:**
- `-o` without `-l`: prefix is skipped (single output file, path is exact)
- `-o` with `-l`: prefix is applied to each chunked output file

**Formatting:** BPM is formatted as integer (e.g., `128`) or 1-decimal float (e.g., `137.8`). Trailing `.0` is stripped.

```bash
# With --bpm-prefix, outputs "137-Winstons.wav"
chirashi "Winstons - Amen, Brother (ver.1).rx2" --bpm-prefix -o out.wav

# With --bpm-prefix and -l, outputs "128-kit_01.wav", "128-kit_02.wav", etc.
chirashi loop.rx2 --bpm-prefix -l 16 -e ./output -f wav
```

## Supported formats

### Input formats

| Format | Extensions | Platform | Notes |
|--------|------------|----------|-------|
| REX2 | `.rx2` | All | Pure Go parser |
| REX | `.rex` | All | Pure Go parser (legacy) |
| RCY | `.rcy` | All | Pure Go parser (ReCycle document) |
| Renoise XRNI | `.xrni` | All | ZIP container, pure Go parser |
| Ableton Simpler | `.adv` | All | needs `--library-path` for sample resolution |
| Ableton Drum Rack | `.adg` | All | needs `--library-path`; 128 pads max, auto-splits |
| Ableton Live Set | `.als` | All | needs `--library-path` |
| Simpler (legacy) | `.simpler` | All | alias for `.adv` |
| Polyend Tracker | `.pti` | All | pure Go parser |
| Octatrack | `.ot` | All | reads `.ot` sidecar + companion `.wav` |
| OP-XY | `.xy` | All | ZIP container (patch.json + per-slice WAVs) |
| WAV | `.wav` | All | reads cue markers for slices |
| AIFF | `.aif`, `.aiff` | All | reads MARK chunk for slices |
| Apple CAF | `.caf` | All | Apple Loop format, reads beat markers for slices |
| Digitakt II | `.dt2pst` | All | ZIP container (manifest.json + WAV + binary preset) |

### Output formats

| Format | Flag | Extension | Device Limit | Notes |
|--------|------|-----------|--------------|-------|
| WAV | `wav` | `.wav` | — | Sliced WAV with cue markers (Dirtywave M8-compatible) |
| AIFF | `aif` | `.aif` | — | IFF format with MARK chunk |
| AIFF (.aiff) | `aiff` | `.aiff` | — | Same as `aif`, different extension |
| OP-1 AIFF | `aif-op1` | `.aif` | 24 slices | TE OP-1 drum kit with APPL metadata |
| Renoise XRNI | `xrni` | `.xrni` | 128 | ZIP with Instrument.xml + PCM WAV |
| Polyend Tracker | `pti` | `.pti` | 48 | Embedded PCM, auto-splits if >48 |
| Octatrack | `ot` | `.ot` + `.wav` | 64 | Sidecar + companion WAV |
| OP-XY preset | `xy` | `.preset.zip` | 24 | ZIP with patch.json + per-slice WAVs |
| Elektron multi-sample | `el` | `_slices.txt` + `.wav` | 64 | TOML-like config + companion WAV |
| Digitakt II | `dt2pst` | `.dt2pst` | 64 | ZIP with manifest.json + WAV + binary preset |
| Apple Loop CAF | `caf` | `.caf` | — | 44100 Hz only; Apple Loop UUID metadata |
| Ableton ADV | `adv` | `.adv` + `.wav` | — | Simpler XML preset + per-slice WAVs |
| Ableton ALS | `als` | `.als` + `.wav` | — | Live Set XML + per-slice WAVs |
| Ableton ADG | `adg` | `.adg` + WAVs | 128 | Drum Rack XML + per-pad WAVs |

**Note:** REX2 (`.rx2`) output is temporarily disabled. The encoder produces valid IFF structure but ReCycle rejects files due to DWOP compression differences from the original SDK. Use WAV output for sliced audio.

## Format details

### WAV + cue markers

Default `wav` output produces a single WAV with optional `cue ` and `adtl` chunks when the input has slice markers. Tailored for **Dirtywave M8** — a strict WAV parser that rejects unexpected chunks after `data`:

- Written in one sequential pass with pre-computed offsets
- Only `fmt `, optional `cue ` / `adtl`, and `data` chunks
- **No** `LIST`/`INFO` chunks (M8 doesn't handle them)
- `dwChunkStart` / `dwBlockStart` = 0, `fccChunk` = `"data"`, `dwSampleOffset` = frame index
- Other DAWs (Ableton, Logic, Reaper) load the same WAV fine

```bash
chirashi loop.rx2 -o sliced.wav    # WAV with M8-compatible cue markers
chirashi loop.rx2 -n -o flat.wav   # plain WAV without slice markers
```

### AIFF / AIFF-C

Standard IFF-based AIFF with 16-bit big-endian PCM. Writes `COMM` (sample rate, channels, bit depth), `SSND` (audio data), and `MARK` (slice points with Pascal-style string labels). The `MARK` chunk follows the IFF specification exactly — Pascal strings (1-byte length prefix, pad to even). Works in Logic Pro, Ableton Live, and most DAWs.

Two flags: `-f aif` (`.aif` extension) and `-f aiff` (`.aiff` extension). Same format, different extension.

### CAF — Apple Loops

Core Audio Format (`.caf`) with Apple Loop UUID metadata. Designed for **Logic Pro** and **GarageBand**, which detect slice markers from the embedded `appleLoopBeatMarkers` UUID chunk, and tempo/time-signature from the `appleLoopMeta` UUID chunk.

- **44100 Hz only** — Apple Loop format requires this sample rate
- Writes `lpcm` (linear PCM) audio — not ALAC (Apple Lossless), since ALAC encoding requires `afconvert` on macOS
- Beat count derived from `slice count × 60 / (total frames / sample rate)`
- Also writes `info` chunk with genre/subcategory metadata

```bash
chirashi loop.rx2 -s 44100 -f caf -o loop.caf   # Apple Loop for Logic/GarageBand
```

### Ableton Live presets

Ableton presets (`.adv` Simpler, `.adg` Drum Rack, `.als` Live Set) are XML wrappers (gzip-compressed) that reference external WAV samples.

**Input (reading):** Use `--library-path` to point chirashi at your Ableton User Library. Search order:
1. Exact path in the preset (if absolute and exists)
2. `<library-path>/<original-path>`
3. `<library-path>/Samples/Imported/<sample-basename>`
4. `<library-path>/Samples/<sample-basename>`

Without `--library-path`, only the exact path in the preset is tried.

```bash
chirashi simpler.adv --library-path ~/Music/Ableton/User\ Library -o out.wav
```

**Output (writing):** Produces a directory layout matching Ableton's convention. `--sample-path-mode` is reserved for future use — output always uses relative paths.

```
output/
├── kit.adv                # Simpler XML preset
├── kit.als                # or Live Set XML preset
├── kit.wav                # single-slice sample
└── Samples/
    └── Imported/
        └── kit_01.wav     # multi-slice samples
```

**ADG specifics:**
- Up to 128 pads per file, assigned starting at MIDI note 36 (C2)
- Inputs with >128 slices auto-split into multiple `.adg` files
- `-l` requests a smaller chunk size (e.g. `-l 64` = 64 pads per file)
- With `-n` (normalize-splits), output is balanced across the effective chunk count

### Renoise XRNI

ZIP container with `Instrument.xml` + sample WAV(s). The XML contains `<SliceMarker>` elements for slice positions. Reads multi-sample instruments and exports all slices with their markers. Writes a single ZIP with embedded PCM.

**Limits:** 128 max slices per file.

### Polyend Tracker (PTI)

Binary format with 392-byte header + embedded PCM. Slice positions are encoded as 16-bit ratios (0–65535) in a 48-entry table at offset 280, representing the position of each slice as a fraction of total sample length.

No hard slice limit — the PTI format supports up to 48 slice slots. Single-slice files use playback mode 0; multi-slice use mode 2.

### Elektron Octatrack (OT)

Two-file output: a `.wav` companion (standard WAV with cue markers) and a `.ot` sidecar containing slice boundary positions. The sidecar uses `FORM DPS1` IFF-style structure with 64 slice slots, each storing start/end byte offsets and loop flags.

**Input:** Requires both `.ot` and companion `.wav`. chirashi looks for a same-named `.wav` next to the `.ot` file. Use `--input-format ot` if chirashi doesn't auto-detect.

**Limits:** 64 max slices per file.

### Teenage Engineering OP-1

AIFF-based format (`.aif`) with an `APPL` chunk containing OP-1-specific JSON metadata. The JSON encodes drum kit parameters: volume, pan, pitch, start/end positions, play modes, and per-slice envelope settings for up to 24 slices.

Uses 16-bit big-endian PCM (same as AIFF). The `COMM` chunk uses standard 80-bit extended float sample rate encoding.

**Limits:** 24 max slices (OP-1 drum kit limit).

```bash
chirashi loop.rx2 -f aif-op1 -o op1_kit.aif   # OP-1 drum kit
```

### Teenage Engineering OP-XY (preset)

ZIP container (`.preset.zip`) with `patch.json` + per-slice WAV files. The JSON encodes region parameters: sample start/end, pitch, gain, pan, fade in/out, and play mode. Each slice is stored as its own WAV file within the ZIP.

**Limits:** 24 max slices (OP-XY drum kit limit).

```bash
chirashi loop.rx2 -l 24 -f xy -o xy_kit.preset.zip
```

### Elektron multi-sample (EL)

Two-file output: a `.wav` companion and a `_slices.txt` configuration file. The text format (TOML-like) defines key-zones, velocity layers, and sample slot mappings. Each slice is assigned to a sequential MIDI note starting at C1 (note 24).

**Limits:** 64 max slices.

```
# ELEKTRON MULTI-SAMPLE MAPPING FORMAT
version = 0
name = 'chirashi'

[[key-zones]]
pitch = 24
key-center = 24.0

[[key-zones.velocity-layers]]
velocity = 0.49411765
strategy = 'Forward'

[[key-zones.velocity-layers.sample-slots]]
sample = 'slice_01.wav'
```

### Elektron Digitakt II (DT2PST)

ZIP container (`.dt2pst`) with `manifest.json` + sample WAV + binary preset. Slice positions are embedded in the binary preset as 8-byte entries (`0x00 0x22 <uint32 LE position> 0x00 0x08`). The manifest stores the WAV path, payload name, and CRC32 hash of the PCM audio.

**Limits:** 64 max slices. Payload name limited to 12 characters.

```bash
chirashi loop.rx2 -f dt2pst -o kit.dt2pst
```

### REX / RX2 / RCY

REX formats are parsed entirely in pure Go (`internal/engine/rex2/`). No external SDK required.

- **Input only** — REX2 output is temporarily disabled (see note below)
- **All platforms supported** — no native SDK dependencies
- Reads slice markers, tempo, sample rate, bit depth, and creator metadata
- Supports 8/16/24/32-bit audio, mono and stereo

#### How the REX decoder works

The `rex2/` package implements a complete REX2 parser in pure Go:

1. **IFF chunk parsing**: REX2 is an IFF-based format. The decoder reads chunks sequentially:
   - `CAT REX2` — file container
   - `HEAD` — file header with format version
   - `CREI` — creator info (name, copyright, URL, email)
   - `TRSH` — transient sensitivity settings
   - `SINF` — sample info (sample rate, bit depth, channels)
   - `GLOB` — global data (BPM, time signature, grid)
   - `SLCE` — slice table (PPQ positions, mute/locked states)
   - `SDAT` — DWOP-compressed audio data

2. **DWOP decompression**: The audio data uses DPCM (differential pulse-code modulation) with variable-length bit stuffing:
   - Stereo: left channel stored directly, right channel as delta from left
   - Predictor state maintained per-channel for decode
   - Variable-length codes (1-5 bits per sample) based on magnitude

3. **Slice filtering**: SLCE entries have visibility flags (normal, muted, locked). `isVisibleSliceBoundary()` filters based on state:
   - State 0 (normal): visible
   - State 1 (muted): hidden from `isVisibleSliceBoundary` but still counted in slice count
   - State 2 (locked): visible

4. **REX1 detection**: REX1 (`CAT REX\x01`) uses inline sample positions without SLCE. Returns error (no REX1 write support yet).

```bash
chirashi loop.rx2 -s 44100 -b 16 -o loop.wav      # REX2 → WAV
chirashi loop.rex -f pti -o rex_kit.pti            # REX → Polyend Tracker
```

**Note:** REX2 output is temporarily disabled. The encoder produces valid IFF structure (GLOB, SLCE, SDAT chunks) but ReCycle rejects files due to DWOP compression producing different byte-for-byte output than the original SDK (~600 bytes size difference). Internal roundtrip (encode→decode) passes PCM validation. Use WAV output for sliced audio that works in ReCycle.

## Architecture

```
                   ┌─────────────────┐
                   │   cmd/root.go   │  CLI flags, validation
                   └────────┬────────┘
                            │
                   ┌────────▼────────┐
                   │ internal/engine │  Go pipeline
                   │   runner.go     │
                   └────────┬────────┘
                            │
         ┌──────────────────┼──────────────────┐
         │                  │                  │
   ┌─────▼──────┐   ┌──────▼──────┐   ┌───────▼──────┐
   │  readers/  │   │    bridge    │   │   encoders/  │
   │            │   │  (rex2.go)  │   │             │
   │  .xrni     │   │             │   │   .wav      │
   │  .adv      │   │  DecodeREX2 │   │   .aif      │
   │  .adg      │   │  EncodeREX2 │   │   .aiff     │
   │  .als      │   │             │   │   .caf      │
   │  .aif      │   │             │   │   .aif-op1  │
   │  .caf      │   └─────────────┘   │   .pti      │
   │  .wav      │                     │   .ot       │
   │  .pti      │                     │   .xy       │
   │  .ot       │                     │   .el       │
   │  .xy       │                     │   .dt2pst   │
   │  .dt2pst   │                     │   .xrni     │
   │  .rex*     │                     │   .adv      │
   │  .rx2      │                     │   .als      │
   │  .rcy      │                     │   .adg      │
   └────────────┘                     └─────────────┘

   * .rex/.rx2/.rcy handled by internal/engine/rex2/ package
```

- **cmd/root.go** — cobra CLI, flag validation, pipeline orchestration
- **internal/engine/runner.go** — converts one file at a time, manages concurrency
- **internal/engine/readers/** — format-specific input parsers (pure Go)
- **internal/engine/rex2/** — pure Go REX2 parser and encoder (DWOP compression)
- **internal/engine/encoders/** — format-specific output writers

## Building from source

```bash
git clone https://github.com/g-lok/chirashi.git
cd chirashi
go build -o chirashi .
```

No external dependencies. Builds on macOS, Linux, and Windows with `CGO_ENABLED=0`.

### Test

```bash
go test ./...
```

## Development

```bash
go build -o chirashi .   # build binary
go test ./...            # run test suite
```

### Test data

Test fixtures in `tests/testdata/`. Reference PCM data from the REX SDK in `tests/testdata/Slice_*.txt`.

### Adding a new output format

1. Add `encoder_<format>.go` with `Encode<Format>(w, extraction, cfg) error`
2. Register format in `runner.go` (`writeOutputFiles` switch)
3. Add CLI flag value in `cmd/root.go` (`outputFormat` choices)
4. Add tests in `tests/encoder_format_test.go` and `internal/engine/encoder_output_test.go`

### Adding a new input format

1. Add `reader_<format>.go` implementing the `InputReader` interface
2. Register in `reader.go` (`RegisterReader`)
3. Add extension mapping in `runner.go` (`inputExtensions`)
4. Add tests in `tests/reader_test.go`

## License

chirashi is licensed under the terms in `LICENSE`.
