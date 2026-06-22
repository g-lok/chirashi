# Contributing

Thanks for your interest in rexconverter!

## Getting Started

1. Read `AGENTS.md` for architecture and build instructions
2. Ensure you have the [REX SDK v1.9.2](https://www.reasonstudios.com/) installed
3. Build the project: `mise run build`
4. Run tests: `go test ./tests/...`

## Pull Requests

- Keep changes focused. One PR = one feature or fix.
- Add or update tests for any new functionality.
- Run `go test ./tests/...` before submitting.
- If adding a CLI flag, follow the pattern in `AGENTS.md`.

## Code Style

- **Go**: `gofmt` (no opinionated formatter required, just clean code)
- **Zig**: `zig fmt` on `extractor.zig`
- No comments unless the "why" isn't obvious from the code
- Match naming conventions of surrounding code

## Testing

Tests live in `tests/` as a separate Go package (no CGo). They run the built binary
as a subprocess, so build first.

```bash
mise run build
go test ./tests/...
```

## REX SDK

The REX SDK is proprietary. See `REX_SDK_LICENSE.txt` and `NOTICE.md` for terms.
Do not submit PRs that depend on copyleft-licensed libraries.
