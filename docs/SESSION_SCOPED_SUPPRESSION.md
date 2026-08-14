# Session-Scoped Suppression Design

Status: proposed

## Problem

`codex-notify` currently suppresses notifications at app level.
If Ghostty, Terminal, VS Code, or Zed is frontmost, every Codex notification is suppressed, even when the notification originated from a different session.

That is too coarse for multi-session workflows.

## Current Implementation Boundary

The current launcher passes source-session metadata through the Codex process
environment and replaces itself with the launched command on macOS. It does not
yet maintain the registry proposed below. The registry remains a future option
only if environment inheritance proves insufficient in real terminal/IDE flows.

Example:

- Ghostty window A runs Codex session A
- Ghostty window B runs Codex session B
- window A is frontmost
- session B finishes

Desired behavior:

- suppress notifications only when the frontmost session is the same session that produced the notification

## Goals

- Support session-scoped suppression where the host app exposes enough metadata
- Preserve current app-level suppression as a fallback
- Support multiple apps with a common core and app-specific adapters
- Keep notification behavior deterministic and local-only
- Avoid depending on unsupported Codex internals such as tailing session files

## Non-Goals

- Perfect session-scoped suppression for every IDE or terminal emulator
- Deep editor integration for apps that do not expose focused terminal/session metadata
- Replacing Codex upstream notify payload design

## Constraints

### Codex notify payload is not enough today

The current hook payload provides event-level data such as:

- event type
- thread id
- preview message

It does not provide a stable origin session identifier for the host UI session.

That means `codex-notify hook` cannot infer the source terminal/editor session by payload alone.

### App automation capabilities vary

- Terminal.app exposes selected tab and `tty`
- Ghostty exposes window/tab/terminal ids via AppleScript
- VS Code can expose active terminal metadata through an extension
- Zed currently does not appear to expose enough official focused-terminal metadata for strict matching

Because of this, the design must be adapter-based.

## Proposed Architecture

The system has three layers:

1. Source session capture
2. Frontmost session detection
3. Match policy and fallback

### 1. Source Session Capture

Codex should be launched through a lightweight wrapper:

```bash
codex-notify launch -- codex
```

The proposed registry design would capture metadata from the session that launches Codex and store a local registry entry before `codex` starts. The current implementation passes the captured metadata through the child process environment instead.

The wrapper then execs the real `codex` process.

#### Why a launcher is needed

The notify hook runs later, often in a different process context.
Without a launcher, there is no reliable source-of-truth that says:

- which app launched this Codex process
- which terminal/editor session it belonged to
- which session-specific identifier should be matched later

### 2. Frontmost Session Detection

When `codex-notify hook` runs, it resolves the source session entry and asks an app-specific adapter for the current frontmost session descriptor.

Each adapter returns a normalized descriptor such as:

- app kind
- bundle id
- terminal/session id if available
- tty if available
- working directory if useful
- window/tab ids if useful

### 3. Match Policy

If source session and frontmost session match at the strongest available level, suppress the notification.

If strict matching is not available for that app, fall back in this order:

1. session-level match
2. terminal/tab/window-level match
3. app-level match
4. no suppression

This lets `codex-notify` support many apps without claiming equal precision across all of them.

## Core Data Model

Suggested registry record:

```json
{
  "session_id": "uuid",
  "created_at": "2026-03-21T12:34:56Z",
  "expires_at": "2026-03-21T20:34:56Z",
  "host_kind": "ghostty",
  "bundle_id": "com.mitchellh.ghostty",
  "workspace_cwd": "/Users/Y.Miura/works/foo",
  "codex_pid": 12345,
  "launcher_pid": 12340,
  "ppid_at_launch": 12000,
  "terminal_id": "ghostty-terminal-id",
  "tab_id": "ghostty-tab-id",
  "window_id": "ghostty-window-id",
  "tty": "/dev/ttys012",
  "title_hint": "repo-name",
  "suppression_mode": "session"
}
```

Notes:

- not every field will exist for every app
- matching should use the strongest available field for the adapter
- records should expire automatically

## Runtime Layout

Suggested runtime location:

```text
~/Library/Caches/codex-notify/
```

Suggested files:

- `sessions/<session_id>.json`
- `pids/<codex_pid>.session`
- `apps/<adapter>.json` for adapter caches if needed

The `pids/` indirection allows the hook path to resolve source metadata from process lineage without trusting notify payload shape.

## Source Resolution Strategy

Primary strategy:

1. launcher stores mapping from `codex_pid` to `session_id`
2. hook resolves the active Codex-related pid lineage
3. hook loads the source session record

Secondary strategy:

- allow explicit environment variables such as `CODEX_NOTIFY_SOURCE_SESSION_ID`

But environment inheritance should be treated as opportunistic, not authoritative.

## Adapter Interface

Each app-specific adapter should implement a common interface conceptually equivalent to:

```text
CaptureLaunchContext() -> SourceSession
CurrentFrontmostSession() -> FrontmostSession
Match(source, frontmost) -> MatchResult
```

Where `MatchResult` contains:

- `matched: bool`
- `strength: session | terminal | tab | window | app | none`
- `reason: string`

## Adapter Plan

### Terminal.app

Feasibility: high

Launch capture:

- store `tty`
- optionally store window id and selected tab index

Frontmost detection:

- query frontmost Terminal window
- read selected tab
- read selected tab `tty`

Match:

- strict match on `tty`
- fallback to app-level only if configured

### Ghostty

Feasibility: high

Launch capture:

- query front window
- query selected tab
- query focused terminal
- store Ghostty `terminal.id`
- optionally store `tab.id`, `window.id`, `working directory`

Frontmost detection:

- query front window -> selected tab -> focused terminal

Match:

- strict match on `terminal.id`
- fallback to `tab.id`, then `window.id`, then app-level if configured

Important:

- Ghostty AppleScript support is still relatively new, so this adapter should be isolated behind one module

### VS Code

Feasibility: medium, but likely requires companion extension

Launch capture:

- wrapper stores shell-side context
- a VS Code extension publishes active terminal metadata to local registry
- source session stores extension-provided terminal identifier or `processId`

Frontmost detection:

- extension publishes the current active terminal when VS Code is frontmost

Match:

- strict match on terminal identity from the extension

Without the extension:

- only app-level suppression is realistic

### Zed

Feasibility: low to medium today

Launch capture:

- shell can inject environment variables or title markers into the integrated terminal

Frontmost detection:

- no strong official session-level API confirmed yet

Fallback options:

- app-level suppression
- optional title-token heuristic

Recommendation:

- keep Zed at app-level until a stable focused-terminal API exists

### Generic Terminal/IDE Adapter

Feasibility: low precision

Generic support can be attempted through:

- `tty`
- window title token
- working directory

But this should be explicitly labeled heuristic-only.

It should never be presented as strict session matching.

## Config Model

Recommended new config concepts:

- `CODEX_NOTIFY_SUPPRESSION_MODE=auto|session|app|off`
- `CODEX_NOTIFY_SUPPRESSION_ADAPTERS=terminal,ghostty,vscode,zed`
- `CODEX_NOTIFY_SUPPRESSION_FALLBACK=app|none`

Optional advanced settings:

- `CODEX_NOTIFY_ENABLE_HEURISTIC_MATCHING=0|1`
- `CODEX_NOTIFY_SOURCE_REGISTRY_TTL_SECONDS=...`

## CLI Additions

Proposed commands:

```bash
codex-notify launch -- codex
codex-notify sessions list
codex-notify sessions prune
codex-notify probe frontmost
codex-notify probe launch-context
```

Purpose:

- `launch`: start Codex with session capture
- `sessions list`: inspect stored session records for debugging
- `sessions prune`: remove stale records
- `probe frontmost`: show what the current adapter sees as frontmost
- `probe launch-context`: show what would be captured from the current shell/app

## Matching Rules

Suggested default behavior:

- `session` match available: suppress
- `session` match unavailable but app supports `app` fallback: suppress only if user allows fallback
- no supported adapter: do not suppress

This is important because false suppression is worse than an extra popup.

## Security and Privacy

- keep registry local to the user account
- do not store terminal contents
- do not store full prompt history
- prefer opaque ids, `tty`, bundle id, and cwd only
- prune stale session records aggressively

## Rollout Plan

### Phase 1

- add launcher and registry core
- add `Terminal.app` adapter
- add `Ghostty` adapter
- keep everything else on app-level fallback

### Phase 2

- add VS Code companion extension
- support strict VS Code integrated terminal matching

### Phase 3

- add heuristic generic adapter behind opt-in flag
- add Zed support only if stable focused-terminal metadata becomes available

### Phase 4

- simplify if Codex upstream exposes stable origin-session metadata in hook payload

## Testing Strategy

### Unit tests

- registry serialization
- pid/session resolution
- adapter match logic
- fallback ordering

### Manual tests

- two Ghostty windows, two Codex sessions
- two Terminal tabs, two Codex sessions
- Ghostty frontmost while VS Code session completes
- VS Code frontmost while Ghostty session completes
- unsupported app frontmost

### Failure-mode tests

- stale registry entry
- missing adapter
- adapter timeout
- app automation permission denied

## Open Questions

- How should hook-side process lineage resolve the source Codex pid robustly on macOS?
- Should launcher mode replace direct `codex` usage, or remain opt-in?
- Should app-level fallback be enabled by default once session mode exists?
- How much helper state is acceptable in `~/Library/Caches/codex-notify/`?

## Recommendation

Do not try to make session-scoped suppression universal in one pass.

Implement an adapter-based core with strict support for:

- Terminal.app
- Ghostty

Then add:

- VS Code via companion extension

Keep:

- Zed and unknown apps on fallback behavior

This gives a path to broad coverage without pretending unsupported apps can be matched accurately.
