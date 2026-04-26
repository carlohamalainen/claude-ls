package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"
)

var version = "dev"

type rawEntry struct {
	Type       string          `json:"type"`
	Timestamp  string          `json:"timestamp"`
	Cwd        string          `json:"cwd"`
	SessionID  string          `json:"sessionId"`
	IsMeta     bool            `json:"isMeta"`
	Message    json.RawMessage `json:"message"`
	LastPrompt string          `json:"lastPrompt"`
	GitBranch  string          `json:"gitBranch"`
}

type rawMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type session struct {
	file      string
	sessionID string
	cwd       string
	branch    string
	latest    time.Time
	snippet   string
}

func main() {
	var (
		limit    int
		sinceStr string
		cwdSub      string
		excludeSub  string
		showAll     bool
		root        string
		showVersion bool
	)
	flag.IntVar(&limit, "n", 20, "max sessions to show (0 = no limit)")
	flag.StringVar(&sinceStr, "since", "", "only show sessions newer than this duration ago (e.g. 24h, 7d)")
	flag.StringVar(&cwdSub, "cwd", "", "filter to sessions whose cwd contains this substring")
	flag.StringVar(&excludeSub, "exclude", "", "exclude sessions whose cwd contains this substring")
	flag.BoolVar(&showAll, "all", false, "show all sessions and don't truncate the snippet to terminal width")
	flag.StringVar(&root, "root", "", "Claude Code projects root (default: $HOME/.claude/projects)")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "claude-ls: list recent Claude Code sessions\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\nFlags:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		return
	}

	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fatal("cannot resolve home dir: %v", err)
		}
		root = filepath.Join(home, ".claude", "projects")
	}

	var since time.Time
	if sinceStr != "" {
		d, err := parseDuration(sinceStr)
		if err != nil {
			fatal("invalid -since: %v", err)
		}
		since = time.Now().Add(-d)
	}

	files, err := findSessionFiles(root)
	if err != nil {
		fatal("scanning %s: %v", root, err)
	}

	sessions := make([]session, 0, len(files))
	for _, f := range files {
		s, err := parseSession(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", f, err)
			continue
		}
		if s.latest.IsZero() {
			continue
		}
		if !since.IsZero() && s.latest.Before(since) {
			continue
		}
		if cwdSub != "" && !strings.Contains(s.cwd, cwdSub) {
			continue
		}
		if excludeSub != "" && strings.Contains(s.cwd, excludeSub) {
			continue
		}
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].latest.After(sessions[j].latest)
	})

	if !showAll && limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}

	render(sessions, showAll)
}

func findSessionFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".jsonl") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return out, nil
}

func parseSession(path string) (session, error) {
	s := session{file: path}

	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 64<<20)

	var (
		latestUserText      string
		latestUserAt        time.Time
		latestLastPrompt    string
		latestLastPromptSet bool
	)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e rawEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}

		if s.sessionID == "" && e.SessionID != "" {
			s.sessionID = e.SessionID
		}
		if e.Cwd != "" {
			s.cwd = e.Cwd
		}
		if e.GitBranch != "" {
			s.branch = e.GitBranch
		}

		if e.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, e.Timestamp); err == nil {
				if t.After(s.latest) {
					s.latest = t
				}
			}
		}

		switch e.Type {
		case "user":
			if e.IsMeta || len(e.Message) == 0 {
				continue
			}
			text := extractMessageText(e.Message)
			if text == "" || isSyntheticUserText(text) {
				continue
			}
			t, _ := time.Parse(time.RFC3339Nano, e.Timestamp)
			if !t.Before(latestUserAt) {
				latestUserAt = t
				latestUserText = text
			}
		case "last-prompt":
			if e.LastPrompt != "" {
				latestLastPrompt = e.LastPrompt
				latestLastPromptSet = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return s, err
	}

	switch {
	case latestLastPromptSet:
		s.snippet = oneLine(latestLastPrompt)
	case latestUserText != "":
		s.snippet = oneLine(latestUserText)
	}

	if s.cwd == "" {
		s.cwd = decodeProjectDirName(filepath.Base(filepath.Dir(path)))
	}

	return s, nil
}

func extractMessageText(raw json.RawMessage) string {
	var msg rawMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return ""
	}
	if len(msg.Content) == 0 {
		return ""
	}
	// Content can be a plain string or an array of {type, text} blocks.
	if msg.Content[0] == '"' {
		var s string
		if err := json.Unmarshal(msg.Content, &s); err == nil {
			return s
		}
		return ""
	}
	if msg.Content[0] == '[' {
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(msg.Content, &blocks); err == nil {
			var sb strings.Builder
			for _, b := range blocks {
				if b.Type == "text" && b.Text != "" {
					if sb.Len() > 0 {
						sb.WriteByte(' ')
					}
					sb.WriteString(b.Text)
				}
			}
			return sb.String()
		}
	}
	return ""
}

// isSyntheticUserText filters out tool-result envelopes, command stubs, and
// other auto-generated user messages that shouldn't be shown as a "last chat".
func isSyntheticUserText(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return true
	}
	prefixes := []string{
		"<local-command-stdout>",
		"<local-command-stderr>",
		"<local-command-caveat>",
		"<command-name>",
		"<command-message>",
		"<command-args>",
		"<bash-stdout>",
		"<bash-stderr>",
		"Caveat:",
		"[Request interrupted",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// decodeProjectDirName converts "-Users-carlo-bin-litestream" back to a
// path-like string. This is a best-effort fallback for sessions that have no
// cwd recorded in any message.
func decodeProjectDirName(name string) string {
	if !strings.HasPrefix(name, "-") {
		return name
	}
	return strings.ReplaceAll(name, "-", "/")
}

func render(sessions []session, showAll bool) {
	if len(sessions) == 0 {
		fmt.Fprintln(os.Stderr, "no sessions found")
		return
	}

	width := 100
	if !showAll {
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
			width = w
		}
	}

	home, _ := os.UserHomeDir()

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tCWD\tSNIPPET")

	// Compute a snippet column budget so output fits the terminal width.
	const timeColWidth = 16 // "2026-04-20 20:17"
	maxCwdLen := 0
	displayCwds := make([]string, len(sessions))
	for i, s := range sessions {
		c := s.cwd
		if home != "" && strings.HasPrefix(c, home) {
			c = "~" + strings.TrimPrefix(c, home)
		}
		displayCwds[i] = c
		if len(c) > maxCwdLen {
			maxCwdLen = len(c)
		}
	}
	// Cap the cwd column so a single huge path doesn't squash the snippet.
	cwdCol := maxCwdLen
	if !showAll {
		hardCap := width / 2
		if cwdCol > hardCap {
			cwdCol = hardCap
		}
	}

	snippetBudget := max(width-timeColWidth-cwdCol-4, 20) // 2 spaces between cols x2

	for i, s := range sessions {
		ts := s.latest.Local().Format("2006-01-02 15:04")
		cwd := displayCwds[i]
		if !showAll && len(cwd) > cwdCol {
			cwd = truncate(cwd, cwdCol)
		}
		snip := s.snippet
		if !showAll {
			snip = truncate(snip, snippetBudget)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", ts, cwd, snip)
	}
	tw.Flush()
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

// parseDuration accepts Go's standard durations plus a "d" suffix for days.
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		var days float64
		if _, err := fmt.Sscanf(s, "%fd", &days); err != nil {
			return 0, err
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}
	return time.ParseDuration(s)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "claude-ls: "+format+"\n", args...)
	os.Exit(1)
}
