package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	gray   = "\033[90m"
)

func usage() {
	fmt.Println(bold + "ts" + reset + " — tmux session manager\n")
	fmt.Println("Usage:")
	fmt.Println("  ts" + gray + "                          " + reset + "list sessions")
	fmt.Println("  ts <name>" + gray + "                   " + reset + "attach or create session")
	fmt.Println("  ts <name> <path>" + gray + "            " + reset + "create session in directory")
	fmt.Println("  ts <name> kill" + gray + "              " + reset + "kill a session")
	fmt.Println("  ts <name> live" + gray + "              " + reset + "attach read-only")
	fmt.Println("  ts <name> rename <new>" + gray + "      " + reset + "rename a session")
	fmt.Println("  ts <name> run <cmd...>" + gray + "      " + reset + "send command to session")
	fmt.Println("  ts switch <name>" + gray + "            " + reset + "switch client to session")
	fmt.Println("  ts kill-all" + gray + "                 " + reset + "kill all sessions")
	fmt.Println("  ts kill-other" + gray + "               " + reset + "kill all except current")
	fmt.Println("  ts kill-other <name>" + gray + "        " + reset + "kill all except <name>")
}

func tmuxCmd(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func shortenHome(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "\033[31m"+msg+reset)
	os.Exit(1)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		listSessions()
		return
	}

	session := args[0]
	action := ""
	if len(args) > 1 {
		action = args[1]
	}

	switch session {
	case "kill-all":
		killAll()
	case "kill-other":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		killOther(name)
	case "switch":
		if len(args) < 2 {
			fatal("Usage: ts switch <name>")
		}
		switchSession(args[1])
	case "help", "--help", "-h":
		usage()
	default:
		switch action {
		case "":
			attachOrCreate(session, "")
		case "kill":
			killSession(session)
		case "live":
			liveSession(session)
		case "rename":
			if len(args) < 3 {
				fatal("Usage: ts <name> rename <new-name>")
			}
			renameSession(session, args[2])
		case "run":
			if len(args) < 3 {
				fatal("Usage: ts <name> run <command...>")
			}
			runInSession(session, args[2:])
		default:
			// ts <name> <path> — create session at path
			attachOrCreate(session, action)
		}
	}
}

func attachOrCreate(session, dir string) {
	// Try attach first
	if dir == "" {
		cmd := exec.Command("tmux", "attach", "-t", session)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if cmd.Run() == nil {
			return
		}
	}

	// Create new session
	args := []string{"new-session", "-s", session}
	if dir != "" {
		absDir := dir
		if !filepath.IsAbs(dir) {
			if wd, err := os.Getwd(); err == nil {
				absDir = filepath.Join(wd, dir)
			}
		}
		args = append(args, "-c", absDir)
	}
	cmd := exec.Command("tmux", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("Failed to create session: " + session)
	}
}

func killSession(session string) {
	_, err := tmuxCmd("kill-session", "-t", session)
	if err != nil {
		fatal("Not found: " + session)
	}
	fmt.Println(green + "Killed: " + reset + session)
}

func liveSession(session string) {
	cmd := exec.Command("tmux", "attach", "-t", session, "-r")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("Not found: " + session)
	}
}

func killAll() {
	_, err := tmuxCmd("kill-server")
	if err != nil {
		fmt.Println(gray + "No sessions running." + reset)
		return
	}
	fmt.Println(green + "All sessions killed." + reset)
}

func getCurrentSession() string {
	tmuxEnv := os.Getenv("TMUX")
	if tmuxEnv == "" {
		return ""
	}
	name, err := tmuxCmd("display-message", "-p", "#{session_name}")
	if err != nil {
		return ""
	}
	return name
}

func killOther(keep string) {
	if keep == "" {
		keep = getCurrentSession()
		if keep == "" {
			fatal("Not inside tmux. Usage: ts kill-other <name>")
		}
	}

	out, err := tmuxCmd("list-sessions", "-F", "#{session_name}")
	if err != nil || out == "" {
		fmt.Println(gray + "No sessions running." + reset)
		return
	}

	killed := 0
	for _, name := range strings.Split(out, "\n") {
		name = strings.TrimSpace(name)
		if name == "" || name == keep {
			continue
		}
		tmuxCmd("kill-session", "-t", name)
		fmt.Println(gray + "  Killed: " + name + reset)
		killed++
	}

	if killed == 0 {
		fmt.Println(gray + "No other sessions to kill." + reset)
	} else {
		fmt.Printf("%sKilled %d session(s). Kept: %s%s\n", green, killed, keep, reset)
	}
}

func renameSession(old, newName string) {
	_, err := tmuxCmd("rename-session", "-t", old, newName)
	if err != nil {
		fatal("Not found: " + old)
	}
	fmt.Println(green + "Renamed: " + reset + old + " → " + newName)
}

func switchSession(name string) {
	if os.Getenv("TMUX") == "" {
		fatal("Not inside tmux. Use: ts " + name)
	}
	_, err := tmuxCmd("switch-client", "-t", name)
	if err != nil {
		fatal("Not found: " + name)
	}
}

func runInSession(session string, cmdArgs []string) {
	command := strings.Join(cmdArgs, " ")
	_, err := tmuxCmd("send-keys", "-t", session, command, "Enter")
	if err != nil {
		fatal("Not found: " + session)
	}
	fmt.Println(green + "Sent to " + session + ": " + reset + command)
}

func listSessions() {
	out, err := tmuxCmd("list-sessions", "-F", "#{session_name}\t#{session_windows}\t#{session_attached}\t#{pane_current_path}\t#{pane_current_command}")
	if err != nil || out == "" {
		fmt.Println(gray + "No active sessions." + reset)
		fmt.Println()
		usage()
		return
	}

	current := getCurrentSession()

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) < 5 {
			continue
		}
		name := parts[0]
		wins := parts[1]
		attached := parts[2]
		dir := shortenHome(parts[3])
		cmd := parts[4]

		winLabel := wins + " win"
		if wins != "1" {
			winLabel += "s"
		}

		marker := "  "
		if name == current {
			marker = yellow + "→ " + reset
		}

		if attached != "0" {
			fmt.Fprintf(w, "%s%s%s%s\t%s\t%s\t%s\t%s(attached)%s\n",
				marker, green+bold, name, reset,
				dir, cmd, winLabel,
				green, reset)
		} else {
			fmt.Fprintf(w, "%s%s%s%s\t%s\t%s\t%s\t%s\n",
				marker, dim, name, reset,
				gray+dir+reset, gray+cmd+reset, gray+winLabel+reset,
				gray+"detached"+reset)
		}
	}
	w.Flush()
	fmt.Println()
	usage()
}
