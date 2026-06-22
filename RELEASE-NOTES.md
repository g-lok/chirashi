# Release Notes

## v0.3.0-beta (2026-05-28)

### Highlights

- **Digitakt II preset format (`--format d2pst`) now working**. Produces valid `.dt2pst` ZIP archives with manifest.json + 48kHz WAV + TLV tag-encoded preset binary. Tested against Elektron Transfer — imports successfully.

### New Features

- DT2 preset encoder (`EncodeDT2Preset`): generates ZIP with 3 Deflate entries — `manifest.json`, `Samples/transfers-YYMMDD/<name>.wav`, and the preset binary (named after the payload)
- Binary preset format: 206-byte template header (reverse-engineered from "SOLE DISPLAY" reference) + per-slice TLV records (`00 22 <pos:4> 00 08`) + 87-byte footer
- Hash/checksum embedded at offset 0xBB (big-endian uint32) and mirrored in manifest `Samples[0].Hash`
- Payload name and WAV filename derived from output basename (not hardcoded) — avoids collisions on Transfer import
- Name sanitized to 12 alphanumeric/spaces max (DT2 firmware constraint)
- WAV rendered at 48kHz/16-bit as DT2 expects on import

### Changes

- `EncodeDT2Preset` signature changed from `(w io.Writer, extraction *SliceExtraction)` to `(w io.Writer, extraction *SliceExtraction, name string)` — caller passes output basename for packaging
- Updated `.gitignore` to cover `rexconverter.a` / `rexconverter.h` from direct `go build -buildmode=c-archive .`

### Bug Fixes

- ZIP writer now produces valid ZIP archives (all CRCs match, entries list correctly in Python `zipfile` module)
- Template truncation in `buildDT2PresetBinary` fixed — header was 207 bytes (should be 206), causing 1-byte offset in TLV records

### Build

- No new dependencies. Same Go 1.26+ / Zig 0.16.0+ / REX SDK v1.9.2 requirements.

## v0.1.1 (2026-05-26)

### Changes

- Migrated from deprecated `@cImport` to `b.addTranslateC()` for REX SDK C header translation (Zig 0.16.0 compliance)
- Replaced `@memcpy` with `std.mem.copyForwards` (removed in Zig 0.16.0)
- Added CI/CD GitHub Actions workflows for automated testing and release building
- Updated documentation to reflect Zig 0.16.0 patterns
- Staged the macOS REX SDK framework automatically in the local compile-time build pipeline (resolves local and CI execution errors)
- Implemented secure AES-256 GPG encryption for public repository storage of the proprietary REX SDK binaries
- Corrected macOS codesigning paths in CI to target the internal Mach-O executable directly, avoiding ambiguous bundle format errors

### Upgrade Notes

- Requires Zig 0.16.0+ (was already the requirement)
- No behavioral changes — all 37 tests pass identically on both local and CI systems
- Encrypted SDK files are committed under `.github/workflows/secrets/` and require the GPG passphrase `REX_SDK_PASSWORD` secret to build in CI/CD
