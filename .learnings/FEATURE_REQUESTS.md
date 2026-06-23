# Feature Requests

## [FEAT-20260622-001] chirashi project scaffolding

**Logged**: 2026-06-22T11:00:00Z
**Priority**: medium
**Status**: completed
**Area**: infra

### Requested Capability
CLI that converts sliced instruments (REX, RX2, RCY, XRNI, ADV, ALS, ADG) into hardware sampler formats (WAV, PTI, OT, OP-1 AIFF, OP-XY preset, Elektron multi-sample, Digitakt II preset).

### User Context
Forked from `rexconverter` (g-lok). Rebranded to `chirashi` v0.1.0. REX SDK is proprietary, encrypted at rest with GPG keypair, decrypted at CI build time.

### Complexity Estimate
complex

### Suggested Implementation
- Go CLI using cobra (cmd/root.go)
- CGo bridge to Zig (internal/engine/bridge.go) calling REX SDK (macOS framework, Windows DLL)
- Zig extractor (internal/engine/extractor.zig) wraps REX API
- Go encoders for each output format (encoder_wav.go, encoder_pti.go, etc.)
- GitHub Actions CI with encrypted SDK secrets (.github/workflows/secrets/*.tar.gz.gpg)
- mise.toml for tool versioning (go 1.26.3, zig 0.16.0)
- Poetry + graphifyy for code analysis

### Resolution
- **Resolved**: 2026-06-22
- **Notes**: Full project at /home/g/Projects/chirashi. Build works on both macOS and Linux. CI runs with REX SDK decrypted from GPG secrets.

---

## [FEAT-20260622-002] graphify knowledge graph for chirashi

**Logged**: 2026-06-22T11:30:00Z
**Priority**: low
**Status**: completed
**Area**: docs

### Requested Capability
Generate knowledge graph of chirashi codebase to understand architecture, find god nodes, surface surprising connections.

### User Context
Want to navigate the rebranded chirashi codebase before making major changes. Understand Go+Zig+CGo integration patterns, encoder architecture, REX SDK call sites.

### Complexity Estimate
medium

### Suggested Implementation
- Local Poetry project with graphifyy[gemini] dep
- Run AST extraction (no LLM needed for code-only corpus)
- Optional: add Gemini API key for semantic extraction
- Output: graph.html (interactive), GRAPH_REPORT.md, graph.json
- `.graphifyignore` to exclude .venv and graphify-out
- `.gitignore` to keep graphify-out uncommitted

### Resolution
- **Resolved**: 2026-06-22T13:00:00Z
- **Notes**: Code-only AST extraction ran successfully: 475 nodes, 876 edges, 25 communities. Output in `graphify-out/` (gitignored).
