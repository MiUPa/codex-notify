package main

import (
	"bufio"
	"bytes"
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
	"strings"
	"time"
)

const (
	appName           = "codex-notify"
	defaultNotifyLine = `notify = ["codex-notify", "hook"]`
	defaultTerminalID = "com.mitchellh.ghostty"
	defaultApproveSeq = "y,enter"
	defaultRejectSeq  = "n,enter"
)

var (
	rootNotifyLineRE  = regexp.MustCompile(`^notify\s*=`)
	codexHookArrayRE = regexp.MustCompile(`\[\s*"(?:[^"]*/)?codex-notify"\s*,\s*"hook"\s*\]`)
)

type notificationRequest struct {
	Title            string
	Message          string
	Group            string
	ExecuteOnClick   string
	ActivateBundleID string
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
  %s action <open|approve|reject> [--thread-id id]
  %s uninstall [--restore-config] [--config path]

Commands:
  init       Add notify hook to Codex config with timestamped backup.
  doctor     Validate runtime requirements and config wiring.
  test       Send a local test notification.
  hook       Receive Codex notify payload and raise macOS notification.
  action     Execute click action (open terminal / send approve or reject keys).
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
		Title:   "Codex Notify",
		Message: message,
		Group:   "codex-notify-test",
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
		return errors.New("action requires one of: open, approve, reject")
	}

	action := strings.ToLower(strings.TrimSpace(args[0]))
	fs := flag.NewFlagSet("action", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	threadID := fs.String("thread-id", "", "thread id")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	bundleID := terminalBundleID()
	switch action {
	case "open":
		return activateApplication(bundleID)
	case "approve":
		return sendActionKeys(bundleID, approveKeySequence(), *threadID)
	case "reject":
		return sendActionKeys(bundleID, rejectKeySequence(), *threadID)
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
		requests = append(requests,
			notificationRequest{
				Title:          "Codex: Approve",
				Message:        "クリックで承認入力を送信",
				Group:          notificationGroup("approve", threadID),
				ExecuteOnClick: buildActionCommand("approve", threadID),
			},
			notificationRequest{
				Title:          "Codex: Reject",
				Message:        "クリックで拒否入力を送信",
				Group:          notificationGroup("reject", threadID),
				ExecuteOnClick: buildActionCommand("reject", threadID),
			},
		)
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
