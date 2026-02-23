# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Changed
- Unified notification UI to bottom-right popup by default for all events (`test`, `agent-turn-complete`, `approval-requested`, unknown events).
- Added `CODEX_NOTIFY_NOTIFICATION_UI` (`popup`/`system`) to control popup usage globally.

## [0.3.0] - 2026-02-23

### Changed
- `approval-requested` now defaults to popup UX (`popup`) with visible choice buttons instead of notification action menus.
- Popup buttons are now dynamic: if payload has two choices (for example `yes/no`), only two buttons are shown.
- Refined popup visual style with semantic action button colors, subtle entrance/exit animation, and timeout progress bar.
- Added `codex-notify action submit --text ...` so unknown option labels can still be sent as typed input.
- Added `single` as an alias of `popup` for backward compatibility.
- Added `CODEX_NOTIFY_ENABLE_POPUP_APPROVAL_ACTIONS` (with legacy alias `CODEX_NOTIFY_ENABLE_NATIVE_APPROVAL_ACTIONS`) and `CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS`.

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
