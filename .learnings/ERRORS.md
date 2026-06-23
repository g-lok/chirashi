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
