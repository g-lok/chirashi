# chirashi

![chirashi logo](assets/chirashi.png)

**chirashi** is a CLI tool for converting between sliced instrument formats used in hardware samplers and DAWs. Convert freely between Renoise, Ableton, Polyend Tracker, Octatrack, OP-1, OP-XY, Elektron multi-sample, Digitakt II, WAV, and AIFF — plus read REX/RX2/RCY (macOS/Windows only).

- **Cross-format conversion** between all supported input and output formats
- **REX/RX2/RCY** input via the Reason REX SDK (macOS/Windows only)
- Reads XRNI, ADV, ADG, ALS, Simpler, Drum Rack, AIFF, PTI, OT, WAV
- Writes WAV, AIFF, PTI, OT, OP-1 AIFF, OP-XY, Elektron, DT2, XRNI, Simpler, ADV, ADG
- Mono downmix, resampling, slice limiting, BPM override

## Platform support

| Platform | All formats | REX input |
|----------|-------------|-----------|
| macOS    | ✓           | ✓         |
| Windows  | ✓           | ✓         |
| Linux    | ✓           | ✗ (REX SDK not available) |

REX (`.rex`, `.rx2`, `.rcy`) is **input only** and requires the proprietary Reason REX SDK, which only ships for macOS and Windows. All other formats work cross-platform.

## Installation

Pick your platform:

### macOS / Linux (Homebrew)

```bash
brew install g-lok/tap/chirashi
```

The Homebrew formula in [`g-lok/homebrew-tap`](https://github.com/g-lok/homebrew-tap)
installs a universal macOS binary (with the REX Shared Library framework bundled) on
macOS, and the prebuilt Linux amd64/arm64 binary on Linux. REX input is not available
on Linux (the SDK is macOS/Windows only).

### Windows (Scoop)

```powershell
scoop bucket add g-lok https://github.com/g-lok/scoop-bucket
scoop install chirashi
```

The Scoop manifest in [`g-lok/scoop-bucket`](https://github.com/g-lok/scoop-bucket)
installs `chirashi.exe` plus the bundled `REX Shared Library.dll`.

### From source

chirashi uses [mise](https://mise.jdx.dev/) for tool versioning.

```bash
git clone https://github.com/g-lok/chirashi.git
cd chirashi
mise install         # installs go 1.26.3 + zig 0.16.0
```

### REX SDK setup (macOS / Windows builds only)

The Reason Studios REX SDK is proprietary and **not included** in this repository. The CI uses a GPG-encrypted tarball with a private key stored as a GitHub secret — this is for the project's automated builds. For local development, you'll need to obtain the SDK from Reason directly (it ships with ReCycle, Reason Studio, or the REX SDK download).

Once you have the REX SDK, the build expects it at:

```
<REX_SDK>/REXSDK_Mac_1.9.2/Mac/Deployment/REX Shared Library.framework
```

The `mise run build` task copies this from `$REX_SDK` (default `/tmp/rexlibs`) into the build tree. If your SDK lives elsewhere, set the `REX_SDK` env var to its parent directory:

```bash
export REX_SDK=/path/to/your/rex-sdk-root
```

For example, if your framework is at `/Users/me/REXSDK/REXSDK_Mac_1.9.2/Mac/Deployment/REX Shared Library.framework`, set `REX_SDK=/Users/me/REXSDK`.

**Note:** chirashi's own REX tarball encryption keys are for CI use only and not shared with end users. Don't expect to be able to decrypt `.github/workflows/secrets/rex-sdk-*.tar.gz.gpg` — you need the SDK from Reason.

### Build

```bash
mise run build       # macOS binary (requires REX SDK at $REX_SDK)
mise run build-linux # Linux binaries (amd64, arm64, armv7) — no REX support
```

The macOS build produces `build/chirashi` plus a `build/Frameworks/REX Shared Library.framework/` directory (the embedded REX framework, with the binary's rpath patched to find it). Ship both together.

The Linux build produces `build/chirashi-linux-amd64`, `build/chirashi-linux-arm64`, and `build/chirashi-linux-arm`. REX/RX2/RCY input is disabled on these binaries (the SDK is macOS/Windows only).

### Test

```bash
mise run test          # build + run Go test suite (macOS, includes REX tests)
mise run test-linux    # build + test Linux binaries (REX tests skipped)
```

If you don't have the REX SDK, you can still run the Go test suite with `CGO_ENABLED=0`:

```bash
CGO_ENABLED=0 go test ./tests/...
```

## Usage

```bash
chirashi [INPUT_FILES...] [flags]
```

### Examples

Convert a Renoise XRNI instrument to a Polyend Tracker project:

```bash
chirashi kit.xrni -f pti -o pt_kit.pti
```

Convert a Polyend Tracker instrument to multiple Octatrack-ready WAVs:

```bash
chirashi kit.pti -s 44100 -b 16 -e ./ot_output -f wav
```

Convert an Ableton Simpler preset to mono 24-bit WAV:

```bash
chirashi simpler.adv -m -b 24 -o output.wav
```

Convert an Ableton Drum Rack with samples from the standard Library:

```bash
chirashi rack.adg --library-path ~/Music/Ableton/User\ Library -e ./output
```

Convert a REX2 file to a standard WAV with slice markers (macOS/Windows):

```bash
chirashi loop.rx2 -s 44100 -b 16 -o loop.wav
```

Convert a REX2 to OP-XY preset with 64 slices per file:

```bash
chirashi loop.rx2 -l 64 -f xy -o xy_loop.wav
```

Batch convert a directory of REX files to Ableton Drum Rack:

```bash
chirashi -d ./rex_files -e ./adg_output -f adg
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--input-file` | `-i` | — | Input file(s) (repeatable) |
| `--input-dir` | `-d` | — | Scan directory for input files |
| `--output-file` | `-o` | — | Output path (single input only) |
| `--output-dir` | `-e` | — | Output directory for batch |
| `--format` | `-f` | `wav` | Output format (see below) |
| `--bit-rate` | `-b` | 16 | Bit depth: 8, 16, or 24 |
| `--sample-rate` | `-s` | source | Output sample rate in Hz (11k–1M) |
| `--mono` | `-m` | false | Downmix to mono |
| `--mono-mode` | — | `sum` | `sum`, `left`, `right`, `difference`, `dual-detect` |
| `--tempo` | `-t` | 0 | Override tempo in BPM (0 = use original) |
| `--slice-limit` | `-l` | 0 | Max slices per output file |
| `--no-slices` | `-n` | false | Ignore slice markers, render plain output |
| `--recursive` | `-r` | false | Recurse subdirs (with `--input-dir`) |
| `--preserve` | `-p` | false | Preserve directory structure (with `--input-dir`) |
| `--quiet` | `-q` | false | Suppress progress |
| `--verbose` | `-v` | false | Debug output (Zig struct diagnostics) |
| `--library-path` | — | — | Ableton User Library path |
| `--input-format` | — | auto | Force input format (auto-detect by extension) |
| `--sample-path-mode` | — | `relative` | Sample path style in XML: `relative`, `absolute`, `library` |

### Supported formats

#### Input formats

| Format | Extensions | Platform | Notes |
|--------|------------|----------|-------|
| REX2 | `.rx2` | macOS/Win | via REX SDK |
| REX | `.rex` | macOS/Win | via REX SDK (legacy) |
| RCY | `.rcy` | macOS/Win | via REX SDK (ReCycle doc) |
| Renoise XRNI | `.xrni` | All | pure Go parser |
| Ableton Simpler | `.adv` | All | needs `--library-path` to find samples (see [Ableton formats](#ableton-formats)) |
| Ableton Drum Rack | `.adg` | All | needs `--library-path` to find samples; 128 pads per file, auto-splits if exceeded |
| Ableton Live Set | `.als` | All | needs `--library-path` to find samples |
| Simpler | `.simpler` | All | alias for `.adv` |
| Polyend Tracker | `.pti` | All | pure Go parser |
| Octatrack | `.ot` | All | reads .ot + companion .wav |
| OP-XY | `.xy` | All | pure Go parser |
| WAV | `.wav` | All | pure Go parser (with cue markers) |
| AIFF | `.aif`, `.aiff` | All | pure Go parser |

#### Output formats

| Format | Flag | Notes |
|--------|------|-------|
| WAV | `wav` | Standard WAV with optional cue markers (see [WAV + cue markers](#wav--cue-markers) below) |
| AIFF | `aif` | Standard AIFF |
| OP-1 AIFF | `aif-op1` | Teenage Engineering OP-1 format |
| Polyend Tracker | `pti` | Polyend Tracker instrument |
| Octatrack | `ot` | Elektron Octatrack .ot preset + companion WAV |
| OP-XY preset | `xy` | Teenage Engineering OP-XY preset |
| Elektron multi-sample | `el` | Elektron multi-sample text format |
| Digitakt II | `d2pst` | Elektron Digitakt II preset |
| Renoise XRNI | `xrni` | Renoise instrument |
| Simpler | `simpler` | Ableton Simpler (via .adv wrapper) |
| Ableton ADV | `adv` | Ableton Sampler preset |
| Ableton ADG | `adg` | Ableton Drum Rack |
| Ableton ALS | `als` | Ableton Live Set (uses library path) |

## WAV + cue markers

The default `wav` output format produces a single WAV file with optional `cue ` and `adtl` chunks when the input has slice markers. The output is tailored to be compatible with the **Dirtywave M8** tracker — a strict WAV parser that rejects unexpected chunks after `data`.

Specifically:
- The WAV is written in one sequential pass with pre-computed offsets
- Only `fmt `, optional `cue `/`adtl`, and `data` chunks are emitted
- **No** `LIST`/`INFO` chunks (the M8 doesn't handle them)
- Cue marker `dwChunkStart` and `dwBlockStart` are set to 0 (M8-compatible)
- Cue marker `fccChunk` is always `"data"`
- `dwSampleOffset` is the frame index in the data chunk

For other DAWs (Ableton, Logic, Reaper) the same WAV loads fine — they're more lenient parsers. The M8 is the strictest consumer, so we optimize for it.

Use `--no-slices` (`-n`) to skip the cue chunks and render plain unsliced audio:

```bash
chirashi loop.rx2 -n -o flat.wav   # WAV without slice markers
chirashi loop.rx2 -o sliced.wav    # WAV with M8-compatible cue markers (default)
```

## Ableton formats

Ableton presets (`.adv` Simpler, `.adg` Drum Rack, `.als` Live Set) are XML wrappers that reference external WAV samples. The relationship between chirashi and the Ableton Library is:

### Input (reading Ableton presets)

When reading an `.adv`/`.adg`/`.als` file, chirashi must find the referenced sample WAVs. Ableton stores these under the **User Library** (typically `~/Music/Ableton/User Library/Samples/Imported/`). Use `--library-path` to point chirashi at it:

```bash
chirashi simpler.adv --library-path ~/Music/Ableton/User\ Library -o out.wav
```

With `--library-path` set, chirashi searches in this order:
1. The exact path in the preset (if absolute and exists)
2. `<library-path>/<original-path>`
3. `<library-path>/Samples/Imported/<sample-basename>`
4. `<library-path>/Samples/<sample-basename>`

Without `--library-path`, only the exact path in the preset is tried. Presets created on another machine typically have absolute paths that don't resolve on yours — that's when `--library-path` is essential.

### Output (writing Ableton presets)

When writing `.adv`/`.als`, chirashi produces a directory layout that matches Ableton's convention:

```
output/
├── kit.adv                # the preset (Simpler/ALS)
├── kit.wav                # the sample (when single-slice)
└── Samples/
    └── Imported/
        └── kit_01.wav, kit_02.wav, ...   # when multiple slices
```

The preset's `Path` and `RelativePath` XML attributes point to `Samples/Imported/<name>.wav` (relative to the preset location). This is the standard Ableton layout — drop the `output/` directory into your User Library and the preset will resolve its samples.

**Note:** `--sample-path-mode` is currently a no-op. The output always uses relative paths (`Samples/Imported/<name>.wav`). The flag is reserved for future absolute/library-path output modes.

### ADG (Drum Rack) specifics

Drum Racks support up to 128 pads. Pads are assigned starting at MIDI note 36 (C2) and incrementing.

If your input has more than 128 slices, chirashi automatically splits the output into multiple Drum Rack files. With 200 slices, you get 2 `.adg` files (100 + 100 pads). With 500 slices, 4 files (~128 each).

`-l` lets you request a *smaller* chunk size (e.g. `-l 64` for 64 pads per file). If you pass a value larger than the 128-pad device limit, chirashi ignores it and uses 128 instead — even with `-n`. Better to produce a few split files than one file with truncated slices.

With `-n` (normalize-splits) and `-l` set, the output is balanced across the effective chunk count. If your requested `-l` is above the device limit, it gets clamped and the output is balanced across however many files that produces.

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
              ┌──────────────┼──────────────┐
              │              │              │
       ┌──────▼──────┐ ┌─────▼─────┐ ┌──────▼──────┐
       │   readers/  │ │ extractor │ │  encoders/  │
       │ .xrni .adv  │ │   .zig    │ │ .wav .pti   │
       │ .adg .aiff  │ │  (REX SDK)│ │ .ot .xy .el │
       │ .oti .pti   │ │           │ │ .d2pst etc  │
       └─────────────┘ └─────┬─────┘ └─────────────┘
                            │ CGo
                     ┌──────▼──────┐
                     │  REX SDK    │  (macOS framework, Windows DLL)
                     │  v1.9.2     │  (proprietary)
                     └─────────────┘
```

- **cmd/root.go** — cobra CLI, flag validation, pipeline orchestration
- **internal/engine/runner.go** — converts one file at a time, manages concurrency
- **internal/engine/readers/** — format-specific input parsers (REX via Zig, others pure Go)
- **internal/engine/encoders/** — format-specific output writers
- **internal/engine/extractor.zig** — Zig wrapper around the REX C API
- **internal/engine/rex_bindings.zig** — manual extern declarations for REX SDK
- **internal/engine/rex/REX.c** — Windows DLL loader

## Development

```bash
mise run build         # build macOS binary
mise run test          # build + run Go test suite
mise run test-linux    # build + test Linux binaries
mise run graphify      # generate knowledge graph
```

### Test data

Test fixtures are in `tests/testdata/`. Reference PCM data from the REX SDK is in `tests/testdata/Slice_*.txt`.

### Adding a new output format

1. Add `encoder_<format>.go` with `Encode<Format>(w, extraction, cfg) error`
2. Register the format in `runner.go` (`writeOutputFiles` switch)
3. Add a CLI flag value in `cmd/root.go` (`outputFormat` choices)
4. Add a test in `tests/encoder_format_test.go`

### Adding a new input format

1. Add `reader_<format>.go` with a `Reader` implementing the `Reader` interface
2. Register in `reader.go` extension dispatch table
3. Add a test in `tests/reader_test.go`

## CI setup

> **For repo maintainers only.** This section documents how the project's automated
> CI builds work. Local development does not need this — see
> [Installation](#installation) above for the regular build path.

chirashi's CI builds the binary and runs the test suite on every push/PR. The CI
uses a GPG-encrypted REX SDK tarball to avoid committing the proprietary SDK binaries.

### Required secrets

- `GPG_SIGNING_KEY` — private GPG key (no passphrase, RSA 4096). Used to decrypt the REX SDK tarball.

### Encrypting the REX SDK

```bash
# One-time setup: generate a keypair
gpg --quick-generate-key "ci@example.com" default default 0

# Export the public key
gpg --export --armor ci@example.com > .github/workflows/secrets/chirashi-ci-gpg-public.key

# Add to GitHub
gh secret set GPG_SIGNING_KEY < private-key.asc

# Encrypt the SDK
gpg --encrypt --batch --recipient ci@example.com \
    --output .github/workflows/secrets/rex-sdk-macos.tar.gz.gpg \
    rex-sdk-macos.tar.gz
```

## License

chirashi is licensed under the terms in `LICENSE`. The Reason Studios REX SDK bundled with this repository is licensed separately — see `REX_SDK_LICENSE.txt`.
