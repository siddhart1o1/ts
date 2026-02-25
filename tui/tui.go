package tui

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddharth/ts/tmux"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x1b\\|\x1b\][^\x07]*\x07`)

// modes
const (
	modeNormal = iota
	modeRename
	modeCommand
	modeNewSession
	modeConfirmKill
	modeConfirmKillProtected
)

// message types
type sessionsMsg []tmux.Session

type windowsMsg struct {
	session string
	windows []tmux.Window
}

type previewMsg string

// listItem represents a single item in the flattened list
type listItem struct {
	session   *tmux.Session
	window    *tmux.Window
	sessionID int
}

// model is the Bubble Tea model
type model struct {
	sessions   []tmux.Session
	windows    map[string][]tmux.Window
	expanded   map[string]bool
	items      []listItem
	cursor     int
	preview    string
	current    string // current tmux session name
	width      int
	height     int
	mode       int
	input      string
	inputLabel string
	err        string
	quitting   bool
	attachTo   string
}

func newModel() model {
	return model{
		windows:  make(map[string][]tmux.Window),
		expanded: make(map[string]bool),
		current:  tmux.GetCurrentSession(),
	}
}

func (m model) Init() tea.Cmd {
	return refreshSessions
}

// --- data fetching ---

func refreshSessions() tea.Msg {
	sessions, _ := tmux.ListSessions()
	return sessionsMsg(sessions)
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

func fetchWindows(session string) tea.Cmd {
	return func() tea.Msg {
		wins, _ := tmux.ListWindows(session)
		return windowsMsg{session: session, windows: wins}
	}
}

// --- list management ---

func rebuildItems(sessions []tmux.Session, expanded map[string]bool, windows map[string][]tmux.Window, oldCursor int) ([]listItem, int) {
	var items []listItem
	for i := range sessions {
		s := &sessions[i]
		items = append(items, listItem{session: s, sessionID: i})
		if expanded[s.Name] {
			if wins, ok := windows[s.Name]; ok {
				for j := range wins {
					items = append(items, listItem{window: &wins[j], sessionID: i})
				}
			}
		}
	}
	cursor := oldCursor
	if cursor >= len(items) && len(items) > 0 {
		cursor = len(items) - 1
	}
	if cursor < 0 && len(items) > 0 {
		cursor = 0
	}
	return items, cursor
}

func (m model) selectedSession() *tmux.Session {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	return &m.sessions[m.items[m.cursor].sessionID]
}

// --- Update ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sessionsMsg:
		m.sessions = []tmux.Session(msg)
		m.items, m.cursor = rebuildItems(m.sessions, m.expanded, m.windows, m.cursor)
		return m, m.refreshPreview()

	case windowsMsg:
		m.windows[msg.session] = msg.windows
		m.items, m.cursor = rebuildItems(m.sessions, m.expanded, m.windows, m.cursor)
		return m, m.refreshPreview()

	case previewMsg:
		m.preview = string(msg)
		return m, nil

	case tea.KeyMsg:
		if m.mode == modeNormal {
			return m.updateNormal(msg)
		}
		return m.updateInput(msg)
	}

	return m, nil
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit

	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			return m, m.refreshPreview()
		}

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			return m, m.refreshPreview()
		}

	case "g":
		m.cursor = 0
		return m, m.refreshPreview()

	case "G":
		if len(m.items) > 0 {
			m.cursor = len(m.items) - 1
			return m, m.refreshPreview()
		}

	case "enter":
		s := m.selectedSession()
		if s != nil {
			m.attachTo = s.Name
			m.quitting = true
			return m, tea.Quit
		}

	case "tab", "right", "l":
		if m.cursor >= 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.session != nil {
				name := item.session.Name
				if !m.expanded[name] {
					m.expanded[name] = true
					m.items, m.cursor = rebuildItems(m.sessions, m.expanded, m.windows, m.cursor)
					return m, fetchWindows(name)
				}
			}
		}

	case "shift+tab", "left", "h":
		if m.cursor >= 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.window != nil {
				name := m.sessions[item.sessionID].Name
				for i, it := range m.items {
					if it.session != nil && it.session.Name == name {
						m.cursor = i
						break
					}
				}
				m.expanded[name] = false
				m.items, m.cursor = rebuildItems(m.sessions, m.expanded, m.windows, m.cursor)
				return m, m.refreshPreview()
			}
			if item.session != nil {
				m.expanded[item.session.Name] = false
				m.items, m.cursor = rebuildItems(m.sessions, m.expanded, m.windows, m.cursor)
				return m, m.refreshPreview()
			}
		}

	case "x":
		s := m.selectedSession()
		if s != nil {
			if tmux.IsProtected(s.Name) {
				m.mode = modeConfirmKillProtected
				m.inputLabel = fmt.Sprintf("Protected! Type 'yes %s' to kill: ", s.Name)
				m.input = ""
			} else {
				m.mode = modeConfirmKill
				m.inputLabel = fmt.Sprintf("Kill '%s'? (y/n)", s.Name)
				m.input = ""
			}
			return m, nil
		}

	case "p":
		s := m.selectedSession()
		if s != nil {
			tmux.SetProtected(s.Name, !tmux.IsProtected(s.Name))
			return m, refreshSessions
		}

	case "r":
		s := m.selectedSession()
		if s != nil {
			m.mode = modeRename
			m.inputLabel = "Rename to: "
			m.input = ""
			return m, nil
		}

	case "c":
		s := m.selectedSession()
		if s != nil {
			m.mode = modeCommand
			m.inputLabel = fmt.Sprintf("Send to '%s': ", s.Name)
			m.input = ""
			return m, nil
		}

	case "n":
		m.mode = modeNewSession
		m.inputLabel = "New session: "
		m.input = ""
		return m, nil

	case "R":
		return m, tea.Batch(refreshSessions, m.refreshPreview())

	case "K":
		current := m.current
		if current == "" {
			s := m.selectedSession()
			if s != nil {
				current = s.Name
			}
		}
		if current != "" {
			for _, s := range m.sessions {
				if s.Name != current && !tmux.IsProtected(s.Name) {
					tmux.Cmd("kill-session", "-t", "="+s.Name)
				}
			}
			return m, refreshSessions
		}
	}

	return m, nil
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input = ""
		m.err = ""
		return m, nil

	case "enter":
		if m.mode == modeConfirmKill {
			m.mode = modeNormal
			m.input = ""
			m.err = ""
			return m, nil
		}
		if m.mode == modeConfirmKillProtected {
			s := m.selectedSession()
			if s != nil && m.input == "yes "+s.Name {
				return m.executeInput()
			}
			m.err = "Confirmation did not match."
			m.input = ""
			return m, nil
		}
		return m.executeInput()

	case "y":
		if m.mode == modeConfirmKill {
			return m.executeInput()
		}
		m.input += "y"
		return m, nil

	case "n":
		if m.mode == modeConfirmKill {
			m.mode = modeNormal
			m.input = ""
			m.err = ""
			return m, nil
		}
		m.input += "n"
		return m, nil

	case " ":
		m.input += " "
		return m, nil

	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil

	default:
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
			tmux.Cmd("rename-session", "-t", "="+s.Name, m.input)
		}
	case modeCommand:
		if m.input != "" && s != nil {
			tmux.Cmd("send-keys", "-t", "="+s.Name, m.input, "Enter")
		}
	case modeNewSession:
		if m.input != "" {
			tmux.Cmd("new-session", "-d", "-s", m.input)
		}
	case modeConfirmKill:
		if s != nil {
			tmux.Cmd("kill-session", "-t", "="+s.Name)
		}
	case modeConfirmKillProtected:
		if s != nil {
			tmux.Cmd("kill-session", "-t", "="+s.Name)
		}
	}
	m.mode = modeNormal
	m.input = ""
	m.err = ""
	return m, refreshSessions
}

// --- View ---
// Built line-by-line with exact width control. No lipgloss layout functions.

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return ""
	}

	w := m.width - 1 // 1 char safety margin to prevent line wrapping
	h := m.height

	// Layout: list panel | separator | preview panel, then help line
	listW := w * 3 / 10
	if listW < 20 {
		listW = 20
	}
	// 1 char for separator
	previewW := w - listW - 1
	if previewW < 10 {
		previewW = 10
	}
	bodyH := h - 1 // last line = help

	listLines := m.getListLines(listW, bodyH)
	previewLines := m.getPreviewLines(previewW, bodyH)

	// Build output line by line
	var sb strings.Builder
	for row := 0; row < bodyH; row++ {
		left := ""
		if row < len(listLines) {
			left = listLines[row]
		}
		right := ""
		if row < len(previewLines) {
			right = previewLines[row]
		}

		// Pad left to exact width
		left = padRight(left, listW)
		// Plain ASCII separator — Unicode box chars can be ambiguous width
		sep := "|"
		// Pad right to exact width
		right = padRight(right, previewW)

		sb.WriteString(left)
		sb.WriteString(sep)
		sb.WriteString(right)
		if row < bodyH-1 {
			sb.WriteByte('\n')
		}
	}

	sb.WriteByte('\n')
	sb.WriteString(padRight(m.renderHelp(), w))

	return sb.String()
}

func (m model) getListLines(w, h int) []string {
	if len(m.items) == 0 {
		lines := make([]string, h)
		lines[0] = helpStyle.Render("No sessions.")
		lines[1] = helpStyle.Render("Press 'n' to create.")
		return lines
	}

	// Scrolling
	start := 0
	end := len(m.items)
	if end > h {
		half := h / 2
		if m.cursor > half {
			start = m.cursor - half
		}
		end = start + h
		if end > len(m.items) {
			end = len(m.items)
			start = end - h
			if start < 0 {
				start = 0
			}
		}
	}

	var lines []string
	for i := start; i < end; i++ {
		item := m.items[i]
		selected := i == m.cursor

		if item.window != nil {
			prefix := "  "
			if selected {
				prefix = "> "
			}
			isLast := i+1 >= len(m.items) || m.items[i+1].window == nil
			tree := "├─"
			if isLast {
				tree = "└─"
			}
			label := fmt.Sprintf("%s  %s %d:%s", prefix, tree, item.window.Index, item.window.Name)
			if item.window.Active {
				label += " *"
			}
			label = runesTruncate(label, w)
			if selected {
				label = selectedStyle.Render(label)
			} else {
				label = windowStyle.Render(label)
			}
			lines = append(lines, label)
		} else if item.session != nil {
			s := item.session
			prefix := "  "
			if selected {
				prefix = "> "
			} else if s.Name == m.current {
				prefix = "→ "
			}

			icon := "▸"
			if m.expanded[s.Name] {
				icon = "▾"
			}

			lock := ""
			if tmux.IsProtected(s.Name) {
				lock = " *"
			}
			winLabel := fmt.Sprintf("%dw", s.Windows)
			label := fmt.Sprintf("%s%s %s%s %s", prefix, icon, s.Name, lock, winLabel)
			label = runesTruncate(label, w)

			if selected {
				label = selectedStyle.Render(label)
			} else if s.Attached {
				label = attachedStyle.Render(label)
			} else {
				label = detachedStyle.Render(label)
			}
			lines = append(lines, label)
		}
	}

	// Pad to full height
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines
}

func (m model) getPreviewLines(w, h int) []string {
	// Title line
	title := " Preview"
	if s := m.selectedSession(); s != nil {
		title = fmt.Sprintf(" %s", s.Name)
	}
	title = runesTruncate(title, w)
	titleLine := lipgloss.NewStyle().Bold(true).Foreground(colorBlue).Render(title)

	lines := make([]string, h)
	lines[0] = titleLine

	if m.preview == "" {
		lines[1] = helpStyle.Render(" No preview")
		return lines
	}

	clean := stripAnsi(m.preview)
	pLines := strings.Split(clean, "\n")

	// Show last (h-1) lines of preview (most recent output)
	maxLines := h - 1
	if len(pLines) > maxLines {
		pLines = pLines[len(pLines)-maxLines:]
	}

	for i, line := range pLines {
		lines[i+1] = runesTruncate(line, w)
	}

	return lines
}

func (m model) renderHelp() string {
	if m.mode == modeConfirmKill {
		return helpStyle.Render(" " + m.inputLabel)
	}
	if m.mode == modeConfirmKillProtected {
		errMsg := ""
		if m.err != "" {
			errMsg = "  " + errorStyle.Render(m.err)
		}
		return " " + inputStyle.Render(m.inputLabel) + m.input + "█  " + helpStyle.Render("esc:cancel enter:confirm") + errMsg
	}
	if m.mode != modeNormal {
		return " " + inputStyle.Render(m.inputLabel) + m.input + "█  " + helpStyle.Render("esc:cancel enter:confirm")
	}
	return helpStyle.Render(" j/k:nav  enter:attach  tab:expand  R:refresh  n:new  r:rename  c:cmd  x:kill  p:protect  q:quit")
}

// --- helpers ---

func stripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// runesTruncate truncates a plain string to maxW runes
func runesTruncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) > maxW {
		return string(runes[:maxW])
	}
	return s
}

// padRight pads a string with spaces to exact width.
// Strips ANSI to measure display width, then appends spaces.
func padRight(s string, w int) string {
	visible := lipgloss.Width(s)
	if visible >= w {
		// Need to truncate — strip ANSI and truncate by runes
		clean := stripAnsi(s)
		return runesTruncate(clean, w)
	}
	return s + strings.Repeat(" ", w-visible)
}

// Run starts the TUI and returns the session name to attach to (if any).
func Run() (string, error) {
	m := newModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}
	fm := finalModel.(model)
	return fm.attachTo, nil
}
