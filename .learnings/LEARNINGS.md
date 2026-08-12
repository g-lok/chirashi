# Learnings

Corrections, insights, and knowledge gaps captured during development.

**Categories**: correction | insight | knowledge_gap | best_practice

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
**Status**: pending
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
