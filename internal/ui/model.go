// Package ui is the Bubble Tea layer: a three-column kanban board with a
// scratchpad, backed by a Markdown file on disk. It owns selection/focus state,
// mutates the in-memory board, saves on every change, and polls the file once a
// second so edits made by another process (e.g. a Claude Code session in the
// same directory) show up live.
package ui

import (
	"os"
	"reflect"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/joey/board/internal/board"
	"github.com/joey/board/internal/theme"
)

// mode is the current input context: navigating the board, typing a new/edited
// card, or editing the scratchpad.
type mode int

const (
	modeBoard mode = iota
	modeAdd
	modeEdit
	modeScratch
)

// Model holds all TUI state.
type Model struct {
	board *board.Board
	path  string

	// File-change poll bookkeeping: the stat of the last version we wrote or read.
	lastMod  time.Time
	lastSize int64

	mode     mode
	focused  board.Column
	selected [board.NumCols]int

	input   textinput.Model // quick-add / edit card
	scratch textarea.Model  // scratchpad notes

	scratchDirty    bool // scratch edited since last save
	scratchConflict bool // disk scratch diverged while we were editing

	w, h   int
	status string // transient one-line message
}

// New builds a model around an already-loaded board and its file stat.
func New(b *board.Board, path string, mod time.Time, size int64) Model {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.Placeholder = "card text…"

	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.Placeholder = "jot notes here — saved with the board"
	ta.SetValue(b.Scratch)

	// Flatten the textarea's default styling: the stock focused theme paints a
	// background on the current line (the gray box while typing) — clear it so
	// the scratchpad sits on the terminal background like everything else. Give
	// the text a readable color and a yellow cursor to match the pane.
	st := ta.Styles()
	st.Focused.CursorLine = lipgloss.NewStyle()
	st.Focused.Text = lipgloss.NewStyle().Foreground(theme.Bone)
	st.Focused.Placeholder = lipgloss.NewStyle().Foreground(theme.Muted)
	st.Blurred.Text = lipgloss.NewStyle().Foreground(theme.Fog)
	st.Blurred.Placeholder = lipgloss.NewStyle().Foreground(theme.Muted)
	st.Cursor.Color = theme.ScratchHue
	ta.SetStyles(st)

	return Model{
		board:    b,
		path:     path,
		lastMod:  mod,
		lastSize: size,
		mode:     modeBoard,
		focused:  board.Backlog,
		input:    ti,
		scratch:  ta,
	}
}

// --- messages / commands ---

// pollMsg means "stat showed no relevant change"; boardLoadedMsg carries a fresh
// board read from disk. Both re-arm the poll loop.
type pollMsg struct{}

type boardLoadedMsg struct {
	board *board.Board
	mod   time.Time
	size  int64
}

// manualReloadMsg is the result of the `r` key: like boardLoadedMsg but it does
// not re-arm the poll loop, so pressing `r` never spawns a second poller.
type manualReloadMsg struct {
	board *board.Board
	mod   time.Time
	size  int64
}

// reloadNowCmd reads the file immediately (not on the 1s tick) for a manual `r`.
func reloadNowCmd(path string) tea.Cmd {
	return func() tea.Msg {
		b, err := board.Load(path)
		if err != nil {
			return manualReloadMsg{} // nil board → handled as a no-op
		}
		var mod time.Time
		var size int64
		if fi, err := os.Stat(path); err == nil {
			mod, size = fi.ModTime(), fi.Size()
		}
		return manualReloadMsg{board: b, mod: mod, size: size}
	}
}

// pollCmd waits one second, then stats the file. If the mtime/size differ from
// what we last wrote, it re-reads and parses the file off the UI goroutine and
// returns a boardLoadedMsg; otherwise a plain pollMsg. The lastMod/lastSize are
// captured at arm time so our own writes don't look like foreign changes.
func pollCmd(path string, lastMod time.Time, lastSize int64) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		fi, err := os.Stat(path)
		if err != nil {
			return pollMsg{}
		}
		if fi.ModTime().Equal(lastMod) && fi.Size() == lastSize {
			return pollMsg{}
		}
		b, err := board.Load(path)
		if err != nil {
			return pollMsg{}
		}
		return boardLoadedMsg{board: b, mod: fi.ModTime(), size: fi.Size()}
	})
}

func (m Model) Init() tea.Cmd {
	return pollCmd(m.path, m.lastMod, m.lastSize)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.layout()
		return m, nil

	case pollMsg:
		return m, pollCmd(m.path, m.lastMod, m.lastSize)

	case boardLoadedMsg:
		if m.applyLoaded(msg) {
			m.status = "↻ reloaded external change"
		}
		return m, pollCmd(m.path, m.lastMod, m.lastSize)

	case manualReloadMsg:
		if msg.board != nil {
			if m.applyLoaded(boardLoadedMsg(msg)) {
				m.status = "↻ reloaded"
			} else {
				m.status = "already up to date"
			}
		}
		return m, nil // poll loop continues on its own; don't re-arm here

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	// Forward everything else (cursor blink, paste, …) to the active widget.
	var cmd tea.Cmd
	switch m.mode {
	case modeAdd, modeEdit:
		m.input, cmd = m.input.Update(msg)
	case modeScratch:
		m.scratch, cmd = m.scratch.Update(msg)
	}
	return m, cmd
}

func (m Model) View() tea.View {
	v := tea.NewView(m.frame())
	v.AltScreen = true
	v.WindowTitle = "board"
	return v
}

// --- helpers ---

// applyLoaded merges a disk read into the model. Cards always adopt the disk
// version (they're the shared, low-conflict data). The scratchpad is only
// overwritten when the user isn't editing it — otherwise we keep the in-memory
// text and just flag a conflict, so live typing is never clobbered. Returns
// whether anything visible changed.
func (m *Model) applyLoaded(msg boardLoadedMsg) bool {
	changed := false
	if !reflect.DeepEqual(m.board.Cols, msg.board.Cols) {
		m.board.Cols = msg.board.Cols
		changed = true
	}
	m.board.Preamble = msg.board.Preamble
	m.board.Extra = msg.board.Extra

	if m.mode != modeScratch && !m.scratchDirty {
		if msg.board.Scratch != m.board.Scratch {
			m.board.Scratch = msg.board.Scratch
			m.scratch.SetValue(msg.board.Scratch)
			changed = true
		}
		m.scratchConflict = false
	} else if msg.board.Scratch != m.board.Scratch {
		m.scratchConflict = true
	}

	m.lastMod = msg.mod
	m.lastSize = msg.size
	m.clampSelection()
	return changed
}

// save writes the board and records the resulting stat so the poller doesn't
// mistake our own write for a foreign one.
func (m *Model) save() {
	if err := board.Save(m.path, m.board); err != nil {
		m.status = "save error: " + err.Error()
		return
	}
	if fi, err := os.Stat(m.path); err == nil {
		m.lastMod = fi.ModTime()
		m.lastSize = fi.Size()
	}
}

// clampSelection keeps every column's selection index within bounds after the
// card lists change.
func (m *Model) clampSelection() {
	for c := 0; c < board.NumCols; c++ {
		n := len(m.board.Cols[c])
		if m.selected[c] >= n {
			m.selected[c] = max(0, n-1)
		}
		if m.selected[c] < 0 {
			m.selected[c] = 0
		}
	}
}

// layout resizes the child widgets to the current terminal width.
func (m *Model) layout() {
	if m.w <= 0 {
		return
	}
	m.input.SetWidth(max(10, m.w-8))
	m.scratch.SetWidth(max(10, m.w-4))
	m.scratch.SetHeight(m.scratchHeight())
}

// scratchHeight is how many text rows the scratchpad gets, scaled to the window.
func (m Model) scratchHeight() int {
	h := m.h / 5
	if h < 3 {
		h = 3
	}
	if h > 8 {
		h = 8
	}
	return h
}
