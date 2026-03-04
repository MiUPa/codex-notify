package main

import "testing"

func TestPopupTimeoutSeconds(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "")

		if got := popupTimeoutSeconds(); got != defaultPopupTimeoutSeconds {
			t.Fatalf("popupTimeoutSeconds() = %d, want %d", got, defaultPopupTimeoutSeconds)
		}
	})

	t.Run("popup timeout env", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "12")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "")

		if got := popupTimeoutSeconds(); got != 12 {
			t.Fatalf("popupTimeoutSeconds() = %d, want 12", got)
		}
	})

	t.Run("legacy approval timeout fallback", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "18")

		if got := popupTimeoutSeconds(); got != 18 {
			t.Fatalf("popupTimeoutSeconds() = %d, want 18", got)
		}
	})

	t.Run("clamps minimum", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "1")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "")

		if got := popupTimeoutSeconds(); got != minPopupTimeoutSeconds {
			t.Fatalf("popupTimeoutSeconds() = %d, want %d", got, minPopupTimeoutSeconds)
		}
	})

	t.Run("falls back after invalid primary value", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "abc")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "21")

		if got := popupTimeoutSeconds(); got != 21 {
			t.Fatalf("popupTimeoutSeconds() = %d, want 21", got)
		}
	})
}

func TestApprovalActionTimeoutSeconds(t *testing.T) {
	t.Run("approval env overrides popup env", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "12")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "30")

		if got := approvalActionTimeoutSeconds(); got != 30 {
			t.Fatalf("approvalActionTimeoutSeconds() = %d, want 30", got)
		}
	})

	t.Run("popup env is shared fallback", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "16")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "")

		if got := approvalActionTimeoutSeconds(); got != 16 {
			t.Fatalf("approvalActionTimeoutSeconds() = %d, want 16", got)
		}
	})

	t.Run("clamps maximum", func(t *testing.T) {
		t.Setenv("CODEX_NOTIFY_POPUP_TIMEOUT_SECONDS", "")
		t.Setenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS", "999")

		if got := approvalActionTimeoutSeconds(); got != maxPopupTimeoutSeconds {
			t.Fatalf("approvalActionTimeoutSeconds() = %d, want %d", got, maxPopupTimeoutSeconds)
		}
	})
}
