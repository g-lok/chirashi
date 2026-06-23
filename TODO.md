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

## Tooling

- [ ] `mise run graphify` task failing (needs Gemini API key for semantic extraction)
- [ ] Add a `lint` task (golangci-lint, zig fmt check)
- [ ] Add a `release` task for cutting tags

## Documentation

- [ ] Add USAGE.md with worked examples (separate from README)
- [ ] Add ARCHITECTURE.md explaining the Go↔Zig↔REX SDK layering
- [ ] Add a TROUBLESHOOTING.md for common issues

## Testing

- [ ] CI cache strategy for test data files
- [ ] Add fuzz tests for format parsers
- [ ] Performance benchmarks (vs. original rexconverter)
