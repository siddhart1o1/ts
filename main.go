package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/siddharth/ts/tmux"
	"github.com/siddharth/ts/tui"
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
	fmt.Println("  ts ui" + gray + "                       " + reset + "interactive TUI mode")
	fmt.Println("  ts <name>" + gray + "                   " + reset + "attach or create session")
	fmt.Println("  ts <name> <path>" + gray + "            " + reset + "create session in directory")
	fmt.Println("  ts <name> kill" + gray + "              " + reset + "kill a session")
	fmt.Println("  ts <name> live" + gray + "              " + reset + "attach read-only")
	fmt.Println("  ts <name> rename <new>" + gray + "      " + reset + "rename a session")
	fmt.Println("  ts <name> run <cmd...>" + gray + "      " + reset + "send command to session")
	fmt.Println("  ts detach" + gray + "                   " + reset + "detach from current session")
	fmt.Println("  ts switch <name>" + gray + "            " + reset + "switch client to session")
	fmt.Println("  ts kill-all" + gray + "                 " + reset + "kill all sessions")
	fmt.Println("  ts kill-other" + gray + "               " + reset + "kill all except current")
	fmt.Println("  ts kill-other <name>" + gray + "        " + reset + "kill all except <name>")
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
	case "ui":
		startTUI()
	case "kill-all":
		killAll()
	case "kill-other":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		killOther(name)
	case "detach":
		detachSession()
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

func startTUI() {
	attachTo, err := tui.Run()
	if err != nil {
		fatal("TUI error: " + err.Error())
	}
	if attachTo != "" {
		attachOrCreate(attachTo, "")
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
	_, err := tmux.Cmd("kill-session", "-t", session)
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
	_, err := tmux.Cmd("kill-server")
	if err != nil {
		fmt.Println(gray + "No sessions running." + reset)
		return
	}
	fmt.Println(green + "All sessions killed." + reset)
}

func killOther(keep string) {
	if keep == "" {
		keep = tmux.GetCurrentSession()
		if keep == "" {
			fatal("Not inside tmux. Usage: ts kill-other <name>")
		}
	}

	out, err := tmux.Cmd("list-sessions", "-F", "#{session_name}")
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
		tmux.Cmd("kill-session", "-t", name)
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
	_, err := tmux.Cmd("rename-session", "-t", old, newName)
	if err != nil {
		fatal("Not found: " + old)
	}
	fmt.Println(green + "Renamed: " + reset + old + " -> " + newName)
}

func detachSession() {
	if os.Getenv("TMUX") == "" {
		fatal("Not inside tmux.")
	}
	_, err := tmux.Cmd("detach-client")
	if err != nil {
		fatal("Failed to detach.")
	}
}

func switchSession(name string) {
	if os.Getenv("TMUX") == "" {
		fatal("Not inside tmux. Use: ts " + name)
	}
	_, err := tmux.Cmd("switch-client", "-t", name)
	if err != nil {
		fatal("Not found: " + name)
	}
}

func runInSession(session string, cmdArgs []string) {
	command := strings.Join(cmdArgs, " ")
	_, err := tmux.Cmd("send-keys", "-t", session, command, "Enter")
	if err != nil {
		fatal("Not found: " + session)
	}
	fmt.Println(green + "Sent to " + session + ": " + reset + command)
}

func listSessions() {
	sessions, err := tmux.ListSessions()
	if err != nil || len(sessions) == 0 {
		fmt.Println(gray + "No active sessions." + reset)
		fmt.Println()
		usage()
		return
	}

	current := tmux.GetCurrentSession()

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	for _, s := range sessions {
		winLabel := fmt.Sprintf("%d win", s.Windows)
		if s.Windows != 1 {
			winLabel += "s"
		}

		marker := "  "
		if s.Name == current {
			marker = yellow + "-> " + reset
		}

		if s.Attached {
			fmt.Fprintf(w, "%s%s%s%s\t%s\t%s\t%s\t%s(attached)%s\n",
				marker, green+bold, s.Name, reset,
				s.Dir, s.Command, winLabel,
				green, reset)
		} else {
			fmt.Fprintf(w, "%s%s%s%s\t%s\t%s\t%s\t%s\n",
				marker, dim, s.Name, reset,
				gray+s.Dir+reset, gray+s.Command+reset, gray+winLabel+reset,
				gray+"detached"+reset)
		}
	}
	w.Flush()
	fmt.Println()
	usage()
}
