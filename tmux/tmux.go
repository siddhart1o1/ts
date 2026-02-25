package tmux

import (
	"os"
	"os/exec"
	"strings"
)

func Cmd(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func ShortenHome(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func GetCurrentSession() string {
	tmuxEnv := os.Getenv("TMUX")
	if tmuxEnv == "" {
		return ""
	}
	name, err := Cmd("display-message", "-p", "#{session_name}")
	if err != nil {
		return ""
	}
	return name
}

type Session struct {
	Name     string
	Windows  int
	Attached bool
	Dir      string
	Command  string
}

type Window struct {
	Index  int
	Name   string
	Active bool
}

func ListSessions() ([]Session, error) {
	out, err := Cmd("list-sessions", "-F",
		"#{session_name}\t#{session_windows}\t#{session_attached}\t#{pane_current_path}\t#{pane_current_command}")
	if err != nil || out == "" {
		return nil, err
	}

	var sessions []Session
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 5 {
			continue
		}
		wins := 0
		for _, c := range parts[1] {
			wins = wins*10 + int(c-'0')
		}
		sessions = append(sessions, Session{
			Name:     parts[0],
			Windows:  wins,
			Attached: parts[2] != "0",
			Dir:      ShortenHome(parts[3]),
			Command:  parts[4],
		})
	}
	return sessions, nil
}

func ListWindows(session string) ([]Window, error) {
	out, err := Cmd("list-windows", "-t", session, "-F",
		"#{window_index}\t#{window_name}\t#{window_active}")
	if err != nil || out == "" {
		return nil, err
	}

	var windows []Window
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		idx := 0
		for _, c := range parts[0] {
			idx = idx*10 + int(c-'0')
		}
		windows = append(windows, Window{
			Index:  idx,
			Name:   parts[1],
			Active: parts[2] == "1",
		})
	}
	return windows, nil
}

func CapturePane(target string) string {
	out, _ := Cmd("capture-pane", "-t", target, "-p")
	return out
}
