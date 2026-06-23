# AGENTS.md — chirashi agent instructions

## Project overview

chirashi is a Go + Zig CLI for converting between sliced instrument formats. The Go side handles CLI, file format parsing, encoders, and orchestration. The Zig side wraps the Reason Studios REX SDK (proprietary) for REX/RX2/RCY input.

- **Go**: cmd/, internal/engine/ (most files), tests/
- **Zig**: internal/engine/extractor.zig, extractor_stub.zig, rex_bindings.zig
- **C**: internal/engine/rex/REX.c (Windows DLL loader only)
- **REX SDK**: macOS framework at internal/engine/libs/macos/, Windows DLL

## Key constraints

### REX SDK is proprietary
- Never commit `internal/engine/libs/`
- Never commit `internal/engine/rex/` SDK headers
- Never commit `internal/engine/Frameworks/`
- All binary SDK files are decrypted at CI build time from GPG-encrypted tarballs
- See `~/.config/opencode/AGENTS.md` "Proprietary CI assets" for the GPG keypair pattern

### Linux has no REX SDK
- `build.zig` uses `extractor_stub.zig` for Linux targets
- `bridge_stubs.go` (`//go:build !cgo`) provides no-op stubs when CGo is disabled
- CI test step on macOS sets `CGO_ENABLED=0` to use stubs (so `go test` doesn't need Zig object files)

### Zig translate-c is broken in 0.16.0
- `zig translate-c` hangs on REX.h (packed structs, function pointer typedefs)
- `build.zig` uses `b.createModule()` with hand-written `rex_bindings.zig` (extern declarations)
- Do not revert to `b.addTranslateC()`

### CLI -o semantics
- When `-o` is set WITHOUT `-l` (slice limit): use the path as-is, no sanitization, no suffix
- When `-o` is set WITH `-l`: treat the path as a base name pattern, append chunk suffixes (_01, _02, ...)
- `cmd/root.go` restricts `-o` to single-input mode

### Format-specific constraints
- **AIFF MARK**: Pascal strings (1-byte length prefix), padded to even boundary per IFF spec
- **AIFF sample rate**: 80-bit extended float with `ldexp(mantissa, exponent-16383-63)` formula
- **CAF**: Linux uses `lpcm` PCM (no afconvert), metadata via standard UUID chunks
- **D2PST**: slice positions embedded in binary payload as `00 22 <uint32 LE pos> 00 08` patterns (no TLV file)
- **PTI readers**: dual-format dispatch — `TI\x01` (encoder, 280–375 ratio table) and `PTI\x00` (legacy)
- **OT readers**: dual-format dispatch — `FORM...DPS1` (encoder, IFF structure, checksum at 0x33E) and `OT\x00\x00` (legacy)
- **PTI limit**: hardware max 48 slices, runner auto-splits via groupSlices
- **OT bit depth**: encoder forces 24-bit (OT hardware expects 24-bit internally)
- **Scale factor**: WAV/OP-1/PTI use 32767.0; AIFF/CAF use 32768.0 (both correct per spec)
- **Simpler ADV**: SlicingRegions value is always 2 (start+end per part)

## Build system

```bash
mise run build         # macOS binary via zig build
mise run build-linux   # Linux binaries (amd64, arm64, armv7)
```

`build.zig` is the coordinator:
1. Runs `go build -buildmode=c-archive` to produce go_engine.a
2. Compiles extractor.zig (with rex_bindings.zig for C bindings)
3. Links them together
4. Links platform-specific SDK (REX framework on macOS, REX.c dynamic loader on Windows, nothing on Linux)

The go_engine.a archive contains Go code only. The Zig code is linked separately.

## Testing

- `tests/integration_test.go` — uses pre-built binary via exec.Command
- `tests/encoder_format_test.go` — tests encoders with the binary
- `tests/reader_test.go` — tests readers in-process (no binary needed)
- `tests/processor_test.go` — slice processing unit tests
- `samples/` — real-world test files fallback when `testdata/` missing
- Test helpers (`findTestXRNI`, `findTestSimpler`, `findTestDrumRack`) search `testdata/` first, then `samples/`

Run with `CGO_ENABLED=0` to avoid Zig linker issues. Integration tests skip if the binary isn't found.

## Workflow

- User uses `jj` (Jujutsu), not raw git
- `jj describe -m "msg"` updates the working copy commit
- `jj git push --bookmark <name>` pushes to remote
- `jj bookmark set <name> -r @` to move a bookmark
- `jj abandon <rev>` to discard a rev
- `jj rebase -d <dest>` to rebase working copy

## Local tooling

- `mise` for go/zig versioning (`mise.toml`)
- `poetry` for the graphify project (`pyproject.toml`, `.venv/`)
- `graphify` for codebase knowledge graph
- `gh` for GitHub operations (PRs, CI runs)

## Don't

- Don't run `apt install`, `brew install`, or system-wide `pip install` — use local workspaces
- Don't commit `a.out`, `build/`, root `rexconverter`, `internal/engine/libs/`, `internal/engine/Frameworks/` — all in `.gitignore`
- Don't use `b.addTranslateC()` in build.zig — it hangs
- Don't add `bin/` to git — gitignore blocks it
- Don't modify the squashed PR #1 commit (`45c4f3b`) — it's immutable
- Don't use `enum(T)` extern in zig 0.16.0 — just `enum(T)` works (extern is rejected)
