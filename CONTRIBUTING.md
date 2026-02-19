# Contributing to codex-notify

Thanks for contributing.

## Development Setup

Requirements:
- macOS
- Go toolchain (version from `go.mod`)
- `terminal-notifier` for local manual testing (optional, `osascript` fallback exists)

Clone and run:

```bash
make build
make test
```

Binary path:
- `bin/codex-notify`

## Code Style

- Keep changes focused and small.
- Prefer explicit error messages.
- Keep behavior backward compatible unless a breaking change is intentional.
- Update `README.md` when commands or behavior change.

## Pull Requests

Before opening a PR:
- Run `make test`.
- Verify `codex-notify help` output if CLI options changed.
- Add or update docs (`README.md`, `docs/RELEASING.md`) as needed.
- Add a changelog entry in `CHANGELOG.md`.

## Release Notes

Releases are tag-driven.
- Push tag: `git tag vX.Y.Z && git push origin vX.Y.Z`
- This creates GitHub release artifacts via workflow.
- Then update Homebrew tap formula (`MiUPa/homebrew-codex-notify`) using `scripts/gen_formula.sh`.
