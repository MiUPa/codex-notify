package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func useTempUserConfigDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	prev := userConfigDir
	userConfigDir = func() (string, error) {
		return dir, nil
	}
	t.Cleanup(func() {
		userConfigDir = prev
	})
	return dir
}

func useTempUserCacheDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	prev := userCacheDir
	userCacheDir = func() (string, error) {
		return dir, nil
	}
	t.Cleanup(func() {
		userCacheDir = prev
	})
	return dir
}

func writePopupSettingsForTest(t *testing.T, configDir string, content string) {
	t.Helper()

	path := filepath.Join(configDir, appName, popupSettingsFilename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func TestPopupTimeoutSeconds(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		useTempUserConfigDir(t)
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "")

		if got := popupTimeoutSeconds(); got != defaultPopupTimeoutSeconds {
			t.Fatalf("popupTimeoutSeconds() = %d, want %d", got, defaultPopupTimeoutSeconds)
		}
	})

	t.Run("popup timeout env", func(t *testing.T) {
		useTempUserConfigDir(t)
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "12")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "")

		if got := popupTimeoutSeconds(); got != 12 {
			t.Fatalf("popupTimeoutSeconds() = %d, want 12", got)
		}
	})

	t.Run("legacy approval timeout fallback", func(t *testing.T) {
		useTempUserConfigDir(t)
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "18")

		if got := popupTimeoutSeconds(); got != 18 {
			t.Fatalf("popupTimeoutSeconds() = %d, want 18", got)
		}
	})

	t.Run("clamps minimum", func(t *testing.T) {
		useTempUserConfigDir(t)
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "1")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "")

		if got := popupTimeoutSeconds(); got != minPopupTimeoutSeconds {
			t.Fatalf("popupTimeoutSeconds() = %d, want %d", got, minPopupTimeoutSeconds)
		}
	})

	t.Run("falls back after invalid primary value", func(t *testing.T) {
		useTempUserConfigDir(t)
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "abc")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "21")

		if got := popupTimeoutSeconds(); got != 21 {
			t.Fatalf("popupTimeoutSeconds() = %d, want 21", got)
		}
	})

	t.Run("saved setting fallback", func(t *testing.T) {
		configDir := useTempUserConfigDir(t)
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "")
		writePopupSettingsForTest(t, configDir, `{"popup_timeout_seconds":27}`)

		if got := popupTimeoutSeconds(); got != 27 {
			t.Fatalf("popupTimeoutSeconds() = %d, want 27", got)
		}
	})

	t.Run("env override beats saved setting", func(t *testing.T) {
		configDir := useTempUserConfigDir(t)
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "12")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "")
		writePopupSettingsForTest(t, configDir, `{"popup_timeout_seconds":27}`)

		if got := popupTimeoutSeconds(); got != 12 {
			t.Fatalf("popupTimeoutSeconds() = %d, want 12", got)
		}
	})

	t.Run("saved setting clamps maximum", func(t *testing.T) {
		configDir := useTempUserConfigDir(t)
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "")
		writePopupSettingsForTest(t, configDir, `{"popup_timeout_seconds":999}`)

		if got := popupTimeoutSeconds(); got != maxPopupTimeoutSeconds {
			t.Fatalf("popupTimeoutSeconds() = %d, want %d", got, maxPopupTimeoutSeconds)
		}
	})
}

func TestApprovalActionTimeoutSeconds(t *testing.T) {
	t.Run("approval env overrides popup env", func(t *testing.T) {
		useTempUserConfigDir(t)
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "12")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "30")

		if got := approvalActionTimeoutSeconds(); got != 30 {
			t.Fatalf("approvalActionTimeoutSeconds() = %d, want 30", got)
		}
	})

	t.Run("popup env is shared fallback", func(t *testing.T) {
		useTempUserConfigDir(t)
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "16")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "")

		if got := approvalActionTimeoutSeconds(); got != 16 {
			t.Fatalf("approvalActionTimeoutSeconds() = %d, want 16", got)
		}
	})

	t.Run("clamps maximum", func(t *testing.T) {
		useTempUserConfigDir(t)
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "999")

		if got := approvalActionTimeoutSeconds(); got != maxPopupTimeoutSeconds {
			t.Fatalf("approvalActionTimeoutSeconds() = %d, want %d", got, maxPopupTimeoutSeconds)
		}
	})

	t.Run("saved setting fallback", func(t *testing.T) {
		configDir := useTempUserConfigDir(t)
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "")
		writePopupSettingsForTest(t, configDir, `{"popup_timeout_seconds":24}`)

		if got := approvalActionTimeoutSeconds(); got != 24 {
			t.Fatalf("approvalActionTimeoutSeconds() = %d, want 24", got)
		}
	})
}

func TestSuppressFrontmostBundleIDs(t *testing.T) {
	t.Run("falls back to default terminal bundle id", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_SUPPRESS_FRONTMOST_BUNDLE_IDS", "")
		t.Setenv("CODEX_NOTIFY_TERMINAL_BUNDLE_ID", "")

		if got := suppressFrontmostBundleIDs(); !reflect.DeepEqual(got, []string{defaultTerminalID}) {
			t.Fatalf("suppressFrontmostBundleIDs() = %#v, want %#v", got, []string{defaultTerminalID})
		}
	})

	t.Run("falls back to configured terminal bundle id", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_SUPPRESS_FRONTMOST_BUNDLE_IDS", "")
		t.Setenv("CODEX_NOTIFY_TERMINAL_BUNDLE_ID", "com.apple.Terminal")

		if got := suppressFrontmostBundleIDs(); !reflect.DeepEqual(got, []string{"com.apple.Terminal"}) {
			t.Fatalf("suppressFrontmostBundleIDs() = %#v, want %#v", got, []string{"com.apple.Terminal"})
		}
	})

	t.Run("parses comma-separated bundle ids and de-duplicates", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_TERMINAL_BUNDLE_ID", "com.apple.Terminal")
		t.Setenv("CODEX_NOTIFY_SUPPRESS_FRONTMOST_BUNDLE_IDS", " com.apple.Terminal, com.mitchellh.ghostty ,, com.microsoft.VSCode, com.apple.Terminal ")

		want := []string{"com.apple.Terminal", "com.mitchellh.ghostty", "com.microsoft.VSCode"}
		if got := suppressFrontmostBundleIDs(); !reflect.DeepEqual(got, want) {
			t.Fatalf("suppressFrontmostBundleIDs() = %#v, want %#v", got, want)
		}
	})
}

func TestDetectLaunchHostKind(t *testing.T) {
	t.Run("ghostty", func(t *testing.T) {
		t.Setenv("TERM_PROGRAM", "ghostty")

		if got := detectLaunchHostKind(); got != hostKindGhostty {
			t.Fatalf("detectLaunchHostKind() = %q, want %q", got, hostKindGhostty)
		}
	})

	t.Run("apple terminal", func(t *testing.T) {
		t.Setenv("TERM_PROGRAM", "Apple_Terminal")

		if got := detectLaunchHostKind(); got != hostKindTerminal {
			t.Fatalf("detectLaunchHostKind() = %q, want %q", got, hostKindTerminal)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		t.Setenv("TERM_PROGRAM", "vscode")

		if got := detectLaunchHostKind(); got != hostKindUnknown {
			t.Fatalf("detectLaunchHostKind() = %q, want empty", got)
		}
	})
}

func TestApplySourceSessionEnv(t *testing.T) {
	source := sourceSession{
		HostKind:   hostKindGhostty,
		BundleID:   defaultTerminalID,
		TTY:        "ttys042",
		TerminalID: "ghostty-terminal-123",
	}

	env := applySourceSessionEnv([]string{"PATH=/usr/bin"}, source)
	got := map[string]string{}
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		got[parts[0]] = parts[1]
	}

	want := map[string]string{
		"PATH":                            "/usr/bin",
		"CODEX_NOTIFY_TERMINAL_BUNDLE_ID": defaultTerminalID,
		envSourceHostKind:                 hostKindGhostty,
		envSourceBundleID:                 defaultTerminalID,
		envSourceTTY:                      "ttys042",
		envSourceTerminalID:               "ghostty-terminal-123",
	}

	for key, value := range want {
		if got[key] != value {
			t.Fatalf("env[%q] = %q, want %q", key, got[key], value)
		}
	}
}

func TestSourceSessionFromEnv(t *testing.T) {
	t.Run("returns false when unset", func(t *testing.T) {
		t.Setenv(envSourceHostKind, "")
		t.Setenv(envSourceBundleID, "")
		t.Setenv(envSourceTTY, "")
		t.Setenv(envSourceTerminalID, "")

		if _, ok := sourceSessionFromEnv(); ok {
			t.Fatal("sourceSessionFromEnv() ok = true, want false")
		}
	})

	t.Run("reads source metadata and normalizes tty", func(t *testing.T) {
		t.Setenv(envSourceHostKind, hostKindTerminal)
		t.Setenv(envSourceBundleID, "")
		t.Setenv(envSourceTTY, "/dev/ttys099")
		t.Setenv(envSourceTerminalID, "")

		got, ok := sourceSessionFromEnv()
		if !ok {
			t.Fatal("sourceSessionFromEnv() ok = false, want true")
		}
		if got.HostKind != hostKindTerminal {
			t.Fatalf("HostKind = %q, want %q", got.HostKind, hostKindTerminal)
		}
		if got.BundleID != terminalAppBundleID {
			t.Fatalf("BundleID = %q, want %q", got.BundleID, terminalAppBundleID)
		}
		if got.TTY != "ttys099" {
			t.Fatalf("TTY = %q, want %q", got.TTY, "ttys099")
		}
	})
}

func TestEffectiveSuppressFrontmostBundleIDs(t *testing.T) {
	t.Run("uses explicit override", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_SUPPRESS_FRONTMOST_BUNDLE_IDS", "com.apple.Terminal")

		got := effectiveSuppressFrontmostBundleIDs(true, nil)
		if len(got) != 0 {
			t.Fatalf("effectiveSuppressFrontmostBundleIDs(true, nil) = %#v, want empty", got)
		}
	})

	t.Run("falls back to configured suppression list", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_SUPPRESS_FRONTMOST_BUNDLE_IDS", "com.apple.Terminal,com.mitchellh.ghostty")

		want := []string{"com.apple.Terminal", "com.mitchellh.ghostty"}
		if got := effectiveSuppressFrontmostBundleIDs(false, nil); !reflect.DeepEqual(got, want) {
			t.Fatalf("effectiveSuppressFrontmostBundleIDs(false, nil) = %#v, want %#v", got, want)
		}
	})
}

func TestApprovalInteractionLockIsScopedToThread(t *testing.T) {
	cacheDir := useTempUserCacheDir(t)

	first, err := approvalInteractionLockPath("thread/a")
	if err != nil {
		t.Fatalf("approvalInteractionLockPath(first): %v", err)
	}
	second, err := approvalInteractionLockPath("thread-b")
	if err != nil {
		t.Fatalf("approvalInteractionLockPath(second): %v", err)
	}
	if first == second {
		t.Fatalf("thread lock paths are equal: %q", first)
	}
	if !strings.HasPrefix(first, filepath.Join(cacheDir, appName)+string(filepath.Separator)) {
		t.Fatalf("lock path %q is outside cache dir %q", first, cacheDir)
	}

	noThread, err := approvalInteractionLockPath("")
	if err != nil {
		t.Fatalf("approvalInteractionLockPath(empty): %v", err)
	}
	if noThread != "" {
		t.Fatalf("approvalInteractionLockPath(empty) = %q, want empty", noThread)
	}

	if err := writeApprovalInteractionLock(first, 60); err != nil {
		t.Fatalf("writeApprovalInteractionLock(): %v", err)
	}
	if !isApprovalInteractionLockActive("thread/a") {
		t.Fatal("same-thread lock is inactive")
	}
	if isApprovalInteractionLockActive("thread-b") {
		t.Fatal("different-thread lock is active")
	}
	if isApprovalInteractionLockActive("") {
		t.Fatal("empty-thread lock is active")
	}
}

func TestFindLatestBackupSupportsLegacyTimestamp(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	now := time.Now()
	names := []string{
		"config.toml.bak.20260321-140409",
		"config.toml.bak.20260321-154248",
	}
	for i, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatalf("WriteFile(%q): %v", path, err)
		}
		stamp := now.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatalf("Chtimes(%q): %v", path, err)
		}
	}

	got, err := findLatestBackup(configPath)
	if err != nil {
		t.Fatalf("findLatestBackup(): %v", err)
	}
	want := filepath.Join(dir, names[1])
	if got != want {
		t.Fatalf("findLatestBackup() = %q, want %q", got, want)
	}
}

func TestSplitLinesHandlesLongLines(t *testing.T) {
	longLine := strings.Repeat("x", 70_000)
	lines := splitLines([]byte(longLine + "\nnext\n"))
	if len(lines) != 2 {
		t.Fatalf("splitLines() returned %d lines, want 2", len(lines))
	}
	if lines[0] != longLine || lines[1] != "next" {
		t.Fatal("splitLines() changed long-line content")
	}
}

func TestIsCodexNotifyHookLine(t *testing.T) {
	t.Run("matches direct codex-notify hook array", func(t *testing.T) {
		line := `notify = ["/Users/Y.Miura/works/codex-notify/bin/codex-notify", "hook"]`
		if !isCodexNotifyHookLine(line) {
			t.Fatalf("isCodexNotifyHookLine(%q) = false, want true", line)
		}
	})

	t.Run("matches env wrapped codex-notify hook array", func(t *testing.T) {
		line := `notify = ["/usr/bin/env", "CODEX_NOTIFY_SUPPRESS_FRONTMOST_BUNDLE_IDS=com.mitchellh.ghostty,com.apple.Terminal", "/Users/Y.Miura/works/codex-notify/bin/codex-notify", "hook"]`
		if !isCodexNotifyHookLine(line) {
			t.Fatalf("isCodexNotifyHookLine(%q) = false, want true", line)
		}
	})

	t.Run("rejects unrelated notify array", func(t *testing.T) {
		line := `notify = ["/usr/bin/env", "FOO=bar", "/tmp/not-codex", "hook"]`
		if isCodexNotifyHookLine(line) {
			t.Fatalf("isCodexNotifyHookLine(%q) = true, want false", line)
		}
	})
}

func TestMultilineCodexNotifyHook(t *testing.T) {
	content := []byte("notify = [\n" +
		"  \"/usr/bin/env\",\n" +
		"  \"CODEX_NOTIFY_SUPPRESS_FRONTMOST_BUNDLE_IDS=com.mitchellh.ghostty\",\n" +
		"  \"/Users/Y.Miura/works/codex-notify/bin/codex-notify\",\n" +
		"  \"hook\",\n" +
		"]\n\n[projects]\nname = \"demo\"\n")

	hasHook, err := configHasCodexNotify(content)
	if err != nil {
		t.Fatalf("configHasCodexNotify(): %v", err)
	}
	if !hasHook {
		t.Fatal("configHasCodexNotify() = false, want true")
	}

	updated := setNotifyLine(content, findNotifyLineIndex(content), defaultNotifyLine)
	if strings.Contains(string(updated), "CODEX_NOTIFY_SUPPRESS_FRONTMOST") {
		t.Fatal("setNotifyLine() left multiline notify content behind")
	}
	if !strings.Contains(string(updated), defaultNotifyLine) {
		t.Fatal("setNotifyLine() did not install default notify line")
	}

	removed, ok := removeCodexNotifyLine(content)
	if !ok {
		t.Fatal("removeCodexNotifyLine() removed = false, want true")
	}
	if strings.Contains(string(removed), "codex-notify") {
		t.Fatal("removeCodexNotifyLine() left hook content behind")
	}
}

func TestPayloadPreviewMessage(t *testing.T) {
	t.Run("prefers assistant message and normalizes whitespace", func(t *testing.T) {
		payload := map[string]any{
			"last-assistant-message": "  first line\n\n second\tline  ",
			"input-messages":         []string{"ignored"},
		}

		if got := payloadPreviewMessage(payload); got != "first line second line" {
			t.Fatalf("payloadPreviewMessage() = %q, want %q", got, "first line second line")
		}
	})

	t.Run("falls back to input messages", func(t *testing.T) {
		payload := map[string]any{
			"input_messages": []any{"  alpha  ", "beta", "", "gamma"},
		}

		if got := payloadPreviewMessage(payload); got != "alpha beta gamma" {
			t.Fatalf("payloadPreviewMessage() = %q, want %q", got, "alpha beta gamma")
		}
	})

	t.Run("strips markdown link targets and code ticks", func(t *testing.T) {
		payload := map[string]any{
			"message": "Updated [main.go:31](/tmp/main.go#L31) and `main_test.go` for readability.",
		}

		if got := payloadPreviewMessage(payload); got != "Updated main.go:31 and main_test.go for readability." {
			t.Fatalf("payloadPreviewMessage() = %q, want %q", got, "Updated main.go:31 and main_test.go for readability.")
		}
	})

	t.Run("truncates by rune count", func(t *testing.T) {
		longMessage := strings.Repeat("あ", payloadPreviewMaxRunes+25)
		payload := map[string]any{
			"message": longMessage,
		}

		got := payloadPreviewMessage(payload)
		if len([]rune(got)) != payloadPreviewMaxRunes {
			t.Fatalf("len([]rune(payloadPreviewMessage())) = %d, want %d", len([]rune(got)), payloadPreviewMaxRunes)
		}
		if !strings.HasSuffix(got, "...") {
			t.Fatalf("payloadPreviewMessage() = %q, want suffix %q", got, "...")
		}
		wantPrefix := strings.Repeat("あ", payloadPreviewMaxRunes-3)
		if !strings.HasPrefix(got, wantPrefix) {
			t.Fatalf("payloadPreviewMessage() prefix = %q, want %q", string([]rune(got)[:8]), string([]rune(wantPrefix)[:8]))
		}
	})
}
