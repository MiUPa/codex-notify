package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
