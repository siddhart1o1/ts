package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

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
)

// message types
type sessionsMsg []tmux.Session

type windowsMsg struct {
	session string
	windows []tmux.Window
}

type previewMsg string
type tickMsg struct{}

// listItem represents a single item in the flattened list
type listItem struct {
	session   *tmux.Session
	window    *tmux.Window
	sessionID int
}

// model is the Bubble Tea model
type model struct {
	sessions []tmux.Session
	windows  map[string][]tmux.Window
	expanded map[string]bool
	items    []listItem
	cursor   int
	preview  string
	current  string // current tmux session name
	width    int
	height   int
	mode     int
	input    string
	inputLabel string
	err      string
	quitting bool
	attachTo string
}

func newModel() model {
	return model{
		windows:  make(map[string][]tmux.Window),
		expanded: make(map[string]bool),
		current:  tmux.GetCurrentSession(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(refreshSessions, tickCmd())
}

// --- data fetching ---

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

	case tickMsg:
		return m, tea.Batch(refreshSessions, tickCmd())

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
			// If on a window, move to its parent session
			if item.window != nil {
				name := m.sessions[item.sessionID].Name
				// Find the parent session item
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
			// If on a session, collapse it
			if item.session != nil {
				m.expanded[item.session.Name] = false
				m.items, m.cursor = rebuildItems(m.sessions, m.expanded, m.windows, m.cursor)
				return m, m.refreshPreview()
			}
		}

	case "x":
		s := m.selectedSession()
		if s != nil {
			m.mode = modeConfirmKill
			m.inputLabel = fmt.Sprintf("Kill session '%s'? (y/n)", s.Name)
			m.input = ""
			return m, nil
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
		m.inputLabel = "New session name: "
		m.input = ""
		return m, nil

	case "K":
		// Kill all other sessions
		current := m.current
		if current == "" {
			s := m.selectedSession()
			if s != nil {
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

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input = ""
		m.err = ""
		return m, nil

	case "enter":
		if m.mode == modeConfirmKill {
			// Require explicit y
			m.mode = modeNormal
			m.input = ""
			m.err = ""
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
			tmux.Cmd("rename-session", "-t", s.Name, m.input)
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

// --- View ---

func (m model) View() string {
	if m.quitting {
		return ""
	}

	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	helpHeight := 2
	availHeight := m.height - helpHeight

	// Split width: 30% list, 70% preview (list needs less space)
	listWidth := m.width * 3 / 10
	if listWidth < 30 {
		listWidth = 30
	}
	previewWidth := m.width - listWidth

	// Account for panel borders and padding (2 border + 2 padding = 4 per panel)
	innerListW := listWidth - 4
	innerPreviewW := previewWidth - 4
	innerH := availHeight - 2 // border top + bottom

	if innerListW < 10 {
		innerListW = 10
	}
	if innerPreviewW < 10 {
		innerPreviewW = 10
	}
	if innerH < 3 {
		innerH = 3
	}

	listContent := m.renderList(innerListW, innerH)
	previewContent := m.renderPreview(innerPreviewW, innerH)

	listPanel := listPanelStyle.
		Width(innerListW).
		Height(innerH).
		Render(listContent)

	previewPanel := previewPanelStyle.
		Width(innerPreviewW).
		Height(innerH).
		Render(previewContent)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, previewPanel)
	help := m.renderHelp()

	return lipgloss.JoinVertical(lipgloss.Left, panels, help)
}

func (m model) renderList(w, h int) string {
	if len(m.items) == 0 {
		return helpStyle.Render("No sessions. Press 'n' to create one.")
	}

	var lines []string

	// Calculate visible range for scrolling
	start := 0
	end := len(m.items)
	if end > h {
		// Scroll to keep cursor visible
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

	for i := start; i < end; i++ {
		item := m.items[i]
		selected := i == m.cursor

		if item.window != nil {
			// Window entry
			prefix := "  "
			if selected {
				prefix = "> "
			}
			// Check if last window
			isLast := i+1 >= len(m.items) || m.items[i+1].window == nil
			tree := "├─"
			if isLast {
				tree = "└─"
			}
			label := fmt.Sprintf("%s  %s %d: %s", prefix, tree, item.window.Index, item.window.Name)
			if item.window.Active {
				label += " *"
			}
			if selected {
				label = selectedStyle.Render(truncate(label, w))
			} else {
				label = windowStyle.Render(truncate(label, w))
			}
			lines = append(lines, label)
		} else if item.session != nil {
			// Session entry
			s := item.session
			marker := "  "
			if s.Name == m.current {
				marker = currentMarkerStyle.Render("→ ")
			}

			prefix := marker
			if selected {
				prefix = selectedStyle.Render("> ")
			}

			// Expand/collapse indicator
			expandIcon := "▸"
			if m.expanded[s.Name] {
				expandIcon = "▾"
			}

			nameStr := s.Name
			detail := fmt.Sprintf(" %s  %dw  %s", s.Dir, s.Windows, s.Command)

			var line string
			if selected {
				line = prefix + selectedStyle.Render(expandIcon+" "+nameStr) + selectedStyle.Render(truncate(detail, w-len(expandIcon)-len(nameStr)-4))
			} else if s.Attached {
				line = prefix + attachedStyle.Render(expandIcon+" "+nameStr) + detachedStyle.Render(truncate(detail, w-len(expandIcon)-len(nameStr)-4))
			} else {
				line = prefix + detachedStyle.Render(expandIcon+" "+nameStr) + detachedStyle.Render(truncate(detail, w-len(expandIcon)-len(nameStr)-4))
			}

			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

func (m model) renderPreview(w, h int) string {
	if m.preview == "" {
		return helpStyle.Render("No preview available")
	}

	// Strip all ANSI escape codes from captured pane content
	clean := stripAnsi(m.preview)
	lines := strings.Split(clean, "\n")

	// Truncate to fit height
	if len(lines) > h {
		lines = lines[len(lines)-h:]
	}

	// Truncate each line to fit width
	for i, line := range lines {
		runes := []rune(line)
		if len(runes) > w {
			lines[i] = string(runes[:w])
		}
	}

	return strings.Join(lines, "\n")
}

func (m model) renderHelp() string {
	if m.mode != modeNormal {
		label := inputStyle.Render(m.inputLabel)
		cursor := inputStyle.Render(m.input + "█")
		if m.err != "" {
			return label + cursor + "  " + errorStyle.Render(m.err)
		}
		return label + cursor
	}

	return helpStyle.Render("  j/k: navigate  enter: attach  tab: expand  n: new  r: rename  c: cmd  x: kill  q: quit")
}

func stripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func truncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w <= maxW {
		return s
	}
	// Strip ANSI first, then truncate by runes
	clean := stripAnsi(s)
	runes := []rune(clean)
	if len(runes) > maxW {
		if maxW > 3 {
			return string(runes[:maxW-3]) + "..."
		}
		return string(runes[:maxW])
	}
	return clean
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
