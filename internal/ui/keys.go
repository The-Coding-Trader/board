package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/The-Coding-Trader/board/internal/board"
)

// handleKey routes a key press based on the current mode. In the input modes the
// key either commits/cancels or is forwarded to the child widget; in board mode
// it drives navigation and card mutations, saving after anything that changes
// the board.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := tea.Key(msg).String()

	switch m.mode {
	case modeAdd, modeEdit:
		return m.handleInputKey(key, msg)
	case modeScratch:
		return m.handleScratchKey(key, msg)
	default:
		return m.handleBoardKey(key)
	}
}

func (m Model) handleInputKey(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		text := m.input.Value()
		if m.mode == modeAdd {
			if idx := m.board.Add(m.focused, text); idx >= 0 {
				m.selected[m.focused] = idx
			}
		} else {
			m.board.Set(m.focused, m.selected[m.focused], text)
		}
		m.input.Reset()
		m.input.Blur()
		m.mode = modeBoard
		m.save()
		return m, nil
	case "esc":
		m.input.Reset()
		m.input.Blur()
		m.mode = modeBoard
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) handleScratchKey(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "tab":
		m.board.Scratch = m.scratch.Value()
		m.scratch.Blur()
		m.scratchDirty = false
		m.scratchConflict = false
		m.mode = modeBoard
		m.save()
		return m, nil
	case "ctrl+p":
		m.promoteScratchLine()
		return m, nil
	}
	var cmd tea.Cmd
	m.scratch, cmd = m.scratch.Update(msg)
	m.scratchDirty = true
	return m, cmd
}

// promoteScratchLine turns the scratchpad line under the cursor into a new
// Backlog card and removes it from the notes — turning a jotted thought into a
// tracked task with one keystroke. It saves immediately and keeps you in the
// scratchpad so you can promote several lines in a row.
func (m *Model) promoteScratchLine() {
	lines := strings.Split(m.scratch.Value(), "\n")
	row := m.scratch.Line()
	if row < 0 || row >= len(lines) {
		m.status = "nothing to promote"
		return
	}
	text := strings.TrimSpace(lines[row])
	if text == "" {
		m.status = "nothing to promote — empty line"
		return
	}

	if idx := m.board.Add(board.Backlog, text); idx >= 0 {
		m.selected[board.Backlog] = idx // preselect it for when you tab back
	}

	// Drop the promoted line and put the cursor back where it was.
	lines = append(lines[:row], lines[row+1:]...)
	m.scratch.SetValue(strings.Join(lines, "\n"))
	m.board.Scratch = m.scratch.Value()
	target := min(row, m.scratch.LineCount()-1)
	m.scratch.MoveToBegin()
	for i := 0; i < target; i++ {
		m.scratch.CursorDown()
	}

	m.scratchDirty = false
	m.save()
	m.status = "promoted to Backlog: " + text
}

func (m Model) handleBoardKey(key string) (tea.Model, tea.Cmd) {
	m.status = "" // any board key clears a transient message
	col := m.focused
	n := len(m.board.Cols[col])

	switch key {
	case "q", "ctrl+c":
		m.save()
		return m, tea.Quit

	// --- navigation ---
	case "left", "h":
		if col > 0 {
			m.focused--
		}
	case "right", "l":
		if col < board.NumCols-1 {
			m.focused++
		}
	case "up", "k":
		if m.selected[col] > 0 {
			m.selected[col]--
		}
	case "down", "j":
		if m.selected[col] < n-1 {
			m.selected[col]++
		}
	case "g", "home":
		m.selected[col] = 0
	case "G", "shift+g", "end":
		m.selected[col] = max(0, n-1)

	// --- add / edit ---
	case "a", "n":
		m.mode = modeAdd
		m.input.Reset()
		return m, m.input.Focus()
	case "e", "enter":
		if n > 0 {
			m.mode = modeEdit
			m.input.SetValue(m.board.Cols[col][m.selected[col]])
			m.input.CursorEnd()
			return m, m.input.Focus()
		}

	// --- move card across columns ---
	case "H", "shift+h", "<":
		m.moveCard(col, col-1)
	case "L", "shift+l", ">":
		m.moveCard(col, col+1)
	case " ", "space": // send to Done (or back to Backlog)
		if n > 0 {
			target := board.Done
			if col == board.Done {
				target = board.Backlog
			}
			m.moveCard(col, target)
		}

	// --- reorder within column ---
	case "K", "shift+k":
		if i := m.selected[col]; i > 0 {
			m.board.Swap(col, i, i-1)
			m.selected[col] = i - 1
			m.save()
		}
	case "J", "shift+j":
		if i := m.selected[col]; i < n-1 {
			m.board.Swap(col, i, i+1)
			m.selected[col] = i + 1
			m.save()
		}

	// --- delete ---
	case "d", "x", "delete", "backspace":
		if n > 0 {
			m.board.Delete(col, m.selected[col])
			m.clampSelection()
			m.save()
		}

	// --- scratchpad / reload ---
	case "tab", "s":
		m.mode = modeScratch
		m.scratchConflict = false
		return m, m.scratch.Focus()
	case "r":
		return m, reloadNowCmd(m.path) // immediate manual reload
	}

	return m, nil
}

// moveCard moves the selected card from col to target (if in range), following
// the card to its new column and saving.
func (m *Model) moveCard(col, target board.Column) {
	if target < 0 || target >= board.NumCols || target == col {
		return
	}
	if len(m.board.Cols[col]) == 0 {
		return
	}
	ni := m.board.MoveCard(col, m.selected[col], target)
	if ni < 0 {
		return
	}
	m.clampSelection()
	m.focused = target
	m.selected[target] = ni
	m.save()
}
