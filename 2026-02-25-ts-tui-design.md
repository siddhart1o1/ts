# ts ui — TUI Mode Design

## Overview

Add a TUI mode to ts (`ts ui`) using Bubble Tea. Split-pane layout with session list on the left and live preview on the right. Laptop power feature — CLI commands stay for iPhone SSH.

## Entry Point

```
ts ui        → launch TUI session manager
```

## Layout

```
┌─ Sessions ──────────┬─ Preview (myapp) ─────────────────┐
│                     │                                    │
│ → myapp      3 wins │  $ cargo build                     │
│   ├─ 0:code         │     Compiling myapp v0.1.0         │
│   ├─ 1:test         │     Finished dev [debug] 2.3s     │
│   └─ 2:logs         │  $                                 │
│   deploy     1 win  │                                    │
│   scratch    1 win  │                                    │
│                     │                                    │
├─────────────────────┴────────────────────────────────────┤
│ ↑↓/jk nav  → expand  enter attach  n new  k kill  q quit│
└──────────────────────────────────────────────────────────┘
```

## Keybindings

| Key | Action |
|-----|--------|
| ↑/↓/j/k | Navigate session list |
| Enter | Attach to selected session (exits TUI) |
| Tab/→ | Expand session to show windows |
| ← | Collapse windows |
| n | Create new session (name prompt) |
| k/x | Kill selected session (confirmation) |
| r | Rename selected session (inline input) |
| c | Send command to selected session (input) |
| K | Kill all other sessions |
| q/Esc | Quit TUI |

## Architecture

```
main.go          → existing CLI, adds "ui" case to router
tui/
  tui.go         → Bubble Tea program, model, update, view
  sessions.go    → session data fetching + list rendering
  preview.go     → preview panel + capture-pane
  keys.go        → key bindings
  styles.go      → Lip Gloss styles (gruvbox-inspired)
```

## Preview

- Auto-refresh every 1 second via tea.Tick
- Uses tmux capture-pane -t <session> -p
- Immediate refresh on navigation change

## Window Expansion

- Tab/→ expands session to show windows via tmux list-windows
- Each window: index, name, active indicator
- Selecting a window previews that window's pane

## Styles (Gruvbox)

- Borders: #3c3836
- Selected: #d8a657
- Attached: #a9b665
- Detached: #928374
- Preview title: #7daea3

## Dependencies

- github.com/charmbracelet/bubbletea
- github.com/charmbracelet/lipgloss
- github.com/charmbracelet/bubbles

## Out of Scope

- No mouse support
- No multi-pane preview
- No session creation wizard
