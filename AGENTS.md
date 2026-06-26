# AGENTS.md — chirashi agent instructions

## Project overview

Pure Go CLI for converting sliced instrument formats. Parses/encodes REX/RX2/RCY in-process (no SDK, no CGo, no Zig).

- **Go**: cmd/, internal/engine/, internal/engine/rex2/, tests/
- **No Zig, no C, no CGo, no REX SDK**

REX2 pure Go implementation: `internal/engine/rex2/` — IFF parser, DWOP decoder/encoder, slice model, legacy PTI/OT fallback, REX1 detection.

## History / Why Pure Go

### REX SDK → Pure Go migration (v0.4.0)

**Why we abandoned the REX SDK:**
- **macOS Intel + Zig dynlb issue**: Zig 0.16.0's dynamic library binding (`dynlb`) crashed on macOS Intel when loading the REX framework via CGo. The crash happened deep in Zig's linker during module initialization — not fixable without upstream Zig changes.
- **v26 breaking change**: Zig 0.16.x broke the CGo calling convention in a way that affected `cgo(callback)` handling. The project was stuck on Zig 0.15.x which had different APIs.
- **macOS framework complexity**: Embedding the REX Shared Library.framework required rpath patching, framework bundling, and different handling per architecture (Apple Silicon vs Intel).
- **Windows DLL loader issues**: The REX.c dynamic loader had path resolution issues on Windows.
- **CI complexity**: Required GPG-encrypted tarballs of the SDK, GPG keys in secrets, and decryption steps in GitHub Actions.

**Solution**: Implemented REX2 format decoding/encoding entirely in Go:
- `rex2/reader.go`: IFF chunk parser, handles CAT REX2, HEAD, CREI, TRSH, SINF, GLOB, SLCE, SDAT
- `rex2/dwop.go`: DPCM decoder with predictor state per channel, variable-length bit stuffing
- `rex2/encoder.go`: Produces valid CAT REX2 files with DWOP compression
- `rex2/legacy.go`: PTI and OT legacy format readers (same SliceInfo output)

**REX2 encoder status (v0.5.0):**
- Produces valid IFF structure (22-byte GLOB matches original)
- DWOP compression implemented but produces slightly different output than original SDK
- ReCycle rejects roundtrip files despite correct IFF structure
- **RX2 OUTPUT DISABLED** — use WAV output for now
- Decoder works correctly for input

## Key constraints

### REX2 Encoder issues (known)
- DWOP compression produces different byte-for-byte output than original SDK
- SDAT size differs from original by ~600 bytes for typical files
- ReCycle validation fails even though IFF structure is correct
- Internal roundtrip (encode→decode) passes PCM validation
- Root cause likely in DWOP predictor state or bit-stuffing implementation

### REX2 decoder details
- `rex2/types.go`: FileInfo, SliceInfo, CreatorInfo, REX2File structs. PTR stubs (format uses 4-byte PTR/length, not 8-byte offset for SDAT chunks, which differ from many docs).
- `rex2/reader.go`: Decode reads CAT REX2 → iterates chunks. Handles HEAD (glob+slice metadata), CREI (creator name/copyright/url/email/free text), TRSH (transient sensitivity/decay/freeze), SINF (sample info), GLOB (BPM, grids), SLCE (slice boundaries+ppq), SDAT (DWOP compressed audio). Raw PCM validated via `MTHD` frame count vs actual decoded frames.
- `rex2/dwop.go`: DWOP decoder — DPCM with predictor state per channel (L + delta for stereo). Supports 8/16/24/32-bit. Bit stuffing: variable-length code per sample, states 0-4 determine encoding length.
- `rex2/encoder.go` (`rex2.Encode`): Produces CAT REX2 with HEAD, CREI, SINF, GLOB, SLCE, SDAT (DWOP). Stereo encodes as L + delta channel. Bit depth from FileInfo.BitDepth. Predictor state per channel.
- `rex2/legacy.go`: PTI (`TI\x01`/`PTI\x00`) and OT (`FORM...DPS1`/`OT\x00\x00`) readers for legacy ReCycle formats. Returns same SliceInfo slice.
- `bridge.go`: Adapter between rex2 package and engine. DecodeREX2 → SliceExtraction (PCM float32, cue markers, metadata). Tempo/OriginalTempo divided by 1000 (REX2 stores BPM*1000). CREI + TRSH mapped to RexMetadata. Pure Go, no CGo, no build tags.
- **Edge cases**: Single-slice files (no SLCE chunk). RCY has no tempo (REX2 has all zeros). 8-bit audio less common.
- **REX1 detection**: REX1 format uses `CAT REX\x01` (no SLCE, inline sample positions). rex2 package detects and returns error (no REX1 write support yet).

### CLI --bpm-prefix
- `--bpm-prefix` (bool): prepend detected BPM to output filename
- BPM resolution chain: metadata OriginalTempo → Tempo → filename `_NNNbpm`/`NNN` prefix → `--tempo` override
- `--tempo` conflict (>0.5 BPM diff from metadata) → use metadata, print warning
- `-o` without `-l`: skip prefix silently, print warning
- `-o` with `-l`: prefix applied to each chunk
- BPM formatted as integer or 1-decimal float, trailing `.0` stripped

### CLI -o semantics
- When `-o` is set WITHOUT `-l` (slice limit): use the path as-is, no sanitization, no suffix
- When `-o` is set WITH `-l`: treat the path as a base name pattern, append chunk suffixes (_01, _02, ...)
- `cmd/root.go` restricts `-o` to single-input mode

### Format-specific constraints
- **AIFF MARK**: Pascal strings (1-byte length prefix), padded to even boundary per IFF spec
- **AIFF MARK default label**: markSize calc and actual write MUST use same default (currently `fmt.Sprintf("S%02d", i+1)`). Using different defaults ("X" vs "S%02d") produces wrong chunk size → data corruption
- **AIFF sample rate**: 80-bit extended float with `ldexp(mantissa, exponent-16383-63)` formula
- **AIFF encoder**: only supports 16-bit PCM. default case falls back to 16-bit silently
- **CAF encoder**: `writeCAFPCM` now handles 8/16/24-bit via switch. `Force44100Spec` forces 16-bit before encoding, but if bitDepth differs, function respects it
- **CAF**: Linux uses `lpcm` PCM (no afconvert), metadata via standard UUID chunks
- **D2PST**: slice positions embedded in binary payload as `00 22 <uint32 LE pos> 00 08` patterns (no TLV file)
- **PTI readers**: dual-format dispatch — `TI\x01` (encoder, 280–375 ratio table) and `PTI\x00` (legacy)
- **OT readers**: dual-format dispatch — `FORM...DPS1` (encoder, IFF structure, checksum at 0x33E) and `OT\x00\x00` (legacy)
- **PTI limit**: hardware max 48 slices, runner auto-splits via groupSlices
- **OT bit depth**: encoder respects user bit depth. OT supports 16 and 24-bit (24-bit DAC/ADC, user selects FLEX FORMAT)
- **Scale factor**: WAV/OP-1/PTI use 32767.0; AIFF/CAF use 32768.0 (both correct per spec)
- **Simpler ADV**: SlicingRegions value is always 2 (start+end per part)
- **EncodeWavContainer**: only DOWNGRADES bit depth (when targetBitDepth < extraction.BitDepth). Never upgrades. To force higher bit depth, set extraction metadata + pass matching targetBitDepth (or 0 to skip check)
- **REX2 output (.rx2)**: DISABLED. The encoder produces valid IFF but ReCycle rejects files due to DWOP compression differences. Use WAV output.

## Build system

```bash
CGO_ENABLED=0 go build -o chirashi .
```

Single binary, no deps, works on macOS/Linux/Windows for ALL formats including REX/RX2/RCY.

### main.go entry point
Single `main.go` with `main() { cmd.Execute() }`. No build tags, no CGo, no Zig archive.

## Distribution / Release packaging

Release pipeline (.github/workflows/release.yml) triggered by `v*` tag. Pure Go cross-compile — no Zig, no REX SDK, no frameworks.

### All platforms
- `go build -ldflags="-s -w"` for all OS/arch combos
- No framework bundling, no DLLs, no rpath patching
- Static binaries on Linux, self-contained on macOS/Windows

### Archive
- `chirashi-$VERSION-$OS-$ARCH.tar.gz` / `.zip`
- Single binary, extract anywhere

### Release body
- CHECKSUMS.txt
- Single binary, no quarantine issues (no frameworks)

## Testing

- `tests/integration_test.go` — uses pre-built binary via exec.Command
- `tests/encoder_format_test.go` — tests encoders with the binary
- `tests/reader_test.go` — tests readers in-process (no binary needed)
- `tests/processor_test.go` — slice processing unit tests
- `samples/` — real-world test files fallback when `testdata/` missing
- Test helpers (`findTestXRNI`, `findTestSimpler`, `findTestDrumRack`) search `testdata/` first, then `samples/`

Run with `go test ./...` — no CGo, no zig, no special flags.

### REX2 tests needed (BLOCKED)
- Round-trip: encode known slices → decode → verify PCM identical
- Real .rx2 file: decode sample file → verify slices + PCM match expected
- Legacy: PTI/OT format parsing
- Edge: single-slice, stereo, various bit depths, RCY
- DWOP: test individual state transitions, bit stuffing
- **NOTE**: REX2 encoder produces incorrect DWOP output — needs debugging before re-enabling

## Workflow

- User uses `jj` (Jujutsu), not raw git
- `jj describe -m "msg"` updates the working copy commit
- `jj git push --bookmark <name>` pushes to remote
- `jj bookmark set <name> -r @` to move a bookmark
- `jj abandon <rev>` to discard a rev
- `jj rebase -d <dest>` to rebase working copy

## Local tooling

- `mise` for go versioning (`mise.toml`) — Zig no longer needed
- `poetry` for the graphify project (`pyproject.toml`, `.venv/`)
- `graphify` for codebase knowledge graph
- `gh` for GitHub operations (PRs, CI runs)

## Don't

- Don't run `apt install`, `brew install`, or system-wide `pip install` — use local workspaces
- Don't commit `a.out`, `build/`, root `rexconverter`, `internal/engine/libs/`, `internal/engine/Frameworks/` — all in `.gitignore`
- Don't add `bin/` to git — gitignore blocks it
- Don't modify the squashed PR #1 commit (`45c4f3b`) — it's immutable
