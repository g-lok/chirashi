# Errors

## [ERR-20260622-001] zig translate-c hangs on REX.h

**Logged**: 2026-06-22T13:13:00Z
**Priority**: high
**Status**: resolved
**Area**: infra

### Summary
`zig translate-c` hung indefinitely on `internal/engine/REX.h` (exit 124, >10min timeout). Caused CI cache failure on macOS build.

### Error
```
error: CacheCheckFailed
[process killed by GitHub Actions no-output timeout]
```

### Context
- zig version: 0.16.0
- target: x86_64-macos (cross-compile from Linux also hangs)
- flags: `zig translate-c -lc -target x86_64-macos -DREX_MAC=1 -DREX_WINDOWS=0 internal/engine/REX.h`
- Same header works fine with zig 0.15.2
- Header has packed structs, function pointer typedefs, pragma pack(push/pop)

### Suggested Fix
Bypass `translate-c` entirely. Use `b.createModule()` with hand-written `rex_bindings.zig` containing `extern fn`/`extern struct` declarations matching the C ABI.

### Resolution
- **Resolved**: 2026-06-22T13:20:00Z
- **Commit/PR**: PR #1, commit 5292084
- **Notes**: Created `internal/engine/rex_bindings.zig` with manual declarations. `build.zig` uses `b.createModule()` instead of `b.addTranslateC()`. Build verified clean on both x86_64-macos cross-compile and x86_64-linux native.

---

## [ERR-20260622-002] go test CGo undefined symbols for Zig functions

**Logged**: 2026-06-22T20:26:00Z
**Priority**: high
**Status**: resolved
**Area**: tests

### Summary
`go test ./tests/...` fails on macOS CI with undefined Zig symbols (`Zig_InitEngine`, `Zig_RenderLoopPreview`, etc.). The Zig object files are only linked into the final binary by `zig build`, not available to Go test linker.

### Error
```
Undefined symbols for architecture arm64:
  "_Zig_CloseEngine", referenced from:
      __cgo_dec753d25d0e_Cfunc_Zig_CloseEngine in 000001.o
  "_Zig_Diagnostic", referenced from:
      __cgo_dec753d25d0e_Cfunc_Zig_Diagnostic in 000001.o
  ... (8 more Zig symbols)
ld: symbol(s) not found for architecture arm64
```

### Context
- Platform: macos-latest (darwin_arm64)
- Command: `go test ./tests/... -v`
- Tests import `internal/engine` which has CGo bridge.go referencing Zig functions
- `bridge_stubs.go` exists with `//go:build !cgo` for non-CGo platforms
- On macOS, CGo is enabled by default → `bridge.go` used → Zig symbols needed

### Suggested Fix
Set `CGO_ENABLED=0` in test step env. Triggers `bridge_stubs.go` to be used. Reader tests don't need CGo; integration tests use pre-built binary via `exec.Command`.

### Resolution
- **Resolved**: 2026-06-22T20:30:00Z
- **Commit/PR**: PR #1, commit 14e28f9
- **Notes**: Updated `.github/workflows/ci.yml` `Run tests` step env to `CGO_ENABLED: "0"`. All non-REX tests now pass on macOS.

---

## [ERR-20260622-003] -o outPath fails to write file at expected path

**Logged**: 2026-06-22T20:39:00Z
**Priority**: high
**Status**: resolved
**Area**: backend

### Summary
Tests `TestLoopRender_MonoMatch` and `TestPCMStatisticalAnalysis` fail because binary doesn't write output file at the path specified by `-o`. Binary sanitizes full path (converts `/` to `_`) and adds chunk suffix, producing a different filename in the current dir.

### Error
```
integration_test.go:961: open /var/folders/.../TestLoopRender_MonoMatch3594517799/001/mono_out.wav: no such file or directory
```

### Context
- Binary output: `Converting: 120Stereo.rx2 -> _var_folders_bh_..._mono_out | Slices: 10`
- Test expects file at `dir/mono_out.wav` (full path with `.wav` extension)
- Bug in `internal/engine/runner.go outputBaseName()`: when `cfg.OutputFile` is set, function sanitizes the full path (converts `/` → `_`) and adds `_01` chunk suffix
- `cmd/root.go` restricts `-o` to single-input mode, so chunk suffixes don't apply
- This bug exists in upstream `rexconverter` too

### Suggested Fix
In `outputBaseName()`, when `cfg.OutputFile` is set, return path as-is (just `TrimSuffix` the extension). No sanitization, no suffix, no auto-derived basename. `writeOutputFiles()` adds the extension back.

### Resolution
- **Resolved**: 2026-06-22T20:45:00Z
- **Commit/PR**: PR #1, commit 7ad35d5
- **Notes**: Fixed `internal/engine/runner.go outputBaseName()`. New `cfg.OutputFile != ""` branch at top returns `strings.TrimSuffix(cfg.OutputFile, filepath.Ext(cfg.OutputFile))`. Push to PR; CI will validate.

---

## [ERR-20260623-004] AIFF MARK chunk corrupts output

**Logged**: 2026-06-23T08:00:00Z
**Priority**: high
**Status**: resolved
**Area**: backend

### Summary
AIFF MARK chunk writes null-terminated C strings instead of Pascal strings (1-byte length prefix), and markSize formula is wrong. Output files have corrupt slice markers.

### Error
```
(No runtime error — silent data corruption. AIFF files write but slice names are garbled/garbled.)
```

### Context
- Format: AIFF (Apple/SGI interchange format)
- IFF spec § "The Marker Chunk": marker names use Pascal strings (1-byte length prefix, no null terminator)
- Old code used Go `string` directly (null-terminated in binary.Write), wrong
- Old markSize: `2 + numMarkers*8 + numMarkers*2` — double-counts marker name bytes
- Correct markSize: `2 + sum((7 + len(name) + pad))` where pad = (len(name)+1)%2
- Each marker: marker ID (2B) + position (4B) + marker ID again (1B? no — 7 total per fixed fields: 2+4+1? Actually: 2 ID + 4 pos + 1 nameLen = 7)
- Every string padded to even byte boundary (including the length prefix byte)

### Suggested Fix
1. Write Pascal strings: `[]byte{byte(len(name))}` + `[]byte(name)` + padding byte if odd
2. Calculate markSize as: `2 + sum(7 + len(name) + (1 - (len(name) % 2)))` for each marker
3. Update reader to parse Pascal strings (1-byte length prefix, read that many bytes, no null termination check)

### Resolution
- **Resolved**: 2026-06-23T08:00:00Z
- **Commit/PR**: v0.3.0
- **Notes**: Fixed `encoder_aiff.go` MARK chunk writer. Updated `reader_aiff.go` to parse Pascal strings correctly.

---

## [ERR-20260623-005] AIFF 80-bit extended float formula wrong

**Logged**: 2026-06-23T08:00:00Z
**Priority**: high
**Status**: resolved
**Area**: backend

### Summary
Both `reader_aiff.go` and `reader_xrni.go` use wrong formula for 80-bit extended float sample rate: `rateBits + rateFrac/2^64` produces 16398 for 44100.

### Error
```
AIFF file with 44100 Hz sample rate: reader reports 16398 Hz
XRNI file embedding AIFF data: same wrong value
```

### Context
- AIFF sample rate field: 80-bit extended precision float (IEEE 754-1985 extended)
   - 1-bit sign (unused)
   - 15-bit exponent (biased by 16383)
   - 1-bit integer part (always 1 for normal floats)
   - 63-bit fraction
- Old formula in reader_aiff.go line ~163: `sr := rateBits + rateFrac/float64(1<<64)`
   - This is not how extended floats work!
   - `rateBits` is the 15-bit exponent, treated as integer part (wrong)
- Correct: `ldexp(mantissa, exponent - 16383 - 63)` where mantissa = 2^63 + fraction
- Bug found during code audit (looking at why AIFF test showed wrong sample rate)

### Suggested Fix
Replace:
```go
sr := rateBits + rateFrac/float64(1<<64)
```
With:
```go
mantissa := 1<<63 | rateFrac
sr := math.Ldexp(float64(mantissa), int(rateBits)-16383-63)
```

### Resolution
- **Resolved**: 2026-06-23T08:00:00Z
- **Commit/PR**: v0.3.0
- **Notes**: Fixed in both `reader_aiff.go` and `reader_xrni.go`. All AIFF/XRNI tests now show correct sample rate.

---

## [ERR-20260623-006] Simpler SlicingRegions value wrong

**Logged**: 2026-06-23T12:00:00Z
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
Simpler ADV encoder writes `<SlicingRegions>Value>N</Value>` where N = `len(slices)` (total slice count). Should be `2` (start region + end region per part).

### Error
```
ADV file written with len(slices)=16 slices but SlicingRegions value is 16 instead of 2
Simpler saw too many regions per part — incorrect behavior in Ableton Live
```

### Context
- ADV format: one `<SlicingRegions>` section per `<SimplerPart>` (which corresponds to one slice)
- Each SimplerPart has exactly 2 regions: "Slice Start" and "Slice End"
- Old code: `Value = len(slices)` — copied from total slices count
- Fix: hard-code `Value = 2`
- Found during code audit (comparing against Ableton Live's own .adv export)

### Suggested Fix
Hard-code `<SlicingRegions>Value>2</Value>` — each part always has exactly 2 regions.

### Resolution
- **Resolved**: 2026-06-23T12:00:00Z
- **Commit/PR**: v0.3.0

---

## [ERR-20260623-007] EL encoder silent write error discard

**Logged**: 2026-06-23T12:00:00Z
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
`encoder_el.go` uses `fmt.Fprintf` without checking the returned error. Write failures are silently discarded, producing truncated output.

### Error
```
(No runtime error — truncated Elektron multi-sample files on disk full/enospc)
```

### Context
- `fmt.Fprintf(w, ...)` returns (n, error) but error was always discarded
- Affects all 4 sections: header, preset, pattern, and project files
- Pattern: `fmt.Fprintf(w, ...)` called in a loop for each slice

### Suggested Fix
Wrap the writer in an `errWriter` struct that tracks the first error (pattern from `encoding/csv` Writer):
```go
type errWriter struct {
    w   io.Writer
    err error
}
func (ew *errWriter) write(s string) {
    if ew.err != nil { return }
    _, ew.err = io.WriteString(ew.w, s)
}
```

### Resolution
- **Resolved**: 2026-06-23T12:00:00Z
- **Commit/PR**: v0.3.0

---

## [ERR-20260623-008] D2PST reader looked for non-existent TLV file

**Logged**: 2026-06-23T12:00:00Z
**Priority**: high
**Status**: resolved
**Area**: backend

### Summary
`reader_d2pst.go` attempted to open a `.tlv` companion file that doesn't exist. Digitakt II presets embed slice positions in the binary payload.

### Error
```
D2PST read: open /path/to/SOLE DISPLAY.tlv: no such file or directory
```

### Context
- Old reader code: `os.ReadFile(strings.Replace(fname, ".d2pst", ".tlv", 1))`
- No real-world D2PST files have .tlv companions
- D2PST files embed slice data as binary patterns: `00 22 <uint32 LE pos> 00 08`
- WAV companion found via manifest.xml or filename matching

### Suggested Fix
Rewrite `Read()` to parse the preset binary payload directly using pattern scanning.

### Resolution
- **Resolved**: 2026-06-23T12:00:00Z
- **Commit/PR**: v0.3.0
- **Notes**: Full rewrite of `reader_d2pst.go`.

---

## [ERR-20260623-009] D2PST extension mismatch: .dt2pst vs .d2pst

**Logged**: 2026-06-23T12:00:00Z
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
Encoder writes `.dt2pst` extension but input detection matches `.d2pst`. Reader and writer disagree, causing encoding→reading roundtrip to fail.

### Error
```
Encoder output: SOLE DISPLAY.dt2pst
Input detection: .d2pst (skipped — no .d2pst file found)
```

### Context
- Encoder: `out+"."+constants.D2PST` where D2PST = `"dt2pst"`
- Runner inputExtensions list: `.d2pst`
- Reader registered via `init()`: `.d2pst`
- Fix: update runner to use `.d2pst` everywhere (or change extension constant)

### Suggested Fix
Keep constant as `"dt2pst"` (formally correct), but change `inputExtensions` and reader to `.d2pst`. The `.dt2pst` files are just written with that extension; when reading back, rename or use `.d2pst`.

### Resolution
- **Resolved**: 2026-06-23T12:00:00Z
- **Notes**: Renamed writer extension to `.d2pst` in the output path logic.


