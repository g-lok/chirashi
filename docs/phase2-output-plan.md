# Phase 2 — Output Encoder Expansion Plan

**Goal:** rexconverter can WRITE every format it can READ. Only REX is excluded
(REX SDK is read-only).

---

## Status Today

| Write? | Format | Encoder | Notes |
|--------|--------|---------|-------|
| ✅ | WAV | `encoder.go` | fmt+data+cue chunks |
| ✅ | PTI | `encoder_pti.go` | 392-byte header + 44.1k/16 mono PCM |
| ✅ | OT | `encoder_ot.go` | 0x340-byte sidecar + WAV |
| ✅ | AIF-OP1 | `encoder_op1.go` | OP-1 AIFF w/ APPL(op-1 JSON) |
| ✅ | XY | `encoder_xy.go` | ZIP: patch.json + per-slice WAVs |
| ✅ | EL | `encoder_el.go` | Text sidecar + WAV |
| ✅ | DT2 | `encoder_d2pst.go` | ZIP: manifest + WAV + TLV |
| ✅ | XRNI | `encoder_xrni.go` | Renoise instrument (ZIP: XML + WAV) |
| ✅ | ADV/ALS | `encoder_simpler.go` | Ableton Simpler preset / Live Set |
| ✅ | ADG | `encoder_adg.go` | Ableton Drum Rack |
| ✅ | AIFF | `encoder_aiff.go` | Standard AIFF (no OP-1 metadata) |

---

## Common Architecture

Every output encoder in `internal/rexengine/encoder_<fmt>.go`:

```
func Encode<Format>(w io.Writer, extraction *SliceExtraction) error
```

But XRNI/ADV/ADG are **composite outputs**: each produces a directory/ZIP
containing both audio files AND a metadata file (XML/JSON). These share a
pattern:

```
writeOutputDir(basePath, extraction, cfg)
  ├── audio files (WAV/FLAC per-slice or monolithic)
  └── metadata (Instrument.xml / ADV GZip XML / ADG GZip XML)
```

---

## Format Specs

### 1. XRNI Encoder — `encoder_xrni.go`

**Container:** ZIP (no compression for WAV, Deflate for XML)

**Structure:**
```
output.xrni
├── Instrument.xml          # Renoise instrument definition
└── <hash>.wav              # Monolithic PCM WAV
```

**Instrument.xml schema:**
```xml
<RenoiseInstrument doc_version="34">
  <Name>Instrument Name</Name>
  <SampleGenerator><Samples>
    <Sample>
      <IsAlias>false</IsAlias>
      <FileName>//File:hash.wav</FileName>
      <SliceMarkers>
        <SliceMarker><SamplePosition>9433</SamplePosition></SliceMarker>
        ...
      </SliceMarkers>
      <Mapping><BaseNote>36</BaseNote></Mapping>
    </Sample>
    <Sample>
      <IsAlias>true</IsAlias>
      <Mapping><BaseNote>37</BaseNote></Mapping>
      <LoopEnd>9427</LoopEnd>
    </Sample>
    ...
  </Samples></SampleGenerator>
</RenoiseInstrument>
```

- First `<Sample>` is the audio source (`<IsAlias>false</IsAlias>`), references monolithic WAV
- Each alias `<Sample>` is one slice (`<IsAlias>true</IsAlias>`), has BaseNote for MIDI mapping
- `<SamplePosition>` values are cumulative sample-offset slice boundaries (N1, N2, ..., total)
- BaseNote starts at 36 (C2) and increments per alias
- LoopEnd in each alias = slice duration in frames
- WAV (not FLAC) used for audio — Renoise handles both, FLAC encoding would add dependency
- `<IsAlias>` uses element syntax (not attribute) to match Go XML decoder (`xml:"IsAlias"`)

**CLI:**
- `--format xrni`
- Each cue-group → one `.xrni` file

### 2. ADV/ALS Encoder — `encoder_simpler.go`

**Container:** GZip XML (same as input)

**Structure:**
```
output.adv (or output.als)
├── GZip header
└── Ableton XML document
```

Output is written alongside companion WAV file(s). For multi-slice output:

```
output/
├── output.adv
└── Samples/Imported/
    └── output.wav
```

**ADV XML schema:**
```xml
<Ableton MajorVersion="5">
  <OriginalSimpler>
    <SampleRef>
      <FileRef>
        <Path Value="Samples/Imported/output.wav"/>
        <RelativePath Value="Samples/Imported/output.wav"/>
      </FileRef>
      <DefaultDuration><Value>88200</Value></DefaultDuration>
      <DefaultSampleRate><Value>44100</Value></DefaultSampleRate>
    </SampleRef>
    <Player>
      <MultiSampleMap>
        <SampleParts>
          <MultiSamplePart HasImportedSlicePoints="true">
            <Name><Value>Slice 01</Value></Name>
            <SampleRef>
              <FileRef>
                <Path Value="Samples/Imported/output.wav"/>
              </FileRef>
              <DefaultDuration><Value>44100</Value></DefaultDuration>
              <DefaultSampleRate><Value>44100</Value></DefaultSampleRate>
            </SampleRef>
            <InitialSlicePointsFromOnsets>
              <SlicePoint TimeInSeconds="0" Rank="0"/>
              <SlicePoint TimeInSeconds="0.6026" Rank="0"/>
            </InitialSlicePointsFromOnsets>
          </MultiSamplePart>
        </SampleParts>
      </MultiSampleMap>
    </Player>
  </OriginalSimpler>
</Ableton>
```

- One `<MultiSamplePart>` per slice
- Each part gets its own `<InitialSlicePointsFromOnsets>` with start and end times (cumulative offsets in seconds)
- Each part also has `<SampleRef>` referencing the same monolithic WAV
- Companion WAV placed in `Samples/Imported/` relative to ADV
- Field values use element syntax (`<Name><Value>X</Value></Name>`) not `<Name Value="X"/>` attribute

**ALS variant:** Wraps same Simpler in Live Set:
```xml
<Ableton MajorVersion="5">
  <LiveSet>
    <Tracks>
      <MidiTrack>
        <DeviceChain>
          <Devices>
            <OriginalSimpler>…</OriginalSimpler>
          </Devices>
        </DeviceChain>
      </MidiTrack>
    </Tracks>
  </LiveSet>
</Ableton>
```

ALS encoder reuses ADV Simpler XML — just wraps it in LiveSet boilerplate.

### 3. ADG Encoder — `encoder_drumrack.go`

**Container:** GZip XML

**Structure:**
```
output/
├── output.adg
└── Samples/Imported/
    ├── pad_01.wav
    ├── pad_02.wav
    └── ...
```

**ADG XML schema:**
```xml
<Ableton MajorVersion="5">
  <GroupDevicePreset>
    <Device>
      <DrumGroupDevice>
        <!-- optional DrumPadsListWrapper -->
      </DrumGroupDevice>
    </Device>
    <BranchPresets>
      <DrumBranchPreset>
        <DevicePresets>
          <AbletonDevicePreset>
            <Device>
              <DrumCell>
                <UserSample>
                  <Value>
                    <SampleRef>
                      <FileRef>
                        <Path Value="Samples/Imported/pad_01.wav"/>
                      </FileRef>
                    </SampleRef>
                  </Value>
                </UserSample>
              </DrumCell>
            </Device>
          </AbletonDevicePreset>
        </DevicePresets>
        <ZoneSettings ReceivingNote="36"/>
      </DrumBranchPreset>
      <!-- N branches, one per slice/pad -->
    </BranchPresets>
  </GroupDevicePreset>
</Ableton>
```

- Each slice becomes a `<DrumBranchPreset>` with one `<DrumCell>`
- `ReceivingNote` increments from 36 (C2) upward
- Each pad references its own WAV file in `Samples/Imported/`
- DMIs can use `Zones>` for velocity layering (extended)

### 4. Standard AIFF Encoder — `encoder_aiff.go`

**Container:** FORM/IFF (same as OP-1 but without APPL(op-1) chunk)

**Chunks:**
```
FORM <size> AIFF
  COMM <18 bytes>    # channels, frames, bitdepth, sampleRate
  MARK <var>         # optional: marker names + positions
  SSND <var>         # PCM audio data
```

- No APPL(op-1) metadata — plain AIFF
- Supports 8/16/24-bit, mono/stereo
- MARK chunk if extraction has CuePoints

---

## CLI Design — Sample Path Control

The `--library-path` flag already exists but isn't wired into readers. Need:

### Input side
- `resolveAndDecode` currently tries:
  1. The path as-is
  2. Path + each known extension
- **Add:** when path not found, try:
  3. `libraryPath + "/" + relativePath` (Ableton convention)
  4. `libraryPath + "/Samples/Imported/" + basename`
  5. `libraryPath + "/Samples/" + basename`
- This requires `resolveAndDecode` to accept `libraryPath` parameter, or use
  a package-level variable set at init time

### Output side
- New flag: `--sample-path-mode` (values: `relative`, `absolute`, `library`)
  - `relative` (default): `Samples/Imported/file.wav`
  - `absolute`: Full path to companion WAV
  - `library`: Path relative to `--library-path`
- Companion WAV placement:
  - `relative` mode: WAVs go in `Samples/Imported/` subdirectory
  - `absolute` mode: WAVs go next to metadata file
  - `library` mode: WAVs go in `--library-path/Samples/Imported/`

### Flag summary additions

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--library-path` | string | `""` | Base dir for Ableton sample resolution (input) + output placement |
| `--sample-path-mode` | string | `relative` | How sample paths written in XML output: relative, absolute, library |

---

## Implementation Priority

| Order | Format | Effort | Complexity | Value |
|-------|--------|--------|------------|-------|
| 1 | Standard AIFF | small | low (copy OP-1 minus JSON) | fills gap immediately |
| 2 | XRNI | medium | medium (FLAC encode) | full round-trip for Renoise |
| 3 | ADV/ALS | large | medium (XML generation) | Ableto→others + others→Ableton |
| 4 | ADG | large | medium | Ableton Drum Rack round-trip |
| 5 | Library path wiring | medium | small | unlocks ADV/ADG input on other machines |

---

## Changes Required

### New files
- `internal/rexengine/encoder_aiff.go` — standard AIFF encoder
- `internal/rexengine/encoder_xrni.go` — XRNI ZIP+XML+WAV encoder
- `internal/rexengine/encoder_simpler.go` — ADV/ALS GZip XML encoder
- `internal/rexengine/encoder_adg.go` — ADG GZip XML encoder

### Changed files
- `internal/rexengine/runner.go` — add format cases in `writeOutputFiles()`,
  `deviceMaxSlices`, `fileNameLimit`, `SetLibraryPath` call
- `cmd/root.go` — add new formats to `--format` help, add `--sample-path-mode`
- `internal/rexengine/reader.go` — add `SetLibraryPath()`, `sampleLibraryPath` var
- `internal/rexengine/reader_simpler.go` — wire `libraryPath` into
  `resolveAndDecode` (shared with Drum Rack reader)
- `internal/rexengine/types.go` — add `SamplePathMode` to `PipelineConfig`

### Tests
- `internal/rexengine/encoder_output_test.go` — 10 unit tests: AIFF (3), XRNI (2),
  ADV (1), ALS (1), ADG (1), fileNameLimit, deviceMaxSlices

---

## Edge Cases

1. **XRNI with >128 slices** — Renoise supports 128 slices max; clamp and warn
2. **ADV with single slice** — no slicing metadata, just SampleRef
3. **ADG with >128 pads** — Drum Rack has 128 pads on one channel; split to
   multiple racks or clamp
4. **Empty CuePoints** — write monolithic sample (no slice markers)
5. **Cross-platform paths** — always use forward slashes in XML
6. **FLAC encode failure** — fall back to WAV in XRNI ZIP
7. **GZip recompression** — ensure ADV/ADG output passes `gunzip` validation
8. **Library path not found** — log warning, continue with direct path
