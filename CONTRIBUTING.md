# Contributing to chirashi

## Development setup

chirashi uses [mise](https://mise.jdx.dev/) for tool versioning.

```bash
git clone https://github.com/g-lok/chirashi.git
cd chirashi
mise install
```

## Workflow

- Use [jj](https://jj-vcs.dev/) (Jujutsu), not raw git
- Create a feature rev off `main@origin`
- Run tests before pushing: `mise run test`
- Push with `jj git push --bookmark <branch>`
- Open a PR against `main`

## Code style

- Go: `gofmt`, no `golint` warnings
- Zig: `zig fmt`
- Commit messages: Conventional Commits (see `~/.config/opencode/AGENTS.md`)
- No AI attribution in commit messages

## Testing

- `tests/reader_test.go` — reader unit tests
- `tests/encoder_format_test.go` — encoder tests via the binary
- `tests/integration_test.go` — end-to-end via the binary
- `tests/processor_test.go` — slice processing unit tests

Run with `CGO_ENABLED=0`:

```bash
CGO_ENABLED=0 go test ./tests/...
```

## Architecture

See [AGENTS.md](AGENTS.md) for project-specific architecture notes and constraints.

## Adding a new format

### New output format

1. Add `internal/engine/encoder_<format>.go` with `Encode<Format>(w, extraction, cfg) error`
2. Register in `runner.go` `writeOutputFiles` switch
3. Add format to `cmd/root.go` `--format` flag description
4. Add test in `tests/encoder_format_test.go`

### New input format

1. Add `internal/engine/reader_<format>.go` with a `Reader` implementing the `Reader` interface
2. Register extension in `reader.go` dispatch table
3. Add test in `tests/reader_test.go`

## Reporting issues

Open an issue on GitHub. Include:
- chirashi version (`chirashi --version`)
- OS + architecture
- Input file (or sample if REX — REX is proprietary, share only if you have rights)
- Command + full output
