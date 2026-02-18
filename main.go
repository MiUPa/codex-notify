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
)

var rootNotifyLineRE = regexp.MustCompile(`^notify\s*=`)

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
  %s uninstall [--restore-config] [--config path]

Commands:
  init       Add notify hook to Codex config with timestamped backup.
  doctor     Validate runtime requirements and config wiring.
  test       Send a local test notification.
  hook       Receive Codex notify payload and raise macOS notification.
  uninstall  Restore config from latest backup created by init.
`, appName, appName, appName, appName, appName, appName)
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
	return sendNotification("Codex Notify", message)
}

func runHook(args []string) error {
	payloadRaw, err := resolveHookPayload(args)
	if err != nil {
		return err
	}

	title := "Codex"
	message := "イベントを受信しました。"

	if strings.TrimSpace(payloadRaw) != "" {
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
			return fmt.Errorf("parse payload json: %w", err)
		}

		title, message = renderPayloadMessage(payload)
	}

	return sendNotification(title, message)
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
		if isRootNotifyLine(trimmed) && strings.Contains(trimmed, "codex-notify") {
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
		if isRootNotifyLine(trimmed) && strings.Contains(trimmed, "codex-notify") {
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

func renderPayloadMessage(payload map[string]any) (string, string) {
	event := getString(payload, "event")
	switch event {
	case "agent-turn-complete":
		return "Codex: Turn Complete", "入力待ちです。"
	case "approval-requested":
		return "Codex: Approval Requested", "承認待ちです。"
	case "agent-error":
		return "Codex: Error", "エラーイベントを受信しました。"
	default:
		if event == "" {
			return "Codex", "通知イベントを受信しました。"
		}
		return "Codex", fmt.Sprintf("イベント: %s", event)
	}
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
	return s
}

func sendNotification(title, message string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("unsupported OS: %s (macOS only)", runtime.GOOS)
	}

	if path, ok := lookupCmd("terminal-notifier"); ok {
		cmd := exec.Command(path,
			"-title", title,
			"-message", message,
			"-group", "codex-notify",
		)
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
