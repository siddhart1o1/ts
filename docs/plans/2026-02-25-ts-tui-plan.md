# ts TUI Mode — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `ts ui` command that launches a Bubble Tea TUI with split-pane session list + live preview.

**Architecture:** Bubble Tea (Elm architecture) with a single model holding session list state, preview content, input mode, and terminal dimensions. Left panel renders session list, right panel renders captured pane content. A 1-second tick refreshes session data and preview. All tmux interaction reuses the existing `tmuxCmd()` helper extracted to a shared package.

**Tech Stack:** Go 1.22, Bubble Tea, Lip Gloss, `tmux capture-pane` for preview.

---

### Task 1: Add dependencies and extract shared tmux helpers

**Files:**
- Modify: `~/Projects/ts/go.mod`
- Create: `~/Projects/ts/tmux/tmux.go`
- Modify: `~/Projects/ts/main.go`

**Step 1: Install Bubble Tea and Lip Gloss**

```bash
cd ~/Projects/ts
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss
```

**Step 2: Create shared tmux package**

Create `~/Projects/ts/tmux/tmux.go`:

```go
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
```

**Step 3: Update main.go to use shared package**

Replace all calls to the local `tmuxCmd()` with `tmux.Cmd()`, `shortenHome()` with `tmux.ShortenHome()`, `getCurrentSession()` with `tmux.GetCurrentSession()`. Remove the local versions of those functions. Update the import to include `"github.com/siddharth/ts/tmux"`.

Also update the `listSessions()` function to use `tmux.ListSessions()` struct-based API.

Add `"ui"` case to the router in `main()`:

```go
case "ui":
	startTUI()
```

Add a stub:
```go
func startTUI() {
	fmt.Println("TODO: TUI mode")
}
```

**Step 4: Build and verify**

```bash
cd ~/Projects/ts && go build -o ts . && ./ts && ./ts help
```

Expected: existing CLI works exactly as before. `./ts ui` prints "TODO: TUI mode".

**Step 5: Commit**

```bash
cd ~/Projects/ts && git add -A && git commit -m "refactor: extract shared tmux package, add ui stub"
```

---

### Task 2: Styles and basic TUI scaffold

**Files:**
- Create: `~/Projects/ts/tui/styles.go`
- Create: `~/Projects/ts/tui/tui.go`
- Modify: `~/Projects/ts/main.go`

**Step 1: Create styles**

Create `~/Projects/ts/tui/styles.go`:

```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Gruvbox Material colors
	colorBg        = lipgloss.Color("#282828")
	colorBorder    = lipgloss.Color("#3c3836")
	colorGreen     = lipgloss.Color("#a9b665")
	colorYellow    = lipgloss.Color("#d8a657")
	colorBlue      = lipgloss.Color("#7daea3")
	colorGray      = lipgloss.Color("#928374")
	colorFg        = lipgloss.Color("#d4be98")
	colorRed       = lipgloss.Color("#ea6962")

	listPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	previewPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorFg)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorYellow)

	currentMarkerStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	attachedStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	detachedStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	windowStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			PaddingLeft(2)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	inputStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed)
)
```

**Step 2: Create basic TUI scaffold**

Create `~/Projects/ts/tui/tui.go`:

```go
package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddharth/ts/tmux"
)

type mode int

const (
	modeNormal mode = iota
	modeRename
	modeCommand
	modeNewSession
	modeConfirmKill
)

type listItem struct {
	session   *tmux.Session
	window    *tmux.Window
	sessionID int // index into sessions slice
}

type model struct {
	sessions   []tmux.Session
	windows    map[string][]tmux.Window // session name -> windows
	expanded   map[string]bool          // which sessions are expanded
	items      []listItem               // flattened list for navigation
	cursor     int
	preview    string
	current    string // current session name
	width      int
	height     int
	mode       mode
	input      string // text input for rename/command/new
	inputLabel string
	err        string
	quitting   bool
	attachTo   string // session to attach to after quit
}

func New() model {
	return model{
		windows:  make(map[string][]tmux.Window),
		expanded: make(map[string]bool),
		current:  tmux.GetCurrentSession(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		refreshSessions,
		tickCmd(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.mode != modeNormal {
			return m.updateInput(msg)
		}
		return m.updateNormal(msg)
	case sessionsMsg:
		m.sessions = msg
		m.rebuildItems()
		return m, m.refreshPreview()
	case windowsMsg:
		m.windows[msg.session] = msg.windows
		m.rebuildItems()
		return m, nil
	case previewMsg:
		m.preview = string(msg)
		return m, nil
	case tickMsg:
		return m, tea.Batch(refreshSessions, m.refreshPreview(), tickCmd())
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	listWidth := m.width * 30 / 100
	if listWidth < 25 {
		listWidth = 25
	}
	previewWidth := m.width - listWidth

	helpHeight := 1
	panelHeight := m.height - helpHeight - 2

	list := m.renderList(listWidth-4, panelHeight-2)
	preview := m.renderPreview(previewWidth-4, panelHeight-2)

	listPanel := listPanelStyle.
		Width(listWidth - 2).
		Height(panelHeight).
		Render(list)

	previewTitle := "Preview"
	if sel := m.selectedSession(); sel != nil {
		previewTitle = fmt.Sprintf("Preview (%s)", sel.Name)
	}
	previewPanel := previewPanelStyle.
		Width(previewWidth - 2).
		Height(panelHeight).
		BorderTop(true).
		Render(titleStyle.Render(previewTitle) + "\n" + preview)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, previewPanel)

	help := m.renderHelp()

	return panels + "\n" + help
}

func Run() (string, error) {
	p := tea.NewProgram(New(), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}
	m := finalModel.(model)
	return m.attachTo, nil
}
```

**Step 3: Wire up main.go**

Replace the `startTUI` stub in `main.go`:

```go
import "github.com/siddharth/ts/tui"

func startTUI() {
	attachTo, err := tui.Run()
	if err != nil {
		fatal("TUI error: " + err.Error())
	}
	if attachTo != "" {
		attachOrCreate(attachTo, "")
	}
}
```

**Step 4: Build (will have compile errors due to missing functions — that's OK, just verify no import issues)**

This step is about getting the scaffold in place. The missing functions (updateNormal, updateInput, renderList, renderPreview, renderHelp, rebuildItems, selectedSession, refreshSessions, tickCmd, refreshPreview, message types) will be added in subsequent tasks.

Add temporary stubs so it compiles:

```go
// At bottom of tui.go temporarily:
type sessionsMsg []tmux.Session
type windowsMsg struct {
	session string
	windows []tmux.Window
}
type previewMsg string
type tickMsg struct{}

func refreshSessions() tea.Msg { return sessionsMsg(nil) }
func tickCmd() tea.Cmd { return nil }
func (m model) refreshPreview() tea.Cmd { return nil }
func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) { return m, nil }
func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) { return m, nil }
func (m model) renderList(w, h int) string { return "TODO" }
func (m model) renderPreview(w, h int) string { return "TODO" }
func (m model) renderHelp() string { return "q quit" }
func (m model) rebuildItems() {}
func (m model) selectedSession() *tmux.Session { return nil }
```

```bash
cd ~/Projects/ts && go build -o ts . && ./ts ui
```

Expected: opens alternate screen, shows "TODO" panels, q quits.

**Step 5: Commit**

```bash
cd ~/Projects/ts && git add -A && git commit -m "feat: TUI scaffold with styles and basic model"
```

---

### Task 3: Session data fetching and list rendering

**Files:**
- Modify: `~/Projects/ts/tui/tui.go`

**Step 1: Implement message types and data fetching commands**

Replace the temporary stubs for data fetching:

```go
import "time"

type sessionsMsg []tmux.Session
type windowsMsg struct {
	session string
	windows []tmux.Window
}
type previewMsg string
type tickMsg struct{}

func refreshSessions() tea.Msg {
	sessions, _ := tmux.ListSessions()
	return sessionsMsg(sessions)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m model) refreshPreview() tea.Cmd {
	target := ""
	if m.cursor >= 0 && m.cursor < len(m.items) {
		item := m.items[m.cursor]
		if item.window != nil {
			target = fmt.Sprintf("%s:%d", m.sessions[item.sessionID].Name, item.window.Index)
		} else if item.session != nil {
			target = item.session.Name
		}
	}
	if target == "" {
		return nil
	}
	return func() tea.Msg {
		return previewMsg(tmux.CapturePane(target))
	}
}
```

**Step 2: Implement rebuildItems and selectedSession**

```go
func (m *model) rebuildItems() {
	m.items = nil
	for i := range m.sessions {
		s := &m.sessions[i]
		m.items = append(m.items, listItem{session: s, sessionID: i})
		if m.expanded[s.Name] {
			if wins, ok := m.windows[s.Name]; ok {
				for j := range wins {
					m.items = append(m.items, listItem{window: &wins[j], sessionID: i})
				}
			}
		}
	}
	if m.cursor >= len(m.items) && len(m.items) > 0 {
		m.cursor = len(m.items) - 1
	}
}

func (m model) selectedSession() *tmux.Session {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	return &m.sessions[m.items[m.cursor].sessionID]
}
```

**Step 3: Implement renderList**

```go
import "strconv"

func (m model) renderList(w, h int) string {
	var lines []string
	lines = append(lines, titleStyle.Render("Sessions"))
	lines = append(lines, "")

	for i, item := range m.items {
		if item.window != nil {
			// Window line
			prefix := "├─ "
			// Check if this is the last window of its session
			isLast := i+1 >= len(m.items) || m.items[i+1].window == nil
			if isLast {
				prefix = "└─ "
			}
			name := fmt.Sprintf("%d:%s", item.window.Index, item.window.Name)
			if item.window.Active {
				name += " *"
			}
			style := windowStyle
			if i == m.cursor {
				style = selectedStyle.PaddingLeft(2)
			}
			lines = append(lines, style.Render(prefix+name))
			continue
		}

		// Session line
		s := item.session
		marker := "  "
		if s.Name == m.current {
			marker = currentMarkerStyle.Render("→ ")
		}

		winLabel := strconv.Itoa(s.Windows) + " win"
		if s.Windows != 1 {
			winLabel += "s"
		}

		var line string
		if i == m.cursor {
			line = marker + selectedStyle.Render(s.Name) + " " + detachedStyle.Render(winLabel)
		} else if s.Attached {
			line = marker + attachedStyle.Bold(true).Render(s.Name) + " " + detachedStyle.Render(winLabel)
		} else {
			line = marker + detachedStyle.Render(s.Name+" "+winLabel)
		}

		if s.Attached {
			line += " " + attachedStyle.Render("●")
		}

		lines = append(lines, line)
	}

	result := ""
	for i, line := range lines {
		if i >= h {
			break
		}
		result += line + "\n"
	}
	return result
}
```

**Step 4: Build and test**

```bash
cd ~/Projects/ts && go build -o ts . && ./ts ui
```

Expected: left panel shows session list with names, window counts, current session marker. Right panel still "TODO".

**Step 5: Commit**

```bash
cd ~/Projects/ts && git add -A && git commit -m "feat: TUI session list with data fetching and auto-refresh"
```

---

### Task 4: Preview panel and key navigation

**Files:**
- Modify: `~/Projects/ts/tui/tui.go`

**Step 1: Implement renderPreview**

```go
func (m model) renderPreview(w, h int) string {
	if m.preview == "" {
		return detachedStyle.Render("No preview available")
	}
	lines := strings.Split(m.preview, "\n")
	// Truncate to panel height
	if len(lines) > h {
		lines = lines[len(lines)-h:]
	}
	// Truncate each line to panel width
	for i, line := range lines {
		if lipgloss.Width(line) > w {
			lines[i] = line[:w]
		}
	}
	return strings.Join(lines, "\n")
}
```

Add `"strings"` to imports.

**Step 2: Implement updateNormal (key handling)**

```go
func fetchWindows(session string) tea.Cmd {
	return func() tea.Msg {
		wins, _ := tmux.ListWindows(session)
		return windowsMsg{session: session, windows: wins}
	}
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			return m, m.refreshPreview()
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			return m, m.refreshPreview()
		}
	case "enter":
		if s := m.selectedSession(); s != nil {
			m.attachTo = s.Name
			m.quitting = true
			return m, tea.Quit
		}
	case "tab", "right", "l":
		if m.cursor >= 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.session != nil {
				name := item.session.Name
				if m.expanded[name] {
					m.expanded[name] = false
					m.rebuildItems()
				} else {
					m.expanded[name] = true
					return m, fetchWindows(name)
				}
			}
		}
	case "left", "h":
		if m.cursor >= 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.session != nil && m.expanded[item.session.Name] {
				m.expanded[item.session.Name] = false
				m.rebuildItems()
			} else if item.window != nil {
				// Jump back to parent session
				name := m.sessions[item.sessionID].Name
				m.expanded[name] = false
				m.rebuildItems()
				// Move cursor to the session
				for i, it := range m.items {
					if it.session != nil && it.session.Name == name {
						m.cursor = i
						break
					}
				}
			}
		}
	case "x":
		if s := m.selectedSession(); s != nil {
			m.mode = modeConfirmKill
			m.inputLabel = fmt.Sprintf("Kill '%s'? (y/n)", s.Name)
		}
	case "r":
		if s := m.selectedSession(); s != nil {
			m.mode = modeRename
			m.input = s.Name
			m.inputLabel = "Rename to:"
		}
	case "c":
		if s := m.selectedSession(); s != nil {
			m.mode = modeCommand
			m.input = ""
			m.inputLabel = fmt.Sprintf("Send to '%s':", s.Name)
		}
	case "n":
		m.mode = modeNewSession
		m.input = ""
		m.inputLabel = "New session name:"
	case "K":
		current := m.current
		if current == "" {
			if s := m.selectedSession(); s != nil {
				current = s.Name
			}
		}
		if current != "" {
			for _, s := range m.sessions {
				if s.Name != current {
					tmux.Cmd("kill-session", "-t", s.Name)
				}
			}
			return m, refreshSessions
		}
	}
	return m, nil
}
```

**Step 3: Implement renderHelp**

```go
func (m model) renderHelp() string {
	if m.mode != modeNormal {
		return helpStyle.Render(" esc cancel  enter confirm")
	}
	return helpStyle.Render(" ↑↓/jk navigate  →/tab expand  enter attach  n new  x kill  r rename  c cmd  K kill-other  q quit")
}
```

**Step 4: Build and test**

```bash
cd ~/Projects/ts && go build -o ts . && ./ts ui
```

Expected: full split-pane TUI. Navigate with j/k, preview updates live. Tab expands windows. q quits.

**Step 5: Commit**

```bash
cd ~/Projects/ts && git add -A && git commit -m "feat: TUI preview panel and key navigation"
```

---

### Task 5: Input modes (rename, command, new session, confirm kill)

**Files:**
- Modify: `~/Projects/ts/tui/tui.go`

**Step 1: Implement updateInput**

```go
func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input = ""
		m.err = ""
		return m, nil
	case "enter":
		return m.executeInput()
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil
	case "ctrl+c":
		m.mode = modeNormal
		m.input = ""
		m.err = ""
		return m, nil
	default:
		if m.mode == modeConfirmKill {
			switch msg.String() {
			case "y":
				return m.executeInput()
			case "n":
				m.mode = modeNormal
				return m, nil
			}
			return m, nil
		}
		// Regular character input
		if len(msg.String()) == 1 {
			m.input += msg.String()
		}
		return m, nil
	}
}

func (m model) executeInput() (tea.Model, tea.Cmd) {
	s := m.selectedSession()
	switch m.mode {
	case modeRename:
		if m.input != "" && s != nil {
			_, err := tmux.Cmd("rename-session", "-t", s.Name, m.input)
			if err != nil {
				m.err = "Rename failed"
			}
		}
	case modeCommand:
		if m.input != "" && s != nil {
			tmux.Cmd("send-keys", "-t", s.Name, m.input, "Enter")
		}
	case modeNewSession:
		if m.input != "" {
			tmux.Cmd("new-session", "-d", "-s", m.input)
		}
	case modeConfirmKill:
		if s != nil {
			tmux.Cmd("kill-session", "-t", s.Name)
		}
	}
	m.mode = modeNormal
	m.input = ""
	m.err = ""
	return m, refreshSessions
}
```

**Step 2: Add input rendering to the View**

Update `renderHelp` to show input when in input mode:

```go
func (m model) renderHelp() string {
	if m.mode == modeConfirmKill {
		return helpStyle.Render(" " + m.inputLabel)
	}
	if m.mode != modeNormal {
		prompt := inputStyle.Render(m.inputLabel) + " " + m.input + "█"
		if m.err != "" {
			prompt += " " + errorStyle.Render(m.err)
		}
		return " " + prompt + "  " + helpStyle.Render("esc cancel  enter confirm")
	}
	return helpStyle.Render(" ↑↓/jk navigate  →/tab expand  enter attach  n new  x kill  r rename  c cmd  K kill-other  q quit")
}
```

**Step 3: Build and test**

```bash
cd ~/Projects/ts && go build -o ts . && ./ts ui
```

Test:
- Press `n`, type a session name, press Enter → new session appears
- Press `r` on a session, type new name, Enter → renamed
- Press `c`, type a command, Enter → sent to session (verify in preview)
- Press `x` on a session, press `y` → session killed

**Step 4: Commit**

```bash
cd ~/Projects/ts && git add -A && git commit -m "feat: TUI input modes for rename, command, new session, kill"
```

---

### Task 6: Polish, install, and final testing

**Files:**
- Modify: `~/Projects/ts/tui/tui.go`
- Modify: `~/Projects/ts/main.go`

**Step 1: Add "ui" to usage output in main.go**

In the `usage()` function, add:
```go
fmt.Println("  ts ui" + gray + "                      " + reset + "open TUI session manager")
```

**Step 2: Handle edge cases in the TUI**

In `tui.go`, ensure:
- Empty session list shows a helpful message in the list panel
- Preview panel shows "No sessions" when list is empty
- Window expansion fetches data immediately when sessions refresh

**Step 3: Build release binary and install**

```bash
cd ~/Projects/ts && go build -o ts . && cp ts ~/.local/bin/ts
```

**Step 4: Full end-to-end test**

```bash
ts              # CLI list still works
ts help         # Shows "ts ui" in commands
ts ui           # Opens TUI, verify:
                #   - Sessions show in left panel
                #   - Current session marked with →
                #   - Preview auto-refreshes (run something in another session)
                #   - j/k navigate
                #   - Tab expands windows
                #   - n creates session
                #   - r renames
                #   - c sends command
                #   - x kills (with confirmation)
                #   - K kills others
                #   - Enter attaches
                #   - q quits
```

**Step 5: Commit**

```bash
cd ~/Projects/ts && git add -A && git commit -m "feat: finalize TUI mode with polish and install"
```

---

## Summary

| Task | What | ~Minutes |
|------|------|----------|
| 1 | Extract shared tmux package, add deps | 5 |
| 2 | Styles and TUI scaffold | 5 |
| 3 | Session list rendering and data fetching | 5 |
| 4 | Preview panel and key navigation | 5 |
| 5 | Input modes (rename, command, new, kill) | 5 |
| 6 | Polish, install, test | 5 |
