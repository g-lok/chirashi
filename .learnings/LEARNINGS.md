# Learnings

## [LRN-20260622-001] knowledge_gap

**Logged**: 2026-06-22T13:00:00Z
**Priority**: high
**Status**: promoted
**Area**: infra

### Summary
zig 0.16.0 `translate-c` hangs on complex C headers (REX.h with packed structs/macros) — regression from 0.15.2.

### Details
- `zig translate-c` on `internal/engine/REX.h` exits 124 (timeout) in 0.16.0
- Same header works fine with zig 0.15.2
- REX.h has packed structs, function pointer typedefs, pragma pack(push/pop)
- Likely regression in 0.16.0 translate-c implementation
- Workaround: hand-write extern declarations matching the C ABI

### Suggested Action
- For C headers that hang in translate-c, use `b.createModule()` in build.zig with manually written bindings
- Bindings should be minimal: only types/functions actually used
- Use `enum(T)` for C-compatible enums (not `extern enum(T)` in 0.16.0)
- Use `callconv(.c)` (lowercase) for C function pointers
- Mark nullable C pointers as `?*anyopaque`

### Metadata
- Source: error
- Related Files: build.zig, internal/engine/rex_bindings.zig
- Tags: zig, translate-c, cgo, ci
- See Also: ERR-20260622-001
- Pattern-Key: zig.translatec.hang
- Recurrence-Count: 1
- First-Seen: 2026-06-22
- Last-Seen: 2026-06-22

### Resolution
- **Resolved**: 2026-06-22T13:20:00Z
- **Promoted**: ~/.config/opencode/AGENTS.md (Zig translate-c section)
- **Notes**: Created `rex_bindings.zig` with manual extern declarations. `build.zig` uses `b.createModule()`. Build verified on both targets.

---

## [LRN-20260622-002] knowledge_gap

**Logged**: 2026-06-22T20:26:00Z
**Priority**: high
**Status**: promoted
**Area**: tests

### Summary
`go test` with CGo requires all referenced symbols to be linkable. Zig object files compiled by `zig build` are not available to Go test linker.

### Details
- Tests in `tests/` import `internal/engine` which has CGo bridge.go
- bridge.go references Zig functions (Zig_InitEngine, etc.)
- Zig code is compiled by `zig build` and linked into final binary only
- `go test` runs independently and tries to link CGo symbols
- Result: undefined symbol errors on macOS
- Linux already worked because CI sets `CGO_ENABLED=0` → bridge_stubs.go used

### Suggested Action
For Go projects that use CGo to call Zig (or other compiled languages), use the pattern:
1. Provide a `bridge.go` (CGo, calls Zig functions)
2. Provide a `bridge_stubs.go` with `//go:build !cgo` (no-op stubs)
3. Set `CGO_ENABLED=0` in CI test step
4. Reader tests use pure Go APIs (don't need CGo)
5. Integration tests use the pre-built binary via `exec.Command`

### Metadata
- Source: error
- Related Files: .github/workflows/ci.yml, internal/engine/bridge.go, internal/engine/bridge_stubs.go
- Tags: go, cgo, zig, testing, ci
- See Also: ERR-20260622-002

### Resolution
- **Resolved**: 2026-06-22T20:30:00Z
- **Notes**: Updated CI workflow. macOS tests now use CGO_ENABLED=0, all non-REX tests pass.

---

## [LRN-20260622-003] best_practice

**Logged**: 2026-06-22T20:39:00Z
**Priority**: high
**Status**: resolved
**Area**: backend

### Summary
When CLI flag specifies a full output path (`-o /path/to/output.ext`), use it as-is — don't sanitize, don't add suffixes, don't auto-derive basename.

### Details
- Bug in `outputBaseName()`: sanitized full path (replaced `/` with `_`) and added `_01` chunk suffix
- `cmd/root.go` already restricts `-o` to single-input mode, so chunk suffixes don't apply
- User intent: "write exactly to this file"
- Test failures: binary output `_var_..._mono_out.wav` (sanitized) but test expects `/var/.../mono_out.wav`

### Suggested Action
For CLI tools with `-o` output flag:
1. If user provides a full path, use it as-is
2. Only sanitize/derive basename when no explicit path is given
3. Validate at cmd layer that single-input mode is required for `-o`
4. Strip extension in path helper, let writer add it back

### Metadata
- Source: error
- Related Files: internal/engine/runner.go, cmd/root.go
- Tags: go, cli, ux, file-paths
- See Also: ERR-20260622-003

### Resolution
- **Resolved**: 2026-06-22T20:45:00Z
- **Commit/PR**: PR #1, commit 7ad35d5
- **Notes**: Refactored `outputBaseName()`. New `cfg.OutputFile != ""` branch at top returns path with just `TrimSuffix` on extension. `writeOutputFiles()` adds extension back. CI will validate.

---

## [LRN-20260622-004] best_practice

**Logged**: 2026-06-22T13:00:00Z
**Priority**: medium
**Status**: promoted
**Area**: infra

### Summary
No global package installs. All project dependencies managed through local workspace tooling (mise + Poetry + .venv).

### Details
- mise: tool versioning (go, zig, python) via `mise.toml`
- Poetry: Python deps via `pyproject.toml` with local `.venv`
- `package-mode = false` in `[tool.poetry]` for non-package projects
- Scaffolding script: `~/bin/newpproj.sh <name>` creates full Poetry + mise setup
- The "trick" for mise to take over venv management: scaffold → inject mise.toml → install deps → nuke .venv → clear cache → re-enter dir
- `poetry add <pkg>` automatically installs + updates pyproject.toml + updates poetry.lock
- This avoids CI hangs from global installs (poetry@latest HTTP 403, python 3.14.3 unavailable as prebuilt binary)

### Suggested Action
- Always create new projects with `~/bin/newpproj.sh <name>`
- Use `poetry add` not `pip install`
- For ad-hoc scripts: create project dir with `pyproject.toml`, `mise.toml`, `.venv`
- Never use system `apt install`, `brew install`, or `pip install --user`
- For Node: use `npm --prefix` or `pnpm workspaces`, never `npm i -g`

### Metadata
- Source: user_feedback
- Tags: mise, poetry, python, venv, workspace, ad-hoc-scripts
- Pattern-Key: infra.workspace.isolation
- Recurrence-Count: 1

### Resolution
- **Promoted**: ~/.config/opencode/AGENTS.md (Workspace Tooling section)
- **Notes**: Added detailed section to global AGENTS.md covering newpproj.sh, poetry add, package-mode=false, and the venv nuke trick.

---

## [LRN-20260622-005] best_practice

**Logged**: 2026-06-22T11:00:00Z
**Priority**: medium
**Status**: resolved
**Area**: infra

### Summary
For proprietary/proprietary-licensed dependencies (like REX SDK), use asymmetric GPG keypair for CI decryption, not symmetric passphrase.

### Details
- Symmetric passphrase stored as GitHub secret: bad practice (anyone with the secret can decrypt and re-encrypt)
- Asymmetric keypair: public key committed to repo, private key stored as GitHub secret
- Generate with `gpg --quick-generate-key` with `%no-protection` (no passphrase) for batch use
- Public key path: `.github/workflows/secrets/<name>-public.key` (committed)
- Private key as `GPG_SIGNING_KEY` secret: `gh secret set GPG_SIGNING_KEY < private.key`
- CI step: `echo "${{ secrets.GPG_SIGNING_KEY }}" | gpg --import --batch`
- Decrypt: `gpg --decrypt --batch --output file.tar.gz file.tar.gz.gpg` (no --passphrase flag)

### Suggested Action
- For any CI that decrypts proprietary assets, use asymmetric GPG keypair
- Commit public key to repo, keep private key as GitHub secret only
- Use `%no-protection` flag for batch CI decryption
- Pin GPG key with specific user identity + email for traceability

### Metadata
- Source: user_feedback
- Tags: gpg, ci, secrets, encryption, proprietary
- Related Files: .github/workflows/ci.yml, .github/workflows/release.yml, .github/workflows/secrets/

### Resolution
- **Resolved**: 2026-06-22
- **Notes**: Implemented for chirashi. REX SDK tarballs encrypted with public key, private key in GitHub secrets. CI imports key before decrypt step.

---

## [LRN-20260622-006] best_practice

**Logged**: 2026-06-22T13:00:00Z
**Priority**: low
**Status**: resolved
**Area**: infra

### Summary
Use `jj` (Jujutsu) for version control. Pairs naturally with `mise` and keeps working copy as a real commit.

### Details
- `jj` uses a different model than git: working copy is a real commit
- No staging area, no detached HEAD confusion
- `jj describe -m "msg"` updates working copy commit message
- `jj git push --branch <name>` pushes to remote
- `jj status` shows working copy changes (like git status but with full content)
- Works with existing git remotes transparently
- The `init` commit in jj is the result of squashing rebranding history

### Suggested Action
- Use `jj` for project version control instead of raw git
- `jj describe -m` for commit messages
- `jj git push --branch <branch-name>` for pushing
- `jj status` to see working copy state

### Metadata
- Source: user_feedback
- Tags: jj, jujutsu, vcs, workflow
- Related Files: project root (chirashi uses jj)

### Resolution
- **Notes**: User prefers jj for chirashi. Global commits done via `jj describe` + `jj git push`.

---

## [LRN-20260622-007] bug

**Logged**: 2026-06-22T23:30:00Z
**Priority**: high
**Status**: resolved
**Area**: tests

### Summary
`-n` (NoSlices) flag in test commands caused normalize-split tests to fail — collapsed 500 slices to 1, preventing multi-file output.

### Details
- `TestIntegration_NormalizeSplits` and `TestLoopRenderMatch_Stereo_Normalize` used `-l 64 -n` flags
- With `-n`, `RenderLoopPreview` returns 1 `SliceExtraction` (full loop), not 500 individual slices
- `groupSlices` sees `total=1` and `maxSlices=64`, enters else branch, produces 1 output file
- Test glob `norm_*.wav` expects multiple `_NN` suffixed files — gets 0 matches

### Suggested Action
- Remove `-n` flag from normalize-split tests; the `-n` was a copy-paste error from other tests
- CI build-and-test passes with this fix (verified on PR #1)

### Metadata
- Source: error
- Related Files: tests/integration_test.go
- Tags: go, tests, normalize-splits, flags
- See Also: LRN-20260622-003 (outputBaseName fix)

### Resolution
- **Resolved**: 2026-06-22T23:25:00Z
- **Commit**: PR #1, commit df7b3ec8

---

## [LRN-20260622-008] best_practice

**Logged**: 2026-06-22T23:35:00Z
**Priority**: medium
**Status**: resolved
**Area**: infra

### Summary
`.DS_Store` files from macOS Finder pollute jj/git tracking when using `jj file untrack` with glob patterns.

### Details
- macOS Finder creates `.DS_Store` in every directory
- These were in the working copy despite being untracked
- Fix: add `.DS_Store` to `.gitignore` and use `jj file untrack '**/.DS_Store'`

### Suggested Action
- Always add `.DS_Store` to `.gitignore` for macOS projects
- Use `jj file untrack '**/.DS_Store'` to remove from tracking

### Metadata
- Source: user_feedback
- Tags: jj, macos, gitignore, ds_store
- Related Files: .gitignore

### Resolution
- **Resolved**: 2026-06-22T23:32:00Z
- **Notes**: Added to .gitignore, untracked via jj file untrack

## [LRN-20260622-009] best_practice

**Logged**: 2026-06-23T00:00:00Z
**Priority**: medium
**Status**: resolved
**Area**: packaging

### Summary
Linux chirashi binary is purely Go (`CGO_ENABLED=0`), no Zig or REX SDK required at build time.

### Details
- Linux build entry point: `main_linux.go` with `//go:build linux && !cgo` tag
- Imports `cmd.Execute()` only — no CGo calls into the Zig extractor
- REX SDK is stubbed on Linux (`internal/engine/extractor_stub.zig` linked in `build.zig`)
- Static binary, no runtime deps — verified with `file` + `ldd`
- Perfect for binary-only AUR/homebrew/scoop packages (no toolchain on user side)

### Suggested Action
- For Linux distribution packages, ship the prebuilt `chirashi-v*-linux-amd64` from the release page
- AUR `makedepends` can stay empty for binary packages
- No need to package Go or Zig as build deps for Linux chirashi
- The homebrew formula's Linux branches can use the raw amd64/arm64 binary directly (no `.tar.gz` needed)

### Metadata
- Source: code_review
- Tags: linux, packaging, homebrew, scoop, aur, cgo, static-binary
- Related Files: main_linux.go, build.zig, mise.toml

### Resolution
- **Resolved**: 2026-06-23T00:00:00Z
- **Notes**: Confirmed by inspecting main_linux.go build tags + `file` output on downloaded release asset

## [LRN-20260622-010] knowledge_gap

**Logged**: 2026-06-23T00:05:00Z
**Priority**: high
**Status**: open
**Area**: packaging

### Summary
AUR (Arch User Repository) account registrations are down — cannot publish new packages.

### Details
- AUR registrations blocked as of 2026-06-23
- chirashi-bin AUR package staged locally at `~/Projects/aur/`
- Cannot add remote, cannot push until registrations resume
- AUR uses SSH at `ssh://aur@aur.archlinux.org/<pkg>.git` per-package
- Each new package needs web registration via https://aur.archlinux.org/submit/ first

### Suggested Action
- Check https://aur.archlinux.org/ status before attempting to publish
- When back up: register via web form, then add remote + push
- Keep local PKGBUILD + .SRCINFO staged in `~/Projects/aur/` so publication is a one-step push
- See `/home/g/Documents/chirashi-aur-instructions.md` for full publish procedure

### Metadata
- Source: user_feedback
- Tags: aur, arch, packaging, blocked, external
- Related Files: ~/Projects/aur/chirashi-bin/PKGBUILD, ~/Projects/aur/chirashi-bin/.SRCINFO, /home/g/Documents/chirashi-aur-instructions.md

---

## [LRN-20260623-011] bug

**Logged**: 2026-06-23T08:00:00Z
**Priority**: high
**Status**: resolved
**Area**: backend

### Summary
AIFF MARK chunk was corrupting output files: null-terminated C strings written instead of Pascal-style (1-byte length prefix), and `markSize` formula was wrong.

### Details
- IFF spec requires Pascal strings: 1-byte length prefix (not null terminator)
- Old formula: `2 + numMarkers*8 + numMarkers*2` (8 bytes per marker + 2 bytes per string)
- Correct formula: `2 + sum((7 + len(name) + pad))` (2 for numMarkers, 7 per fixed marker fields, len(name) for string, padding to even)
- Each marker name padded to even length per IFF spec

### Metadata
- Source: error
- Related Files: internal/engine/encoder_aiff.go, internal/engine/reader_aiff.go
- Tags: aiff, iff, mark, audio
- Pattern-Key: aiff.mark.pascal

### Resolution
- **Resolved**: 2026-06-23T08:00:00Z
- **Notes**: Encoder now writes Pascal strings. Reader updated to parse Pascal strings correctly.

---

## [LRN-20260623-012] bug

**Logged**: 2026-06-23T08:00:00Z
**Priority**: high
**Status**: resolved
**Area**: backend

### Summary
AIFF 80-bit extended float formula was wrong everywhere: used `rateBits + rateFrac/2^64` which produced 16398 for 44100 instead of 44100.

### Details
- AIFF sample rate uses 80-bit extended precision float (IEEE 754 extended)
- Old formula: `rateBits + rateFrac / 2^64` — completely wrong
- Correct formula: `ldexp(mantissa, exponent - 16383 - 63)` where mantissa = 2^63 + fraction
- Bug was in both `reader_aiff.go` and `reader_xrni.go`

### Metadata
- Source: error
- Related Files: internal/engine/reader_aiff.go, internal/engine/reader_xrni.go
- Tags: aiff, extended-float, sample-rate
- Pattern-Key: aiff.extended.float.formula

### Resolution
- **Resolved**: 2026-06-23T08:00:00Z

---

## [LRN-20260623-013] best_practice

**Logged**: 2026-06-23T09:00:00Z
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
PTI TI-format reader must parse 392-byte header with ratio table at offset 280–375 as uint16 LE. Each value / 65535 gives normalized frame position.

### Details
- Community format verified against jaap3/pti-file-format + jaap3/pti-tools
- Magic: `TI\x01`
- 392-byte header (not the old 4-byte magic assumption)
- Ratio table at bytes 280–375 (96 bytes = 48 × uint16 LE)
- Embeds PCM audio after header
- Legacy PTI\x00 format is a separate code path

### Metadata
- Tags: pti, polyend-tracker, format
- Related Files: internal/engine/reader_pti.go

### Resolution
- **Resolved**: 2026-06-23T09:00:00Z

---

## [LRN-20260623-014] best_practice

**Logged**: 2026-06-23T09:00:00Z
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
OT DPS1 format is an IFF-style structure: `FORM...DPS1` with 64×12-byte slice slots at offset 58 containing frame offsets, checksum at 0x33E.

### Details
- Community format verified against ot-tools-io, icaroferre/ot_utils, digichain
- `FORM` (4 bytes) + 4-byte size + `DPS1` (4 bytes) = IFF container
- Slice slots: 64 slots × 12 bytes each = 768 bytes starting at offset 58
  - Each slot: 4-byte start frame (BigEndian), 4-byte end frame (BigEndian), 4-byte unknown (0xFFFFFFFF)
- Checksum at 0x33E: uint16 BigEndian, sum of bytes 0x10 → 0x33E (exclusive)
- Num slices at offset 0x33A: uint32 BigEndian
- Sample offset at offset 0x2E (48): uint16 BigEndian sector offset
- Legacy OT\x00\x00 format uses byte offsets instead of frame offsets

### Metadata
- Tags: ot, octatrack, format
- Related Files: internal/engine/reader_ot.go, internal/engine/encoder_ot.go

### Resolution
- **Resolved**: 2026-06-23T09:00:00Z

---

## [LRN-20260623-015] best_practice

**Logged**: 2026-06-23T10:00:00Z
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
D2PST (Digitakt II preset) has no separate TLV file. Slice positions are embedded in the preset binary payload as `0x00 0x22 <uint32 LE pos> 0x00 0x08` byte patterns.

### Details
- The old reader expected a .tlv companion file that doesn't exist
- Real D2PST files embed slice data in the binary payload
- Pattern: `00 22 xx xx xx xx 00 08` where `xx xx xx xx` is uint32 LE byte offset
- WAV companion file referenced via `$name.manifest.xml` (or `$name.wav`)
- Chunk-level parsing not needed — pattern scan across the payload is sufficient

### Metadata
- Tags: d2pst, digitakt, format
- Related Files: internal/engine/reader_d2pst.go, internal/engine/encoder_d2pst.go

### Resolution
- **Resolved**: 2026-06-23T10:00:00Z

---

## [LRN-20260623-016] best_practice

**Logged**: 2026-06-23T10:00:00Z
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
CAF on Linux must use `lpcm` PCM format since `afconvert` (Apple's ALAC encoder) is unavailable. Apple Loop metadata is stored via standard UUID chunks.

### Details
- CAF encoder uses `lpcm` (linear PCM), format/format-flags from CAF spec
- Apple Loop metadata: 4 UUID chunks
  - `AURL` beat markers: slice positions + beat number
  - Loop info: beat count, time sig numerator/denominator, render fps
  - Transient info: transient positions per slice
  - Gap info: gap/fill positions
- Detected via `init()` registration as `.caf` extension
- 8-byte packet table required for variable-rate formats but empty for lpcm

### Metadata
- Tags: caf, apple, format
- Related Files: internal/engine/encoder_caf.go, internal/engine/reader_caf.go

### Resolution
- **Resolved**: 2026-06-23T10:00:00Z

---

## [LRN-20260623-017] best_practice

**Logged**: 2026-06-23T11:00:00Z
**Priority**: low
**Status**: resolved
**Area**: backend

### Summary
Scale factor asymmetry between PCM encoders: WAV/OP-1/PTI use `32767.0` for float→int16 conversion, AIFF/CAF use `32768.0`. Both are correct within their respective range limits.

### Details
- WAV: 32767.0 matches the standard max positive value (no clipping at +1.0)
- AIFF: 32768.0 matches the AIFF specification which uses full [-32768, 32767] range
- Both produce correct output for their respective formats
- Not a bug — intentional behavior matching each format's spec

### Metadata
- Tags: pcm, scale-factor, audio
- Related Files: internal/engine/encoder_wav.go, internal/engine/encoder_aiff.go, internal/engine/encoder_caf.go, internal/engine/encoder_pti.go

---

## [LRN-20260623-018] best_practice

**Logged**: 2026-06-23T12:00:00Z
**Priority**: low
**Status**: resolved
**Area**: backend

### Summary
`samples/` directory at project root should be used for real-world test files with test helpers falling back when `testdata/` files are absent.

### Details
- Test helpers now search `testdata/` first, then `../samples/` (relative to `tests/`)
- Covers: XRNI, ADV, ADG, D2PST, OT
- `findTestOT()` and `findTestD2PST()` look only in `samples/` (no testdata copies)
- New tests added for real files: `TestD2PSTReader_RealFile`, `TestOTReader_RealSampleFile`

### Metadata
- Tags: testing, samples
- Related Files: tests/reader_test.go

### Resolution
- **Resolved**: 2026-06-23T12:00:00Z

---

## [LRN-20260623-019] best_practice

**Logged**: 2026-06-23T12:00:00Z
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
When readers handle both encoder and legacy formats, dispatch on magic bytes at `Read()` time (not `Probe()`). Both formats must pass `Probe()` with acceptable sample rate/channels metadata.

### Details
- PTI: `TI\x01` (encoder) vs `PTI\x00` (legacy)
  - Encoder format verified by jaap3/pti-file-format + pti-tools
  - Legacy format has zero community references but existing files may exist
- OT: `FORM...DPS1` (encoder) vs `OT\x00\x00` (legacy)
  - Encoder format verified by ot-tools-io, ot_utils, digichain
  - Legacy format has zero community references
- Both readers return full descriptive errors on invalid content within each path
- Bounds checking: slice slot count, checksum verification for DPS1, ratio table bounds for TI

### Metadata
- Tags: readers, format-detection
- Related Files: internal/engine/reader_pti.go, internal/engine/reader_ot.go

### Resolution
- **Resolved**: 2026-06-23T12:00:00Z

---

## [LRN-20260623-020] best_practice

**Logged**: 2026-06-23T12:00:00Z
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Simpler encoder SlicingRegions value must be 2 (start + end per part), not `len(slices)`. The Ableton `.adv` format expects a `<SlicingRegions>Value>2</Value>` for each simplex slice region.

### Details
- Bug: `SlicingRegions>Value>` was set to `len(slices)` (e.g., 16)
- Fix: set to `2` — each part has exactly 2 regions (slice start + slice end)
- Verified against Ableton Live's own .adv export

### Metadata
- Tags: simpler, ableton, adv
- Related Files: internal/engine/encoder_simpler.go

### Resolution
- **Resolved**: 2026-06-23T12:00:00Z
- **Notes**: Impacted `TestSimplerReader_Basic` which was reading back the ADV file and comparing slice count.

---

## [LRN-20260623-021] best_practice

**Logged**: 2026-06-23T12:00:00Z
**Priority**: low
**Status**: resolved
**Area**: backend

### Summary
OT encoder writes 24-bit audio even when BitDepth=16, because OT hardware expects 24-bit resolution internally. BitDepth field is set to 24 unconditionally.

### Details
- OT hardware: 24-bit DAC, 16-bit input gets padded internally
- For best quality: always write 24-bit PCM in the companion WAV
- `cfg.BitDepth` is set to 24 in the OT encoder regardless of user input

### Metadata
- Tags: ot, octatrack, bitdepth
- Related Files: internal/engine/encoder_ot.go

### Resolution
- **Resolved**: 2026-06-23T12:00:00Z

---

## [LRN-20260624-022] knowledge_gap

**Logged**: 2026-06-24T08:00:00Z
**Priority**: high
**Status**: pending
**Area**: packaging

### Summary
macOS release bundles universal binary + REX Shared Library.framework in `Frameworks/` subdirectory. Binary rpath-patched to `@executable_path/Frameworks/`.

### Details
- `mise run build-releases` (mise.toml tasks.build-releases):
  1. Cross-compile Go → c-archive for amd64 + arm64 via `zig cc` as CC
  2. Zig build links Go archive + rex_bindings.zig for each arch
  3. `lipo -create` produces universal binary
  4. `install_name_tool -change` rewrites framework rpath from `@loader_path/../Frameworks/` to `@executable_path/Frameworks/`
  5. `build.zig` sets `headerpad_max_install_names = true` + `addRPath(b.path("Frameworks"))`
  6. REX framework copied into `build/Frameworks/` alongside binary
  7. Packaged as `chirashi-$VERSION-macos.tar.gz` containing `chirashi-$VERSION-macos/` dir with `chirashi` + `Frameworks/`
  8. macOS binary ad-hoc signed in release workflow (`codesign --force --sign -`)
- Manual install: extract tarball → `sudo mv` to `/usr/local/opt/chirashi` → `sudo ln -s` into PATH
- Framework resolves at `@executable_path/Frameworks/REX Shared Library.framework/...`
- `build.zig` only builds the binary; `mise run build` and `mise run build-releases` handle framework copy + rpath patching
- `CGO_LDFLAGS_ALLOW = "-Wl,-rpath,@executable_path"` env var needed for CGo linker

### Metadata
- Source: user_feedback
- Tags: macos, packaging, release, framework, rpath, codesign
- Related Files: mise.toml (tasks.build-releases, tasks.build), build.zig, .github/workflows/release.yml
- Pattern-Key: packaging.macos.framework.bundling

---

## [LRN-20260624-023] knowledge_gap

**Logged**: 2026-06-24T08:00:00Z
**Priority**: high
**Status**: pending
**Area**: packaging

### Summary
Windows release bundles `chirashi.exe` + `REX Shared Library.dll` + `.lib`. Built via Zig cross-compile with Go c-archive, packaged as ZIP.

### Details
- `mise run build-releases` builds Windows x86_64:
  1. `CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC="zig cc -target x86_64-windows-gnu" go build -buildmode=c-archive` produces `go_engine_windows.a`
  2. BSD ar format fix: `ar x ... && zig ar rcs ... && rm *.o` (Go produces BSD-format archives, lld-link needs GNU format)
  3. `zig build -Dtarget=x86_64-windows-gnu` links Go archive + REX.c (dynamic loader) + rex_bindings.zig
  4. `build.zig` adds `internal/engine/rex/REX.c` with `-DREX_WINDOWS=1 -DREX_MAC=0` flags
- DLL + .lib copied from SDK: `$REX_SDK/REXSDK_Win_1.9.2/Win/x64/Deployment/`
  - `REX Shared Library.dll` — runtime dependency
  - `REX Shared Library.lib` — import library (for dev, not needed at runtime)
- Packaged as `chirashi-$VERSION-windows.zip` containing `chirashi-$VERSION-windows/` dir
- Windows requires `REX_DLL_LOADER=1` (dynamically loaded via REX.c), unlike macOS direct framework linking
- Scoop manifest installs both exe + dll, PATH to install dir
- Manual install via PowerShell: unzip → `$env:LOCALAPPDATA\Programs\chirashi` → add to PATH

### Metadata
- Source: user_feedback
- Tags: windows, packaging, release, dll
- Related Files: mise.toml (tasks.build-releases), build.zig, .github/workflows/release.yml, internal/engine/rex/REX.c
- Pattern-Key: packaging.windows.dll.bundling

---

## [LRN-20260624-024] knowledge_gap

**Logged**: 2026-06-24T08:00:00Z
**Priority**: high
**Status**: pending
**Area**: packaging

### Summary
Linux release is pure Go — no Zig, no CGo, no REX SDK. Built from `main_linux.go` with `CGO_ENABLED=0`. Three standalone binaries (amd64, arm64, armv7).

### Details
- Entry point: `main_linux.go` with `//go:build linux && !cgo` build tag
  - Simple: imports `cmd` → calls `cmd.Execute()`
  - No CGo imports, no `//export GoMainEntry`, no empty main()
- macOS/Windows use `main.go` with `//go:build !linux`
  - Imports `"C"`, defines `//export GoMainEntry`, `main()` is empty
  - Zig binary entrypoint calls `GoMainEntry()` via c-archive
- Linux build tasks (in mise.toml tasks.build-linux and CI):
  - `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/chirashi-linux-amd64 main_linux.go`
  - Same for arm64, arm (GOARM=7 for armv7)
- Binary is pure Go, no runtime deps beyond libc — verified with `file` + `ldd`
- Homebrew formula on Linux downloads raw binary (no .tar.gz, no Frameworks)

### Metadata
- Source: user_feedback
- Tags: linux, packaging, release, cgo, go
- Related Files: main_linux.go, main.go, mise.toml, .github/workflows/ci.yml, .github/workflows/release.yml
- Pattern-Key: packaging.linux.static

---

## [LRN-20260624-025] knowledge_gap

**Logged**: 2026-06-24T08:00:00Z
**Priority**: medium
**Status**: pending
**Area**: packaging

### Summary
Release pipeline triggered by `v*` tag. Produces 3 platform artifacts: macOS tar.gz (universal + Frameworks), Windows zip (exe + DLL), Linux binaries (amd64, arm64, arm). GPG-decrypted REX SDK from committed tarballs.

### Details
- `.github/workflows/release.yml`:
  - `build-linux` job (ubuntu-latest): builds 3 Linux binaries, uploads to GitHub Release via `softprops/action-gh-release`
  - `build-and-release` job (macos-latest): builds macOS universal + Windows x86_64, signs, packages, computes CHECKSUMS.txt, creates Release
- `REX_SDK` decrypted from GPG-encrypted tarballs in `.github/workflows/secrets/`
  - macOS: `rex-sdk-macos.tar.gz.gpg` → framework → `/tmp/rexlibs/REXSDK_Mac_1.9.2/Mac/Deployment/`
  - Windows: `rex-sdk-windows.tar.gz.gpg` → DLL + .lib → `/tmp/rexlibs/REXSDK_Win_1.9.2/Win/x64/Deployment/`
  - Cache layer avoids re-decrypting on every run (`actions/cache`)
- REX SDK not decrypted on PRs from forks (no secrets access) — build uses stale cache
- macOS binary ad-hoc signed (`codesign --force --sign -`). Not notarized yet.
  - Commented-out notarization step in release.yml for future production signing
- CHECKSUMS.txt computed and included in release body
- `CGO_LDFLAGS_ALLOW: "-Wl,-rpath,@executable_path"` required for CGo rpath acceptance
- Release body auto-generated with checksums + notes about quarantine xattr

### Metadata
- Source: user_feedback
- Tags: release, ci, gpg, packaging, platform
- Related Files: .github/workflows/release.yml, .github/workflows/ci.yml, .github/workflows/secrets/
- Pattern-Key: packaging.release.pipeline

---

## [LRN-20260624-026] knowledge_gap

**Logged**: 2026-06-24T08:00:00Z
**Priority**: medium
**Status**: pending
**Area**: packaging

### Summary
`main.go` vs `main_linux.go` build-tag split: macOS/Windows use CGo + Zig c-archive entrypoint, Linux uses pure Go. Zig calls `GoMainEntry()` export; Linux just calls `cmd.Execute()`.

### Details
- `main.go` (`//go:build !linux`):
  - Imports `"C"` for `//export GoMainEntry`
  - `func main()` is empty — Zig binary has its own `main()` that calls `GoMainEntry()`
  - Zig source in `extractor.zig` calls the Go entrypoint after REX SDK init
- `main_linux.go` (`//go:build linux && !cgo`):
  - No CGo imports, no Zig dependency
  - `func main() { cmd.Execute() }` directly
  - Uses `extractor_stub.zig` (no-op stub) — but stub is never actually linked on Linux builds because `CGO_ENABLED=0` and `build.zig` step is skipped entirely
  - The Linux build bypasses `build.zig` and `zig build` completely — just `go build main_linux.go`
- This split is the fundamental architecture difference: macOS/Windows have a Go runtime embedded as a C-archive called from Zig; Linux is standalone Go

### Metadata
- Source: user_feedback
- Tags: build, cgo, zig, go, architecture
- Related Files: main.go, main_linux.go, build.zig, internal/engine/extractor.zig, internal/engine/extractor_stub.zig
- Pattern-Key: packaging.main.split

---

## [LRN-20260624-027] bug

**Logged**: 2026-06-24T18:30:00Z
**Priority**: high
**Status**: resolved
**Area**: backend

### Summary
OT encoder was incorrectly force to 24-bit (`-b 16` → 24 internally). OT hardware supports both 16 and 24-bit (24-bit DAC/ADC, user selects via FLEX FORMAT). Encoder should respect user's bit depth choice.

### Details
- v0.3.0 CHANGELOG claimed OT hardware "expects 24-bit internally" — wrong
- OT manual: "Both Flex and Static machines can handle 16 or 24 bit/44.1 kHz wav/aiff files"
- User chooses FLEX FORMAT: 16-bit or 24-bit mode in OT settings
- 24-bit DAC/ADC is a hardware capability, not a constraint on sample files
- Fix: revert the 24-bit forcing — `ForceOTSpec()` only forces 44.1kHz sample rate, OT `writeOutputFiles` clamps `cfg.BitRate` floor at 16, `EncodeWavContainer` uses clamped value

### Metadata
- Source: user_feedback
- Tags: ot, bitdepth, encoder, wav, correction
- Related Files: internal/engine/resample.go, internal/engine/runner.go, internal/engine/encoder.go
- Pattern-Key: encoder.ot.bitdepth

### Resolution
- **Resolved**: 2026-06-24T18:30:00Z
- **Notes**: Reverted 24-bit forcing. OT encoder respects user bit depth with floor clamp at 16.

---

## [LRN-20260624-028] bug

**Logged**: 2026-06-24T18:30:00Z
**Priority**: high
**Status**: resolved
**Area**: backend

### Summary
AIFF MARK chunk `markSize` calculation used `"X"` (1 char) as default empty-slice label, but actual write used `"S%02d"` (3+ chars). Result: `markSize` underreported by 2 bytes per empty-label marker → IFF chunk size wrong → downstream parsers truncate data.

### Details
- `encoder_aiff.go` markSize calculation loop (line 37-52): empty labels → `"X"` (1 char), nameLen=1, pascalLen=2, entrySize=8
- MARK write loop (line 104-114): empty labels → `fmt.Sprintf("S%02d", i+1)` (e.g. "S01"), nameLen=3, pascalLen=4, entrySize=10
- Difference: 2 bytes per marker with empty label (common for REX/WAV input without adtl labels)
- IFF parser reads chunk size from header → underreads MARK data → misparses SSND

### Metadata
- Source: audit
- Tags: aiff, mark, iff, chunk-size, corruption
- Related Files: internal/engine/encoder_aiff.go
- Pattern-Key: encoder.aiff.mark.sizemismatch

### Resolution
- **Resolved**: 2026-06-24T18:30:00Z
- **Notes**: Changed markSize loop from `for _, cp` to `for i, cp` and default label from `"X"` to `fmt.Sprintf("S%02d", i+1)` to match write path.

---

## [LRN-20260624-029] bug

**Logged**: 2026-06-24T18:30:00Z
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
AIFF encoder only supports 16-bit PCM. If `extraction.Metadata.BitDepth` is 8 or 24, the `default` case in `EncodeAIFF` writes zero-byte data (no sample data). Currently mitigated by `Force44100Spec` calling `ConvertBitDepth(extraction, 16)` but latent if called directly.

### Details
- `EncodeAIFF` bitDepth switch (encoder_aiff.go:151-168):
  - `case 16:` — correct, writes int16 samples
  - `default:` — increments `written` by `chunk` but writes nothing. File has correct header + zeroed PCM data
- Currently prevented by `Force44100Spec` setting bit depth to 16 in runner.go:193-196
- But `EncodeAIFF` is exported — direct calls with non-16 bit depth produce silent corruption

### Metadata
- Source: audit
- Tags: aiff, encoder, bitdepth, latent-bug
- Related Files: internal/engine/encoder_aiff.go, internal/engine/resample.go
- Pattern-Key: encoder.aiff.bitdepth.limited

---

## [LRN-20260624-030] bug

**Logged**: 2026-06-24T18:30:00Z
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
CAF encoder's `writeCAFPCM()` always writes int16 samples regardless of `bitDepth` parameter. If metadata bitDepth were 24, the CAF desc chunk would report 24-bit but actual PCM would be 16-bit. Mitigated by `Force44100Spec` always setting bitDepth=16.

### Details
- `writeCAFPCM()` (encoder_caf.go:212-244): hardcodes `int16(interleaved[sampleIdx] * 32768)` and writes 2 bytes per sample
- `bitDepth` parameter is used to calculate `bytesPerSample` and `blockAlign` but the actual sample type is always int16
- If `bitDepth` were set to 24, bytesPerSample=3, blockAlign=3*channels, but samples are written as 2-byte int16 — format violation
- Currently mitigated by `Force44100Spec` setting bitDepth=16 before CAF encoding

### Metadata
- Source: audit
- Tags: caf, encoder, pcm, bitdepth, latent-bug
- Related Files: internal/engine/encoder_caf.go
- Pattern-Key: encoder.caf.pcm.bitdepth

---

## [LRN-20260624-031] bug

**Logged**: 2026-06-24T18:30:00Z
**Priority**: low
**Status**: resolved
**Area**: backend

### Summary
`outputBaseName()` chunk truncation (`-o` + `-l`) operates on the full path, not just the filename. Long directory paths with chunking could be truncated at directory separator, producing unexpected output locations.

### Details
- `runner.go outputBaseName()` at line 560-569: when `-o` is set with `-l` (chunking), strips extension then truncates basename to fit format-specific `nameLimit`
- `nameLimit` for dt2pst=12, aif-op1=8 (short)
- If user passes `-o /output/dir/longname -l 64 -f dt2pst`, the path `/output/dir/longname` gets truncated to `/output/di` after nameLimit truncation
- Directory separators in the path produce unexpected output dirs
- Unlikely in practice (chunking + short-limit formats + long paths is rare combo)

### Metadata
- Source: audit
- Tags: runner, output, filename, chunking
- Related Files: internal/engine/runner.go
- Pattern-Key: runner.output.basename.truncation

---

## [LRN-20260624-032] workflow

**Logged**: 2026-06-24T19:45:00Z
**Priority**: medium
**Status**: resolved
**Area**: workflow

### Summary
Release tagging uses `~/bin/release <tag> [rev]`. Creates jj tag + pushes via `jj tag set` + `jj git push --tag`, triggers `.github/workflows/release.yml` on v* push.

### Details
- `~/bin/release` validates: tag format (`vX.Y.Z`), clean working copy, commit pushed to remote
- Default rev: `main` (jj bookmark)
- Creates tag: `jj tag set <tag> -r <rev>`
- Pushes tag: `jj git push --tag <tag>`
- Release workflow auto-triggers on tag push to origin
- Track with: `gh run watch` from repo root
- After artifacts built, update `../homebrew-tap` (SHA256) + `../scoop-bucket` (SHA256 + paths)

### Metadata
- Source: documentation
- Tags: release, workflow, jj, tag, automation
- Related Files: ~/bin/release, .github/workflows/release.yml
- Pattern-Key: workflow.release.tag

### Resolution
- **Resolved**: 2026-06-24T19:45:00Z

