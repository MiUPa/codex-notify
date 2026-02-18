# codex-notify

`codex-notify` is a macOS-first notification bridge for Codex CLI.

- macOS only (MVP)
- Go single binary
- Safe config setup with backup
- Commercial use allowed (`Apache-2.0`)

## What It Solves

Codex already supports notifications, but behavior can depend on terminal support.
`codex-notify` gives you a direct macOS desktop notification path via Codex `notify` hook.

## Install (Homebrew)

```bash
brew tap MiUPa/homebrew-codex-notify
brew install codex-notify
```

`Formula/codex-notify.rb` in this repository is the source template for your tap repository (`MiUPa/homebrew-codex-notify`).

## Quick Start

1) Initialize Codex config:

```bash
codex-notify init
```

2) Validate setup:

```bash
codex-notify doctor
```

3) Send a test notification:

```bash
codex-notify test "Codex通知テスト"
```

## Commands

```bash
codex-notify init [--replace] [--config path]
codex-notify doctor [--config path]
codex-notify test [message]
codex-notify hook [json-payload]
codex-notify uninstall [--restore-config] [--config path]
```

## How `init` Works

- Detects `~/.codex/config.toml`
- Creates timestamped backup before edits
- Adds `notify = ["codex-notify", "hook"]`
- Refuses to overwrite existing `notify` unless `--replace` is specified
- Keeps repeated runs idempotent

## Example Codex Config

```toml
notify = ["codex-notify", "hook"]
```

## Uninstall

Restore from the latest backup:

```bash
codex-notify uninstall --restore-config
```

## Event Support

- `agent-turn-complete`: supported
- `approval-requested`: best effort (depends on Codex event delivery to `notify`)
- Unknown events: generic notification

## Development

```bash
make build
make test
```

## Release

```bash
# 1) tag and push
git tag v0.1.0
git push origin v0.1.0

# 2) after GitHub Release is created, generate Formula snippet
./scripts/gen_formula.sh v0.1.0
```

Use the generated output to update `Formula/codex-notify.rb` in your tap repo.

## License

Apache-2.0. See `LICENSE`.
