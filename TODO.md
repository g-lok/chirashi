# TODO

## Build / CI

- [ ] Investigate `240FiveHundredSlices.rx2` test failures — REX SDK reports 1 slice
      for a 500-slice file. May be SDK limitation or test data issue.
- [ ] Add Linux REX support via Wine/REX.c (currently stub-only on Linux)
- [ ] Force-push history rewrite to remove build artifacts from commit `45c4f3b`
      (immutable, requires destructive force-push — coordinate if needed)

## Features

- [ ] Add WAV reading for batch input (currently WAV is output-only)
- [ ] Add SFZ output format
- [ ] Add EXS24 output format
- [ ] Sample rate auto-detection from input file
- [ ] Multi-channel output (>2 channels)
- [ ] D2PST companion WAV lookup via manifest.xml (currently requires matching
      filename without extension — manifest.xml may specify a different WAV path)

## Tooling

- [ ] `mise run graphify` task failing (needs Gemini API key for semantic extraction)
- [ ] Add a `lint` task (golangci-lint, zig fmt check)
- [ ] Add a `release` task for cutting tags (for v0.3.0 and beyond)

## Documentation

- [ ] Add USAGE.md with worked examples (separate from README)
- [ ] Add ARCHITECTURE.md explaining the Go↔Zig↔REX SDK layering
- [ ] Add a TROUBLESHOOTING.md for common issues

## Testing

- [x] samples/ directory with real-world test files (XRNI, D2PST, ADV, ADG, OT)
- [x] Test helpers fallback to samples/ when testdata/ missing
- [ ] CI cache strategy for test data files
- [ ] Add fuzz tests for format parsers
- [ ] Performance benchmarks (vs. original rexconverter)
- [ ] Add more real-world sample files as they become available

## v0.3.0 Completed ✓

Out of scope for this release but captured for future:

- [ ] AIFF 80-bit float write support (for non-44100 sample rates, encoder writes
      44100 directly; correct extended float encoding would add complexity)
- [ ] CAF ALAC encoding (requires macOS afconvert or external encoder)
- [ ] OT companion WAV generation in reader (currently returns error requiring
      manual WAV extraction)
