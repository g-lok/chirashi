# Changelog

All notable changes to chirashi will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.0] - 2026-06-26

### Added

- **Pure Go REX2 implementation**: Decodes and encodes REX/RX2/RCY entirely in Go
  with no external dependencies. The `internal/engine/rex2/` package implements:
  - IFF chunk parser (CAT REX2, HEAD, CREI, TRSH, SINF, GLOB, SLCE, SDAT)
  - DWOP compression decoder with per-channel predictor state and variable-length bit stuffing
  - DWOP compression encoder for producing valid REX2 files
  - Legacy PTI and OT format readers
  - REX1 detection (returns error, no write support yet)

- **`--bpm-prefix` flag**: Prepends detected BPM to output filename (e.g.,
  `128-SourceName.wav`). BPM resolved from: metadata (REX OriginalTempo/Tempo,
  CAF Apple Loop beat count) → filename patterns (`_NNNbpm` suffix, `NNN` numeric
  prefix) → `--tempo` override.

### Changed

- **Build system simplified**: No more Zig, CGo, or REX SDK. Single `CGO_ENABLED=0 go build`
  produces a working binary for all platforms.
- **Linux support complete**: REX/RX2/RCY input now works on Linux (was blocked
  by REX SDK unavailability).

### Fixed

- **GLOB chunk layout**: `reserved` field is 2 bytes (not 4), `silenceSelected`
  is 2 bytes (not 1). GLOB chunk now correctly 22 bytes to match original.
- **Tempo clamping**: REX format requires 20-450 BPM. Tempo is now clamped to
  this range in the encoder. Values below 20 BPM → 20 BPM; above 450 BPM → 450 BPM.
- **Progress message BPM fallback**: `Tempo` field now displayed when
  `OriginalTempo` is 0 (previously showed `0.0 BPM` for non-REX/CAF sources).

### Known Issues

- **REX2 encoder DWOP compression**: Produces valid IFF structure but ReCycle
  rejects roundtrip files. SDAT size differs from original by ~600 bytes.
  Internal decode→encode→decode passes PCM validation. **RX2 output is
  temporarily disabled** — use WAV output for now.

### Removed

- **REX SDK dependency**: Zig bindings, CGo wrapper, and framework/DLL loading
  code removed. No more GPG-encrypted SDK tarballs in CI.

## [0.4.0] - 2026-06-25

### Added

- **`--bpm-prefix` flag**: Prepends detected BPM to output filename (e.g.,
  `128-SourceName.wav`). BPM resolved from: metadata (REX SDK `OriginalTempo` /
  `Tempo`, CAF Apple Loop beat count) → filename patterns (`_NNNbpm` suffix,
  `NNN` numeric prefix) → `--tempo` override.

### Fixed

- **Progress message BPM fallback**: `Tempo` field now displayed when
  `OriginalTempo` is 0 (previously showed `0.0 BPM` for non-REX/CAF sources).

## [0.3.1] - 2026-06-24

### Fixed

- **OT bit depth**: `ForceOTSpec()` was incorrectly forced to 24-bit. Reverted —
  OT supports both 16 and 24-bit via user's FLEX FORMAT setting. Encoder now
  respects user-specified bit depth with floor clamp at 16 (8-bit → 16).
- **AIFF MARK chunk size mismatch**: `markSize` calculation used `"X"` (1 byte)
  as default empty-label but actual write used `"S%02d"` (3+ bytes). Chunk size
  underreported by 2 bytes per empty-label marker → IFF data corruption.
- **CAF encoder hardcoded 16-bit**: `writeCAFPCM()` always wrote int16 samples
  regardless of `bitDepth` parameter. Rewritten with switch for 8/16/24-bit.
- **AIFF encoder only 16-bit**: `default` case in bitDepth switch silently wrote
  zero-byte PCM for 8/24-bit input. Now falls back to 16-bit.
- **`outputBaseName` path truncation with `-o` + `-l`**: truncated full path
  including directory separators when `nameLimit` applied. Now uses
  `filepath.Base()` before truncation, preserving directory structure.

## [0.3.0] - 2026-06-23

### Added

- **CAF encoder+reader**: writes Apple CAF with `lpcm` PCM + Apple Loop metadata
  UUIDs (beat count, time sig, descriptors) + beat markers UUID for slice
  positions. Reader reconstructs slices from beat markers. Self-registers via
  `init()`.
- **PTI reader dual-format**: dispatches on magic — `TI\x01` (encoder format,
  392-byte header, ratio table uint16 LE → frame pos) and `PTI\x00` (legacy
  format). `Probe()` accepts both.
- **OT reader dual-format**: dispatches on magic — `FORM...DPS1` (encoder
  format, IFF structure, 64×12-byte slice slots at offset 58, checksum at
  0x33E) and `OT\x00\x00` (legacy format). `Probe()` accepts both.
- **D2PST reader**: rewritten from stub to parse preset binary payload
  (`0x00 0x22 <uint32 LE pos> 0x00 0x08` patterns). No more non-existent .tlv
  file dependency.
- **samples/** directory with real-world test files: XRNI, D2PST, ADV/ADG
  drum rack kits, OT sample.ot
- Test helpers fallback to `samples/` when `testdata/` missing.
- All missing reader tests: D2PST roundtrip, XY roundtrip, OP-1 basic, EL
  basic, PTI basic, OT basic, CAF minimal reader, OT reader (ReadOTWithWAV),
  PTI TI-format roundtrip (4 slices + no-slice fallback), OT DPS1 roundtrip,
  OT/PTI Probe dual-format acceptance, DeviceMaxSlices PTI=48 verification.
- `simpler` format flag aliased to `adv` (was silently writing WAV).

### Changed

- **README.md rewritten**: TOC, accurate format details per row (Pascal
  strings, 80-bit float sample rate, 48-slice PTI limit, Apple Loop UUID docs),
  architecture diagram lists all 11 readers + 12 encoders, build/dev at bottom.
- **PTI device limit**: `deviceMaxSlices["pti"]` set to 48; runner auto-splits
  via `groupSlices()` when exceeded.
- AIFF MARK chunk uses Pascal strings (1-byte length prefix) per IFF spec.
- AIFF 80-bit extended float uses `ldexp(mantissa, exponent-16383-63)`.
- EL encoder uses `errWriter` wrapper for write error propagation.
- Full format audit: all 10 readers + 12 encoders verified for sample+slice
  correctness.

### Fixed

- **AIFF MARK corruption**: was writing null-terminated C strings + wrong
  `markSize` (`2 + numMarkers*8 + numMarkers*2`) → Pascal strings + correct
  formula `2 + sum((7 + len(name) + pad))`.
- **AIFF sample rate corruption**: used `rateBits + rateFrac/2^64` (16398 for
  44100) → `ldexp(mantissa, exponent-16383-63)` produces correct 44100.
- **AIFF sample rate in XRNI reader**: same extended float bug
  (`reader_xrni.go`).
- **Simpler SlicingRegions**: `len(slices)` → `2` (start+end per part).
- **EL encoder**: silent `fmt.Fprintf` error discard → `errWriter` propagates.
- **D2PST extension mismatch**: writer was `.dt2pst` but runner/reader used `.d2pst`
  (typo). Fixed to `.dt2pst` everywhere.
- **OT encoder**: disable 16-bit optimization (`BitDepth=16` sets `BitDepth=24`
  internally) to match OT hardware expectations.

## [0.1.0] - 2026-06-22

### Added

- Initial chirashi release (rebranded from rexconverter)
- REX/RX2/RCY input via Reason REX SDK v1.9.2
- Output formats: WAV, AIFF, PTI (Polyend Tracker), OT (Octatrack), OP-1 AIFF,
  OP-XY preset, Elektron multi-sample, DT2 (Digitakt II), XRNI (Renoise),
  Simpler, ADV/ADG/ALS (Ableton)
- Input formats: REX/RX2/RCY (via SDK), XRNI, ADV, ADG, AIFF, PTI, Simpler, Drum Rack
- Mono downmix with mode selection (sum, left, right, difference, dual-detect)
- Resampling (any rate 11kHz – 1MHz)
- Slice limiting + balanced splits for multi-file output
- BPM override
- GPG-encrypted REX SDK distribution via GitHub Actions

### Changed

- Package renamed `rexengine` → `engine`
- Binary renamed `rexconverter` → `chirashi`
- Build pipeline uses `b.createModule()` with manual Zig bindings instead of
  `zig translate-c` (which hangs on REX.h in 0.16.0)
- Test step uses `CGO_ENABLED=0` to avoid linking CGo symbols in test mode

### Fixed

- `-o` flag now uses the output path as-is in single-file mode
- `-o` + `-l` produces chunked output with `_01`, `_02` suffixes
- PTI encoder handles empty CuePoints (`--no-slices`) without panic
- OT checksum range corrected to `0x10:0x33E` (excludes checksum slot)
