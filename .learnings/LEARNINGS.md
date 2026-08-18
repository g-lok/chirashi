# Learnings

Corrections, insights, and knowledge gaps captured during development.

**Categories**: correction | insight | knowledge_gap | best_practice

---

## [LRN-20260817-DT2] insight

**Logged**: 2026-08-17T18:00:00Z
**Priority**: low
**Status**: resolved
**Area**: backend

### Summary
Removed hardcoded name artifact in DT2 binary template

### Details
Reverse-engineered binary formats often contain strings from the original reference file. In `internal/engine/encoder_dt2pst.go`, the sequence `"SOLE DISPLAY"` was present in the bytecode header. While technically overwritten by dynamic logic, it created confusion and potential for partial name leakage if the overwrite logic failed.

### Suggested Action
Replace string literals in binary templates with `0x00` padding to clarify they are variable slots.

### Metadata
- Source: code_audit
- Related Files: internal/engine/encoder_dt2pst.go
- Tags: dt2, digitakt, binary-format

---

## [LRN-20260817-BATCH] best_practice

**Logged**: 2026-08-17T17:00:00Z
**Priority**: high
**Status**: promoted
**Area**: backend

### Summary
Implemented fault-tolerant batch processing for directory conversion

### Details
When processing thousands of files (e.g., Rhythm Lab collection), a single corrupted or redirected download (HTML page) can halt the entire pipeline. 
1. **Runner Fault Tolerance**: Wrapped file processing in a loop that logs errors to `os.Stderr` and continues.
2. **Explicit Format Validation**: Added explicit detection for `<!DOCTYPE html>` to identify failed web downloads early.
3. **Contextual Errors**: Enhanced errors to include the source filename in batch output.

### Suggested Action
Always prefer "log and continue" for batch CLI operations unless the error is systemic (e.g., Disk Full).

### Metadata
- Source: user_feedback
- Related Files: internal/engine/runner.go, internal/engine/rex2/reader.go
- Tags: batch-processing, robustness, fault-tolerance
- Promoted: AGENTS.md, CHANGELOG.md

---

## [LRN-20260812-REX-CODE] insight

**Logged**: 2026-08-12T12:05:00Z
**Priority**: critical
**Status**: promoted
**Area**: backend

### Summary
Reverse-engineered bit-perfect REX2 DWOP encoding via bitstream parity analysis

### Details
Achieving bit-parity with the original REX2 SDK requires exact symmetry with the decoder's internal state machine:

1. **Predictor Symmetry**: 
   The encoder's `predictorResidual` must exactly invert the decoder's `applyPredictor`. 
   - Case 1: `res = sample - d0`
   - Case 2: `res = (sample - d0) - d1`
   - Case 3: `res = ((sample - d0) - d1) - d2`
   - Case 4: `res = (((sample - d0) - d1) - d2) - d3`
   Previous implementation used an incorrect cumulative subtraction order.

2. **Stereo Channel Coupling**:
   Stereo frames are encoded as `Left` followed by `Delta`. The bitstream for the Delta channel starts immediately after the Left channel's variable-length code, sharing the same 32-bit word boundary until the end of the SDAT chunk.

3. **Word Alignment Padding**:
   The SDAT chunk payload must be padded with zeros to a 4-byte (32-bit) boundary. Failure to pad causes ReCycle to reject the file because the DWOP stream reader expects full words.

4. **SLCE Chunk Metadata**:
   Slices must be sorted by `SampleStart` in the SLCL container.

### Suggested Action
Always use bit-by-bit bitstream comparison against known-good SDK files when modifying the DWOP state machine.

### Metadata
- Source: exploration
- Related Files: internal/engine/rex2/encoder.go, internal/engine/rex2/dwop.go
- Tags: rex2, compression, audio, reverse-engineering
- Promoted: AGENTS.md, README.md

---

## [LRN-20260812-V110] best_practice

**Logged**: 2026-08-12T12:15:00Z
**Priority**: medium
**Status**: promoted
**Area**: backend

### Summary
Implemented "Open Ecosystem" formats (SFZ, Decent Sampler, MPC XPM)

### Details
1. **SFZ**: Uses `offset` and `end` opcodes to map slices in a companion WAV. This avoids splitting audio into multiple files and preserves high fidelity.
2. **Decent Sampler**: XML-based (.dspreset). Maps regions using `start` and `end` attributes.
3. **Akai MPC XPM**: Maps up to 128 slices to Pads/Instruments. Critical for modern MPC hardware workflow.
4. **Sample Rate Auto-Detection**: Pipeline now defaults to source rate if -s is omitted, improving UX for multi-rate libraries.

### Suggested Action
Verify XPM compatibility with MPC Software 2.11+.

### Metadata
- Source: feature_expansion
- Related Files: internal/engine/encoder_sfz.go, internal/engine/encoder_ds.go, internal/engine/encoder_xpm.go
- Tags: sfz, mpchardware, decentsampler
- Promoted: README.md, AGENTS.md

---

## [LRN-20260813-CI] correction

**Logged**: 2026-08-13T20:25:00Z
**Priority**: high
**Status**: promoted
**Area**: infra

### Summary
Fix Windows-specific glob failure in release checksum generation

### Details
GitHub Actions release workflow failed on Windows matrix job because `sha256sum *.tar.gz *.zip` attempted to match both extensions. Windows only produces `.zip`, causing `*.tar.gz` to fail the shell command.

### Suggested Action
Use single wildcard `sha256sum filename.*` or ignore missing files in CI scripts.

### Metadata
- Source: ci_failure
- Related Files: .github/workflows/release.yml
- Tags: github-actions, ci, windows
- Promoted: AGENTS.md
