package main

import (
	"fmt"
	"os"
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
	fmt.Println("TODO: list sessions")
}
