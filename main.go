package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	appName              = "codex-notify"
	defaultNotifyLine    = `notify = ["codex-notify", "hook"]`
	defaultTerminalID    = "com.mitchellh.ghostty"
	defaultApproveSeq    = "y,enter"
	defaultRejectSeq     = "n,enter"
	approvalUIPopup      = "popup"
	approvalUISingle     = "single"
	approvalUIMulti      = "multi"
	notificationUIPopup  = "popup"
	notificationUISystem = "system"

	defaultApprovalPromptTimeoutSeconds = 45
	helperSourceFilename                = "approval_action_notifier.swift"
	helperBinaryName                    = "approval_action_notifier"
	helperHashName                      = "approval_action_notifier.sha256"
)

var (
	rootNotifyLineRE  = regexp.MustCompile(`^notify\s*=`)
	codexHookArrayRE  = regexp.MustCompile(`\[\s*"(?:[^"]*/)?codex-notify"\s*,\s*"hook"\s*\]`)
	errDialogCanceled = errors.New("dialog canceled")
)

//go:embed internal/swift/approval_action_notifier.swift
var approvalActionNotifierSource string

type notificationRequest struct {
	Title             string
	Message           string
	Group             string
	ExecuteOnClick    string
	ActivateBundleID  string
	PopupPrimaryLabel string
}

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = runInit(os.Args[2:])
	case "doctor":
		err = runDoctor(os.Args[2:])
	case "test":
		err = runTest(os.Args[2:])
	case "hook":
		err = runHook(os.Args[2:])
	case "action":
		err = runAction(os.Args[2:])
	case "uninstall":
		err = runUninstall(os.Args[2:])
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return
	default:
		err = fmt.Errorf("unknown command: %s", os.Args[1])
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `%s: macOS desktop notifications for Codex CLI

Usage:
  %s init [--replace] [--config path]
  %s doctor [--config path]
  %s test [message]
  %s hook [json-payload]
  %s action <open|approve|reject|choose|submit> [--thread-id id] [--text value]
  %s uninstall [--restore-config] [--config path]

Commands:
  init       Add notify hook to Codex config with timestamped backup.
  doctor     Validate runtime requirements and config wiring.
  test       Send a local test notification.
  hook       Receive Codex notify payload and raise macOS notification.
  action     Execute click action (open terminal / choose / submit text / send approve or reject keys).
  uninstall  Restore config from latest backup created by init.
`, appName, appName, appName, appName, appName, appName, appName)
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	replace := fs.Bool("replace", false, "replace existing notify setting")
	config := fs.String("config", "", "path to Codex config.toml")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfgPath, err := resolveConfigPath(*config)
	if err != nil {
		return err
	}

	existing, err := readFileMaybe(cfgPath)
	if err != nil {
		return err
	}

	if len(existing) == 0 {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}

		content := defaultNotifyLine + "\n"
		if err := writeFileAtomic(cfgPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		fmt.Printf("created %s and configured notify hook\n", cfgPath)
		return nil
	}

	hasCodexNotify, err := configHasCodexNotify(existing)
	if err != nil {
		return err
	}
	if hasCodexNotify {
		fmt.Printf("notify hook already configured in %s\n", cfgPath)
		return nil
	}

	notifyLineIdx := findNotifyLineIndex(existing)
	if notifyLineIdx >= 0 && !*replace {
		return errors.New("existing notify config found; rerun with --replace to update it")
	}

	backupPath, err := createBackup(cfgPath, existing)
	if err != nil {
		return err
	}

	updated := setNotifyLine(existing, notifyLineIdx, defaultNotifyLine)
	if err := writeFileAtomic(cfgPath, updated, 0o644); err != nil {
		return fmt.Errorf("update config: %w", err)
	}

	fmt.Printf("updated %s\n", cfgPath)
	fmt.Printf("backup created: %s\n", backupPath)
	return nil
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	config := fs.String("config", "", "path to Codex config.toml")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfgPath, err := resolveConfigPath(*config)
	if err != nil {
		return err
	}

	problems := 0

	fmt.Println("codex-notify doctor")
	fmt.Println("-------------------")

	if runtime.GOOS != "darwin" {
		fmt.Printf("[FAIL] OS: expected darwin, got %s\n", runtime.GOOS)
		problems++
	} else {
		fmt.Println("[ OK ] OS: darwin")
	}

	terminalNotifierPath, terminalNotifierOK := lookupCmd("terminal-notifier")
	if terminalNotifierOK {
		fmt.Printf("[ OK ] terminal-notifier: %s\n", terminalNotifierPath)
	} else {
		fmt.Println("[WARN] terminal-notifier: not found (will use osascript fallback)")
	}

	osascriptPath, osascriptOK := lookupCmd("osascript")
	if osascriptOK {
		fmt.Printf("[ OK ] osascript: %s\n", osascriptPath)
	} else {
		fmt.Println("[FAIL] osascript: not found")
		problems++
	}

	if notificationUIStyle() == notificationUIPopup {
		swiftcPath, swiftcOK := lookupCmd("swiftc")
		if swiftcOK {
			fmt.Printf("[ OK ] swiftc: %s\n", swiftcPath)
		} else {
			fmt.Println("[WARN] swiftc: not found (popup UI will fall back to system notifications)")
		}
	}

	cfg, err := readFileMaybe(cfgPath)
	if err != nil {
		return err
	}
	if len(cfg) == 0 {
		fmt.Printf("[WARN] config: not found at %s\n", cfgPath)
		problems++
	} else {
		ok, err := configHasCodexNotify(cfg)
		if err != nil {
			return err
		}
		if ok {
			fmt.Printf("[ OK ] config: notify hook is configured (%s)\n", cfgPath)
		} else {
			fmt.Printf("[WARN] config: notify hook not configured (%s)\n", cfgPath)
			problems++
		}
	}

	if problems > 0 {
		return fmt.Errorf("doctor found %d issue(s)", problems)
	}

	fmt.Println("all checks passed")
	return nil
}

func runTest(args []string) error {
	message := "Codex通知テスト"
	if len(args) > 0 {
		message = strings.Join(args, " ")
	}
	return sendNotification(notificationRequest{
		Title:             "Codex Notify",
		Message:           message,
		Group:             "codex-notify-test",
		ExecuteOnClick:    buildActionCommand("open", ""),
		PopupPrimaryLabel: "Open",
	})
}

func runHook(args []string) error {
	payloadRaw, err := resolveHookPayload(args)
	if err != nil {
		return err
	}

	payload := map[string]any{}
	if strings.TrimSpace(payloadRaw) != "" {
		if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
			return fmt.Errorf("parse payload json: %w", err)
		}
	}

	if shouldUseNativeApprovalNotification(payload) {
		if err := sendNativeApprovalNotification(payload); err == nil {
			return nil
		}
	}

	requests, err := buildHookNotifications(payload)
	if err != nil {
		return err
	}

	for _, req := range requests {
		if err := sendNotification(req); err != nil {
			return err
		}
	}
	return nil
}

func runAction(args []string) error {
	if len(args) == 0 {
		return errors.New("action requires one of: open, approve, reject, choose, submit")
	}

	action := strings.ToLower(strings.TrimSpace(args[0]))
	fs := flag.NewFlagSet("action", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	threadID := fs.String("thread-id", "", "thread id")
	text := fs.String("text", "", "text payload for submit action")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	bundleID := terminalBundleID()
	switch action {
	case "open":
		return activateApplication(bundleID)
	case "choose":
		return runChooseAction(bundleID, *threadID)
	case "approve":
		return sendActionKeys(bundleID, approveKeySequence(), *threadID)
	case "reject":
		return sendActionKeys(bundleID, rejectKeySequence(), *threadID)
	case "submit":
		if strings.TrimSpace(*text) == "" {
			return errors.New("submit action requires --text")
		}
		return sendActionKeys(bundleID, []string{*text, "enter"}, *threadID)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func runUninstall(args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	restore := fs.Bool("restore-config", true, "restore latest config backup")
	config := fs.String("config", "", "path to Codex config.toml")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfgPath, err := resolveConfigPath(*config)
	if err != nil {
		return err
	}

	current, err := readFileMaybe(cfgPath)
	if err != nil {
		return err
	}
	if len(current) == 0 {
		fmt.Printf("config not found: %s\n", cfgPath)
		return nil
	}

	if *restore {
		latest, err := findLatestBackup(cfgPath)
		if err != nil {
			return err
		}
		backupContent, err := os.ReadFile(latest)
		if err != nil {
			return fmt.Errorf("read backup: %w", err)
		}
		if err := writeFileAtomic(cfgPath, backupContent, 0o644); err != nil {
			return fmt.Errorf("restore config: %w", err)
		}
		fmt.Printf("restored %s from %s\n", cfgPath, latest)
		return nil
	}

	updated, removed := removeCodexNotifyLine(current)
	if !removed {
		fmt.Println("no codex-notify line found; nothing changed")
		return nil
	}

	backupPath, err := createBackup(cfgPath, current)
	if err != nil {
		return err
	}

	if err := writeFileAtomic(cfgPath, updated, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Printf("removed codex-notify line from %s\n", cfgPath)
	fmt.Printf("backup created: %s\n", backupPath)
	return nil
}

func resolveConfigPath(configFlag string) (string, error) {
	if configFlag != "" {
		return configFlag, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

func readFileMaybe(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		return b, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return nil, fmt.Errorf("read %s: %w", path, err)
}

func configHasCodexNotify(content []byte) (bool, error) {
	lines := splitLines(content)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if isCodexNotifyHookLine(trimmed) {
			return true, nil
		}
	}
	return false, nil
}

func findNotifyLineIndex(content []byte) int {
	lines := splitLines(content)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if isRootNotifyLine(trimmed) {
			return i
		}
	}
	return -1
}

func setNotifyLine(content []byte, idx int, notifyLine string) []byte {
	lines := splitLines(content)
	if idx >= 0 {
		lines[idx] = notifyLine
	} else {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, notifyLine)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func removeCodexNotifyLine(content []byte) ([]byte, bool) {
	lines := splitLines(content)
	out := make([]string, 0, len(lines))
	removed := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isCodexNotifyHookLine(trimmed) {
			removed = true
			continue
		}
		out = append(out, line)
	}

	joined := strings.Join(out, "\n")
	if strings.TrimSpace(joined) == "" {
		return []byte{}, removed
	}
	return []byte(joined + "\n"), removed
}

func splitLines(content []byte) []string {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	scanner := bufio.NewScanner(bytes.NewReader(normalized))
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func createBackup(configPath string, content []byte) (string, error) {
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano())
	backupPath := fmt.Sprintf("%s.bak.%s", configPath, timestamp)
	if err := writeFileAtomic(backupPath, content, 0o644); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}
	return backupPath, nil
}

func findLatestBackup(configPath string) (string, error) {
	pattern := regexp.QuoteMeta(configPath) + `\.bak\.\d+$`
	re := regexp.MustCompile(pattern)

	dir := filepath.Dir(configPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read config dir: %w", err)
	}

	backups := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if re.MatchString(path) {
			backups = append(backups, path)
		}
	}

	if len(backups) == 0 {
		return "", errors.New("no backup found; cannot restore")
	}

	sort.Strings(backups)
	return backups[len(backups)-1], nil
}

func writeFileAtomic(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = os.Remove(tmpPath)
	}

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func resolveHookPayload(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	stdinInfo, err := os.Stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("read stdin stat: %w", err)
	}
	if (stdinInfo.Mode() & os.ModeCharDevice) != 0 {
		return "", nil
	}

	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

func buildHookNotifications(payload map[string]any) ([]notificationRequest, error) {
	eventName := payloadEventName(payload)
	threadID := payloadThreadID(payload)
	title, message := renderPayloadMessage(payload)

	base := notificationRequest{
		Title:          title,
		Message:        message,
		Group:          notificationGroup(eventName, threadID),
		ExecuteOnClick: buildActionCommand("open", threadID),
	}

	requests := []notificationRequest{base}
	if eventName == "approval-requested" && approvalActionsEnabled() {
		if approvalUIStyle() == approvalUIMulti {
			requests = append(requests,
				notificationRequest{
					Title:             "Codex: Approve",
					Message:           "クリックで承認入力を送信",
					Group:             notificationGroup("approve", threadID),
					ExecuteOnClick:    buildActionCommand("approve", threadID),
					PopupPrimaryLabel: "Approve",
				},
				notificationRequest{
					Title:             "Codex: Reject",
					Message:           "クリックで拒否入力を送信",
					Group:             notificationGroup("reject", threadID),
					ExecuteOnClick:    buildActionCommand("reject", threadID),
					PopupPrimaryLabel: "Reject",
				},
			)
		} else {
			requests[0].ExecuteOnClick = buildActionCommand("choose", threadID)
		}
	}

	return requests, nil
}

func renderPayloadMessage(payload map[string]any) (string, string) {
	event := payloadEventName(payload)
	preview := payloadPreviewMessage(payload)

	switch event {
	case "agent-turn-complete":
		if preview == "" {
			preview = "入力待ちです。"
		}
		return "Codex: Turn Complete", preview
	case "approval-requested":
		if preview == "" {
			preview = "承認待ちです。"
		}
		return "Codex: Approval Requested", preview
	case "agent-error":
		if preview == "" {
			preview = "エラーイベントを受信しました。"
		}
		return "Codex: Error", preview
	default:
		if event == "" {
			if preview == "" {
				preview = "通知イベントを受信しました。"
			}
			return "Codex", preview
		}
		if preview != "" {
			return "Codex", fmt.Sprintf("%s: %s", event, preview)
		}
		return "Codex", fmt.Sprintf("イベント: %s", event)
	}
}

func payloadEventName(payload map[string]any) string {
	return getStringAny(payload, "event", "type")
}

func payloadThreadID(payload map[string]any) string {
	return getStringAny(payload, "thread-id", "thread_id", "threadId")
}

func payloadPreviewMessage(payload map[string]any) string {
	msg := getStringAny(
		payload,
		"last-assistant-message",
		"last_assistant_message",
		"message",
		"text",
	)
	if msg == "" {
		msgs := getStringSliceAny(payload, "input-messages", "input_messages")
		if len(msgs) > 0 {
			msg = strings.Join(msgs, " ")
		}
	}
	msg = strings.Join(strings.Fields(msg), " ")
	if len(msg) > 180 {
		msg = msg[:177] + "..."
	}
	return msg
}

func getString(payload map[string]any, key string) string {
	v, ok := payload[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func getStringAny(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := getString(payload, key); s != "" {
			return s
		}
	}
	return ""
}

func getStringSliceAny(payload map[string]any, keys ...string) []string {
	for _, key := range keys {
		v, ok := payload[key]
		if !ok || v == nil {
			continue
		}

		switch typed := v.(type) {
		case []string:
			out := []string{}
			for _, item := range typed {
				item = strings.TrimSpace(item)
				if item != "" {
					out = append(out, item)
				}
			}
			if len(out) > 0 {
				return out
			}
		case []any:
			out := []string{}
			for _, item := range typed {
				itemStr := strings.TrimSpace(fmt.Sprintf("%v", item))
				if itemStr != "" {
					out = append(out, itemStr)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func notificationGroup(kind, threadID string) string {
	kind = sanitizeID(kind)
	if kind == "" {
		kind = "event"
	}
	if threadID == "" {
		return "codex-notify-" + kind
	}
	return fmt.Sprintf("codex-notify-%s-%s", kind, sanitizeID(threadID))
}

func sanitizeID(v string) string {
	if v == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func buildActionCommand(action, threadID string) string {
	executable := appName
	if path, err := os.Executable(); err == nil && strings.TrimSpace(path) != "" {
		executable = path
	}

	parts := []string{
		shellQuote(executable),
		"action",
		shellQuote(action),
	}
	if threadID != "" {
		parts = append(parts, "--thread-id", shellQuote(threadID))
	}
	return strings.Join(parts, " ")
}

func buildSubmitActionCommand(text, threadID string) string {
	executable := appName
	if path, err := os.Executable(); err == nil && strings.TrimSpace(path) != "" {
		executable = path
	}

	parts := []string{
		shellQuote(executable),
		"action",
		"submit",
		"--text",
		shellQuote(text),
	}
	if threadID != "" {
		parts = append(parts, "--thread-id", shellQuote(threadID))
	}
	return strings.Join(parts, " ")
}

func shellQuote(v string) string {
	if v == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(v, "'", `'"'"'`) + "'"
}

func terminalBundleID() string {
	v := strings.TrimSpace(os.Getenv("CODEX_NOTIFY_TERMINAL_BUNDLE_ID"))
	if v != "" {
		return v
	}
	return defaultTerminalID
}

func approveKeySequence() []string {
	return keySequenceFromEnv("CODEX_NOTIFY_APPROVE_KEYS", defaultApproveSeq)
}

func rejectKeySequence() []string {
	return keySequenceFromEnv("CODEX_NOTIFY_REJECT_KEYS", defaultRejectSeq)
}

func keySequenceFromEnv(key, fallback string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		raw = fallback
	}
	parts := strings.Split(raw, ",")
	out := []string{}
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token != "" {
			out = append(out, token)
		}
	}
	if len(out) == 0 && fallback != "" {
		out = append(out, strings.Split(fallback, ",")...)
	}
	return out
}

func approvalActionsEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("CODEX_NOTIFY_ENABLE_APPROVAL_ACTIONS")))
	if v == "" {
		return true
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func approvalUIStyle() string {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("CODEX_NOTIFY_APPROVAL_UI")))
	switch v {
	case "", approvalUIPopup, approvalUISingle:
		return approvalUIPopup
	case approvalUIMulti:
		return approvalUIMulti
	default:
		return approvalUIPopup
	}
}

func notificationUIStyle() string {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("CODEX_NOTIFY_NOTIFICATION_UI")))
	switch v {
	case "", notificationUIPopup:
		return notificationUIPopup
	case notificationUISystem:
		return notificationUISystem
	default:
		return notificationUIPopup
	}
}

func shouldUseNativeApprovalNotification(payload map[string]any) bool {
	if notificationUIStyle() == notificationUISystem {
		return false
	}
	if payloadEventName(payload) != "approval-requested" {
		return false
	}
	if !approvalActionsEnabled() {
		return false
	}
	if approvalUIStyle() == approvalUIMulti {
		return false
	}

	v := strings.TrimSpace(strings.ToLower(os.Getenv("CODEX_NOTIFY_ENABLE_POPUP_APPROVAL_ACTIONS")))
	if v == "" {
		v = strings.TrimSpace(strings.ToLower(os.Getenv("CODEX_NOTIFY_ENABLE_NATIVE_APPROVAL_ACTIONS")))
	}
	if v == "" {
		return true
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func sendNativeApprovalNotification(payload map[string]any) error {
	helperPath, err := ensureApprovalActionHelper()
	if err != nil {
		return err
	}

	threadID := payloadThreadID(payload)
	title, message := renderPayloadMessage(payload)
	choices := approvalChoicesFromPayload(payload, threadID)
	if len(choices) == 0 {
		choices = defaultApprovalChoices(threadID)
	}

	args := []string{
		"--title", title,
		"--message", message,
		"--identifier", notificationGroup("approval-native", threadID),
		"--timeout-seconds", strconv.Itoa(approvalActionTimeoutSeconds()),
	}
	for _, choice := range choices {
		args = append(args, "--choice-label", choice.Label)
		args = append(args, "--choice-cmd", choice.Command)
	}

	cmd := exec.Command(helperPath, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start native approval notifier: %w", err)
	}
	return nil
}

func sendNativePopupNotification(req notificationRequest, title, message, group string) error {
	helperPath, err := ensureApprovalActionHelper()
	if err != nil {
		return err
	}

	choices := popupChoicesForRequest(req)
	args := []string{
		"--title", title,
		"--message", message,
		"--identifier", group,
		"--timeout-seconds", strconv.Itoa(approvalActionTimeoutSeconds()),
	}
	for _, choice := range choices {
		args = append(args, "--choice-label", choice.Label)
		args = append(args, "--choice-cmd", choice.Command)
	}

	cmd := exec.Command(helperPath, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start native popup notifier: %w", err)
	}
	return nil
}

func popupChoicesForRequest(req notificationRequest) []approvalChoice {
	command := strings.TrimSpace(req.ExecuteOnClick)
	label := strings.TrimSpace(req.PopupPrimaryLabel)
	if label == "" {
		label = inferPopupLabelFromCommand(command)
	}
	if label == "" {
		if command == "" {
			label = "Close"
		} else {
			label = "Open"
		}
	}

	return []approvalChoice{
		{Label: label, Command: command},
	}
}

func inferPopupLabelFromCommand(command string) string {
	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd == "" {
		return ""
	}

	switch {
	case strings.Contains(cmd, " action approve"):
		return "Approve"
	case strings.Contains(cmd, " action reject"):
		return "Reject"
	case strings.Contains(cmd, " action choose"):
		return "Choose"
	case strings.Contains(cmd, " action submit"):
		return "Submit"
	case strings.Contains(cmd, " action open"):
		return "Open"
	default:
		return "Open"
	}
}

func approvalActionTimeoutSeconds() int {
	raw := strings.TrimSpace(os.Getenv("CODEX_NOTIFY_APPROVAL_TIMEOUT_SECONDS"))
	if raw == "" {
		return defaultApprovalPromptTimeoutSeconds
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return defaultApprovalPromptTimeoutSeconds
	}
	if parsed < 5 {
		return 5
	}
	if parsed > 300 {
		return 300
	}
	return parsed
}

func ensureApprovalActionHelper() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir: %w", err)
	}

	helperDir := filepath.Join(cacheDir, appName)
	if err := os.MkdirAll(helperDir, 0o755); err != nil {
		return "", fmt.Errorf("create helper dir: %w", err)
	}

	sourcePath := filepath.Join(helperDir, helperSourceFilename)
	binaryPath := filepath.Join(helperDir, helperBinaryName)
	hashPath := filepath.Join(helperDir, helperHashName)

	expectedHash := helperSourceHash(approvalActionNotifierSource)
	currentHash, _ := os.ReadFile(hashPath)
	if strings.TrimSpace(string(currentHash)) == expectedHash {
		if info, err := os.Stat(binaryPath); err == nil && info.Mode().IsRegular() {
			return binaryPath, nil
		}
	}

	swiftcPath, ok := lookupCmd("swiftc")
	if !ok {
		return "", errors.New("swiftc not found")
	}

	if err := writeFileAtomic(sourcePath, []byte(approvalActionNotifierSource), 0o644); err != nil {
		return "", fmt.Errorf("write helper source: %w", err)
	}

	tmpBinaryPath := binaryPath + ".tmp"
	_ = os.Remove(tmpBinaryPath)

	compileCmd := exec.Command(swiftcPath, "-O", "-suppress-warnings", sourcePath, "-o", tmpBinaryPath)
	if out, err := compileCmd.CombinedOutput(); err != nil {
		_ = os.Remove(tmpBinaryPath)
		return "", fmt.Errorf("compile helper failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	if err := os.Chmod(tmpBinaryPath, 0o755); err != nil {
		_ = os.Remove(tmpBinaryPath)
		return "", fmt.Errorf("chmod helper: %w", err)
	}
	if err := os.Rename(tmpBinaryPath, binaryPath); err != nil {
		_ = os.Remove(tmpBinaryPath)
		return "", fmt.Errorf("install helper: %w", err)
	}
	if err := writeFileAtomic(hashPath, []byte(expectedHash+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("write helper hash: %w", err)
	}

	return binaryPath, nil
}

func helperSourceHash(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

type approvalChoice struct {
	Label   string
	Command string
}

func defaultApprovalChoices(threadID string) []approvalChoice {
	return []approvalChoice{
		{Label: "Open", Command: buildActionCommand("open", threadID)},
		{Label: "Approve", Command: buildActionCommand("approve", threadID)},
		{Label: "Reject", Command: buildActionCommand("reject", threadID)},
	}
}

func approvalChoicesFromPayload(payload map[string]any, threadID string) []approvalChoice {
	options := payloadApprovalOptions(payload)
	if len(options) == 0 {
		return nil
	}

	choices := make([]approvalChoice, 0, len(options))
	for i, option := range options {
		label := strings.TrimSpace(option)
		if label == "" {
			continue
		}

		action := actionForApprovalOption(label, i, len(options))
		command := buildSubmitActionCommand(label, threadID)
		if action != "" {
			command = buildActionCommand(action, threadID)
		}
		choices = append(choices, approvalChoice{
			Label:   label,
			Command: command,
		})
	}
	return choices
}

func payloadApprovalOptions(payload map[string]any) []string {
	return getStringSliceAny(
		payload,
		"approval-options",
		"approval_options",
		"options",
		"choices",
		"actions",
	)
}

func actionForApprovalOption(label string, idx, total int) string {
	norm := strings.ToLower(strings.TrimSpace(label))
	norm = strings.ReplaceAll(norm, " ", "")
	norm = strings.ReplaceAll(norm, "-", "")
	norm = strings.ReplaceAll(norm, "_", "")

	switch norm {
	case "open", "show", "focus":
		return "open"
	case "approve", "approved", "allow", "yes", "y", "ok":
		return "approve"
	case "reject", "denied", "deny", "no", "n", "cancel":
		return "reject"
	}

	// Common approval UX is binary yes/no; map by position if labels are unknown.
	if total == 2 {
		if idx == 0 {
			return "approve"
		}
		return "reject"
	}

	return ""
}

func activateApplication(bundleID string) error {
	path, ok := lookupCmd("osascript")
	if !ok {
		return errors.New("osascript not found")
	}

	script := fmt.Sprintf(`tell application id "%s" to activate`, escapeAppleScript(bundleID))
	cmd := exec.Command(path, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("activate app failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func sendActionKeys(bundleID string, seq []string, threadID string) error {
	if err := activateApplication(bundleID); err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond)

	if len(seq) == 0 {
		return nil
	}
	return sendKeySequence(seq, threadID)
}

func runChooseAction(bundleID, threadID string) error {
	choice, err := chooseApprovalAction(threadID)
	if err != nil {
		if errors.Is(err, errDialogCanceled) {
			return nil
		}
		return err
	}

	switch choice {
	case "open":
		return activateApplication(bundleID)
	case "approve":
		return sendActionKeys(bundleID, approveKeySequence(), threadID)
	case "reject":
		return sendActionKeys(bundleID, rejectKeySequence(), threadID)
	default:
		return fmt.Errorf("unknown chosen action: %s", choice)
	}
}

func chooseApprovalAction(threadID string) (string, error) {
	path, ok := lookupCmd("osascript")
	if !ok {
		return "", errors.New("osascript not found")
	}

	prompt := "承認待ちです。実行する操作を選択してください。"
	if threadID != "" {
		prompt = fmt.Sprintf("thread: %s\\n承認待ちです。実行する操作を選択してください。", threadID)
	}

	script := fmt.Sprintf(`try
	set dialogResult to display dialog "%s" with title "Codex Notify" buttons {"Open", "Approve", "Reject"} default button "Open" giving up after %d
	if gave up of dialogResult then
		return "none"
	end if
	set selectedButton to button returned of dialogResult
	if selectedButton is "Open" then
		return "open"
	else if selectedButton is "Approve" then
		return "approve"
	else
		return "reject"
	end if
on error number -128
	return "none"
end try`, escapeAppleScript(prompt), approvalActionTimeoutSeconds())

	cmd := exec.Command(path, "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("choose action failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	choice := strings.ToLower(strings.TrimSpace(string(out)))
	if choice == "" || choice == "none" {
		return "", errDialogCanceled
	}
	switch choice {
	case "open", "approve", "reject":
		return choice, nil
	default:
		return "", fmt.Errorf("unknown choice from dialog: %s", choice)
	}
}

func sendKeySequence(seq []string, threadID string) error {
	path, ok := lookupCmd("osascript")
	if !ok {
		return errors.New("osascript not found")
	}

	for _, token := range seq {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		var script string
		if code, special := keyCodeForToken(token); special {
			script = fmt.Sprintf(`tell application "System Events" to key code %d`, code)
		} else {
			script = fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escapeAppleScript(token))
		}

		cmd := exec.Command(path, "-e", script)
		if out, err := cmd.CombinedOutput(); err != nil {
			if threadID != "" {
				return fmt.Errorf("send key for thread %s: %w (%s)", threadID, err, strings.TrimSpace(string(out)))
			}
			return fmt.Errorf("send key: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		time.Sleep(80 * time.Millisecond)
	}

	return nil
}

func keyCodeForToken(token string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "enter", "return":
		return 36, true
	case "tab":
		return 48, true
	case "esc", "escape":
		return 53, true
	case "space":
		return 49, true
	case "up":
		return 126, true
	case "down":
		return 125, true
	case "left":
		return 123, true
	case "right":
		return 124, true
	default:
		return 0, false
	}
}

func sendNotification(req notificationRequest) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("unsupported OS: %s (macOS only)", runtime.GOOS)
	}

	title := req.Title
	if title == "" {
		title = "Codex"
	}
	message := req.Message
	if message == "" {
		message = "通知イベントを受信しました。"
	}
	group := req.Group
	if group == "" {
		group = "codex-notify"
	}

	if notificationUIStyle() == notificationUIPopup {
		if err := sendNativePopupNotification(req, title, message, group); err == nil {
			return nil
		}
	}

	if path, ok := lookupCmd("terminal-notifier"); ok {
		args := []string{
			"-title", title,
			"-message", message,
			"-group", group,
		}
		if req.ExecuteOnClick != "" {
			args = append(args, "-execute", req.ExecuteOnClick)
		}
		if req.ActivateBundleID != "" {
			args = append(args, "-activate", req.ActivateBundleID)
		}

		cmd := exec.Command(path, args...)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	path, ok := lookupCmd("osascript")
	if !ok {
		return errors.New("no notifier available (terminal-notifier and osascript not found)")
	}

	script := fmt.Sprintf(`display notification "%s" with title "%s"`, escapeAppleScript(message), escapeAppleScript(title))
	cmd := exec.Command(path, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("osascript failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func escapeAppleScript(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
	)
	return replacer.Replace(s)
}

func lookupCmd(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return path, true
}

func isRootNotifyLine(trimmedLine string) bool {
	return rootNotifyLineRE.MatchString(trimmedLine)
}

func isCodexNotifyHookLine(trimmedLine string) bool {
	if !isRootNotifyLine(trimmedLine) {
		return false
	}

	parts := strings.SplitN(trimmedLine, "=", 2)
	if len(parts) != 2 {
		return false
	}

	rhs := strings.TrimSpace(parts[1])
	return codexHookArrayRE.MatchString(rhs)
}
