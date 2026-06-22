# Changelog

## [Unreleased]

### Added
- **Phase 2 cross-format input readers** — 10 pure Go readers for 14 extensions:
  WAV (cue chunks), AIFF/AIFC (MARK chunks), XRNI (ZIP+XML+FLAC),
  ADV/ALS (Ableton Simpler GZip XML), ADG (Ableton Drum Rack GZip XML),
  PTI (Polyend Tracker), OT (Octatrack sidecar), XY (OP-XY ZIP),
  D2PST (Digitakt II ZIP+TLV)
- **Phase 2 output encoders** — 5 new formats for full round-trip:
  XRNI (ZIP: Instrument.xml + WAV), ADV/ALS (GZip Simpler XML + companion WAV),
  ADG (GZip Drum Rack XML + per-pad WAVs), AIFF (standard COMM/SSND/MARK)
- **`--library-path` flag** — Ableton User Library path for sample resolution
  on ADV/ADG input (attempts 5 fallback strategies)
- **`--sample-path-mode` flag** — control sample path style in XML output
  (relative/absolute/library)
- **`--input-format` flag** — force input format override

### Changed
- **CLI scope** — `rexconverter` now accepts any of 14 input extensions,
  not just REX. Detects by extension or explicit `--input-format`
- **`scanDirectory()`** — scans for all supported input extensions
  (`.rex`, `.rx2`, `.rcy`, `.xrni`, `.als`, `.adv`, `.adg`, `.wav`, `.aif`,
  `.aiff`, `.pti`, `.ot`, `.xy`, `.d2pst`)
- **Concurrency** — REX SDK path remains mutex-guarded (`rexMutex` in
  `bridge.go`); all other readers are lock-free and goroutine-safe.
  Removed global per-file mutex from `runPipeline()` — Go operations
  (downmix, group, encode) now run fully concurrent across files.
- **`PipelineConfig`** — moved from `bridge.go` (CGo) to `types.go` (pure Go)
  for CGo-free test builds

### Fixed
- **Ableton XML attribute parsing** — ADV reader correctly handles
  `<Path Value="..."/>` (attribute-style) via `attrString` helper type
- **ADG drum rack structure** — reads `BranchPresets`/`DrumBranchPreset`
  with `DrumCell` + `ZoneSettings`/`ReceivingNote` (not legacy
  `DrumPadsListWrapper`/`Pad` path)
- **FLAC decode API** — updated for `mewkiz/flac` v1.0.13+ API
  (`ParseNext()` → `*frame.Frame` with `Subframes` slice, `BlockSize`)

## [0.3.0] — REX → multi-format (current)

### Added
- Initial release: REX/RX2/RCY → WAV, PTI, OT, OP-1 AIFF, XY, EL, DT2
- REX SDK integration via Zig bridge
- Tempo-based loop rendering, cue marker calculation
- Batch directory scanning, slice splitting, mono downmix
- macOS native + Windows cross-compile support
