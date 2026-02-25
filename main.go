package main

import (
	"fmt"
	"os"
	"os/exec"
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

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		listSessions()
		return
	}
	// TODO: route commands
	usage()
}

func listSessions() {
	out, err := tmuxCmd("list-sessions", "-F", "#{session_name}\t#{session_windows}\t#{session_attached}\t#{pane_current_path}\t#{pane_current_command}")
	if err != nil || out == "" {
		fmt.Println(gray + "No active sessions." + reset)
		fmt.Println()
		usage()
		return
	}

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

		if attached != "0" {
			fmt.Fprintf(w, "%s%s%s\t%s\t%s\t%s\t%s(attached)%s\n",
				green+bold, name, reset,
				dir, cmd, winLabel,
				green, reset)
		} else {
			fmt.Fprintf(w, "%s%s%s\t%s\t%s\t%s\t%s\n",
				dim, name, reset,
				gray+dir+reset, gray+cmd+reset, gray+winLabel+reset,
				gray+"detached"+reset)
		}
	}
	w.Flush()
}
