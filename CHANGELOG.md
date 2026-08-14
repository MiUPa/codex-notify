# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

## [0.4.0] - 2026-08-11

### Fixed
- Suppressed popup notifications entirely when the configured Codex terminal/IDE is already frontmost, instead of flashing a popup over the active Codex window.
- Added session-scoped suppression for wrapped Ghostty and Terminal.app sessions so a notification is skipped only when the same source session is frontmost.
- Scoped approval-interaction suppression to each thread so one Codex session cannot hide another session's notifications.
- Preserved config file permissions and recognized legacy timestamped config backups during restore.
- Made popup actions asynchronous so a slow AppleScript action cannot freeze the popup timer.
- Made local builds select the host Mac architecture by default.

### Added
- Added `CODEX_NOTIFY_SUPPRESS_FRONTMOST_BUNDLE_IDS` for app-level popup suppression across multiple frontmost apps such as Ghostty, Terminal, VS Code, and Zed.
- Added `codex-notify launch -- codex ...` to inject source-session metadata into Codex for exact Ghostty/Terminal frontmost matching.

## [0.3.5] - 2026-03-15

### Fixed
- Fixed GitHub Actions release and tap-sync workflows so Homebrew token checks no longer fail during job selection.
- Fixed tap sync automation to regenerate the Formula from the latest published release instead of copying a stale repository snapshot.

## [0.3.4] - 2026-03-15

### Changed
- Popup window now uses a wider fixed frame with stable dimensions before `Read more`, longer preview text, and tighter inline action spacing.
- Single neutral actions such as `Open` now render as inline text links instead of large footer buttons.
- Popup header controls are simplified by removing the bolt icon and using a plain `...` menu label.

### Fixed
- Fixed release automation so Formula generation no longer executes embedded backticks.
- Fixed payload preview rendering by stripping Markdown link targets and code ticks before display, and truncating by rune count so Japanese text previews stay readable.
- Fixed popup visibility across Spaces/desktops by making the popup panel stationary while its timeout is active.

## [0.3.2] - 2026-02-24

### Fixed
- Fixed config file parsing error with Codex CLI 0.104.0 by ensuring `notify` hook is placed at TOML root level (not inside sections).
- `codex-notify init` now correctly inserts `notify` directive before any TOML sections.

## [0.3.1] - 2026-02-23

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
