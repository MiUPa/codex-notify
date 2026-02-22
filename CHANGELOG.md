# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Changed
- `approval-requested` now defaults to a single notification that opens a chooser dialog (`Open`, `Approve`, `Reject`) on click.
- Added `codex-notify action choose` for chooser-based handling.
- Added `CODEX_NOTIFY_APPROVAL_UI` (`single`/`multi`) to switch between single-notification and legacy multi-notification behavior.

## [0.2.1] - 2026-02-20

### Fixed
- Accept absolute-path `notify` command forms (for example `/usr/local/bin/codex-notify`) in config detection.

## [0.2.0] - 2026-02-20

### Added
- Actionable approval notifications:
  - Open terminal action
  - Approve action (send key sequence)
  - Reject action (send key sequence)
- New command: `codex-notify action <open|approve|reject> [--thread-id id]`
- Configurable action environment variables:
  - `CODEX_NOTIFY_TERMINAL_BUNDLE_ID`
  - `CODEX_NOTIFY_APPROVE_KEYS`
  - `CODEX_NOTIFY_REJECT_KEYS`
  - `CODEX_NOTIFY_ENABLE_APPROVAL_ACTIONS`

## [0.1.1] - 2026-02-19

### Fixed
- Tightened hook detection to avoid false positives from unrelated `notify` values.

## [0.1.0] - 2026-02-19

### Added
- Initial macOS MVP:
  - `init`, `doctor`, `test`, `hook`, `uninstall`
  - Safe config backup and restore
  - Homebrew distribution support
