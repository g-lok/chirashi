# Changelog

All notable changes to chirashi will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
