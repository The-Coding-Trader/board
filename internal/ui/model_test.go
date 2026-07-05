package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/joey/board/internal/board"
)

// --- test helpers ---

func newTestModel(t *testing.T) (Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".board.md")
	b, err := board.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	m := New(b, path, time.Time{}, 0)
	m.w, m.h = 90, 30
	m.layout()
	return m, path
}

func send(m Model, msg tea.Msg) Model {
	nm, _ := m.Update(msg)
	return nm.(Model)
}

func runeKey(r rune) tea.KeyPressMsg { return tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}) }
func special(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}
func ctrlKey(r rune) tea.KeyPressMsg { return tea.KeyPressMsg(tea.Key{Code: r, Mod: tea.ModCtrl}) }

// typeScratch types multi-line text into the scratchpad (already focused),
// using Enter between lines.
func typeScratch(m Model, lines ...string) Model {
	for i, ln := range lines {
		if i > 0 {
			m = send(m, special(tea.KeyEnter))
		}
		for _, r := range ln {
			m = send(m, runeKey(r))
		}
	}
	return m
}

func addCard(m Model, text string) Model {
	m = send(m, runeKey('a'))
	for _, r := range text {
		m = send(m, runeKey(r))
	}
	return send(m, special(tea.KeyEnter))
}

// --- tests ---

func TestAddCardFlow(t *testing.T) {
	m, path := newTestModel(t)

	m = send(m, runeKey('a'))
	if m.mode != modeAdd {
		t.Fatalf("expected modeAdd, got %d", m.mode)
	}
	m = addCardChars(m, "buy milk")
	m = send(m, special(tea.KeyEnter))

	if got := m.board.Cols[board.Backlog]; len(got) != 1 || got[0] != "buy milk" {
		t.Fatalf("backlog = %#v", got)
	}
	if m.mode != modeBoard {
		t.Fatalf("should return to board mode, got %d", m.mode)
	}
	// The mutation was persisted to disk (the Claude-interop write path).
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "- [ ] buy milk") {
		t.Fatalf("card not saved to disk:\n%s", data)
	}
}

func addCardChars(m Model, text string) Model {
	for _, r := range text {
		m = send(m, runeKey(r))
	}
	return m
}

func TestMoveCardRightAndFollowFocus(t *testing.T) {
	m, _ := newTestModel(t)
	m = addCard(m, "task")

	m = send(m, runeKey('L')) // "L" -> move right to In Progress
	if len(m.board.Cols[board.Backlog]) != 0 || len(m.board.Cols[board.InProgress]) != 1 {
		t.Fatalf("move-right failed: %#v", m.board.Cols)
	}
	if m.focused != board.InProgress {
		t.Fatalf("focus should follow the card, got %d", m.focused)
	}

	// The "shift+h" representation must also move left (terminals send either).
	m = send(m, tea.KeyPressMsg(tea.Key{Code: 'h', Mod: tea.ModShift}))
	if len(m.board.Cols[board.Backlog]) != 1 {
		t.Fatalf("shift+h move-left failed: %#v", m.board.Cols)
	}
	if m.focused != board.Backlog {
		t.Fatalf("focus should follow back to backlog, got %d", m.focused)
	}
}

func TestSpaceSendsToDoneAndBack(t *testing.T) {
	m, _ := newTestModel(t)
	m = addCard(m, "ship it")

	m = send(m, special(tea.KeySpace)) // send to Done
	if len(m.board.Cols[board.Done]) != 1 || m.focused != board.Done {
		t.Fatalf("space->done failed: cols=%#v focus=%d", m.board.Cols, m.focused)
	}
	m = send(m, special(tea.KeySpace)) // from Done, back to Backlog
	if len(m.board.Cols[board.Backlog]) != 1 || m.focused != board.Backlog {
		t.Fatalf("space back-to-backlog failed: %#v", m.board.Cols)
	}
}

func TestDelete(t *testing.T) {
	m, _ := newTestModel(t)
	m = addCard(m, "a")
	m = addCard(m, "b")
	if len(m.board.Cols[board.Backlog]) != 2 {
		t.Fatalf("setup: %#v", m.board.Cols[board.Backlog])
	}
	m = send(m, runeKey('d'))
	if len(m.board.Cols[board.Backlog]) != 1 {
		t.Fatalf("delete failed: %#v", m.board.Cols[board.Backlog])
	}
}

func TestReorderWithinColumn(t *testing.T) {
	m, _ := newTestModel(t)
	m = addCard(m, "first")
	m = addCard(m, "second")  // selection is on "second" (index 1)
	m = send(m, runeKey('K')) // move up
	if m.board.Cols[board.Backlog][0] != "second" || m.selected[board.Backlog] != 0 {
		t.Fatalf("reorder up failed: %#v sel=%d", m.board.Cols[board.Backlog], m.selected[board.Backlog])
	}
}

func TestColumnNavigationClamps(t *testing.T) {
	m, _ := newTestModel(t)
	m = send(m, runeKey('l'))
	m = send(m, runeKey('l'))
	m = send(m, runeKey('l')) // past the end
	if m.focused != board.Done {
		t.Fatalf("focus should clamp at Done, got %d", m.focused)
	}
	m = send(m, runeKey('h'))
	if m.focused != board.InProgress {
		t.Fatalf("h should move focus left, got %d", m.focused)
	}
}

func TestEditPreloadsAndKeepsText(t *testing.T) {
	m, _ := newTestModel(t)
	m = addCard(m, "original")

	m = send(m, runeKey('e'))
	if m.mode != modeEdit {
		t.Fatalf("e should enter edit mode, got %d", m.mode)
	}
	if m.input.Value() != "original" {
		t.Fatalf("edit should preload text, got %q", m.input.Value())
	}
	m = send(m, special(tea.KeyEnter)) // commit unchanged
	if m.board.Cols[board.Backlog][0] != "original" {
		t.Fatalf("edit commit changed text: %#v", m.board.Cols[board.Backlog])
	}
}

func TestScratchGuardKeepsInMemoryWhileEditing(t *testing.T) {
	m, _ := newTestModel(t)
	m.board.Scratch = "my live edits"
	m.mode = modeScratch
	m.scratchDirty = true

	changed := m.applyLoaded(boardLoadedMsg{
		board: &board.Board{Scratch: "someone else's version"},
		mod:   time.Now(),
		size:  10,
	})
	if m.board.Scratch != "my live edits" {
		t.Fatalf("scratch was clobbered mid-edit: %q", m.board.Scratch)
	}
	if !m.scratchConflict {
		t.Fatalf("expected scratchConflict to be flagged")
	}
	_ = changed
}

func TestReloadAdoptsCardsAndClampsSelection(t *testing.T) {
	m, _ := newTestModel(t)
	m.board.Add(board.Backlog, "a")
	m.board.Add(board.Backlog, "b")
	m.selected[board.Backlog] = 1

	// External edit empties the backlog and sets scratch (we're not editing it).
	m.applyLoaded(boardLoadedMsg{
		board: &board.Board{Scratch: "fresh notes"},
		mod:   time.Time{},
		size:  0,
	})
	if len(m.board.Cols[board.Backlog]) != 0 {
		t.Fatalf("cards not reloaded: %#v", m.board.Cols[board.Backlog])
	}
	if m.selected[board.Backlog] != 0 {
		t.Fatalf("selection not clamped: %d", m.selected[board.Backlog])
	}
	if m.board.Scratch != "fresh notes" {
		t.Fatalf("scratch not adopted when idle: %q", m.board.Scratch)
	}
}

// TestDumpFrame prints a rendered frame when BOARD_DUMP is set, for eyeballing
// the layout. It's a no-op in the normal suite.
func TestDumpFrame(t *testing.T) {
	if os.Getenv("BOARD_DUMP") == "" {
		t.Skip("set BOARD_DUMP=1 to print a frame")
	}
	m, _ := newTestModel(t)
	m.w, m.h = 96, 26
	m.layout()
	m.board.Cols[board.Backlog] = []string{"wire up scratch pane", "add color theme", "handle [auth] bug"}
	m.board.Cols[board.InProgress] = []string{"column rendering"}
	m.board.Cols[board.Done] = []string{"project scaffold", "markdown store + tests"}
	m.board.Scratch = "remember: atomic writes so Claude never sees a torn file\npoll mtime once a second"
	m.scratch.SetValue(m.board.Scratch)
	m.selected[board.Backlog] = 1
	fmt.Println(m.frame())
}

func TestPromoteScratchLineToCard(t *testing.T) {
	m, path := newTestModel(t)
	m = send(m, special(tea.KeyTab)) // into scratch
	m = typeScratch(m, "line one", "line two", "line three")

	// Cursor is at the end (line three); move up to "line two".
	m = send(m, special(tea.KeyUp))
	m = send(m, ctrlKey('p')) // promote

	if got := m.board.Cols[board.Backlog]; len(got) != 1 || got[0] != "line two" {
		t.Fatalf("promoted card wrong: %#v", got)
	}
	if m.scratch.Value() != "line one\nline three" {
		t.Fatalf("scratch after promote = %q", m.scratch.Value())
	}
	if m.mode != modeScratch {
		t.Fatalf("should stay in scratch mode, got %d", m.mode)
	}
	if m.selected[board.Backlog] != 0 {
		t.Fatalf("promoted card should be preselected, got %d", m.selected[board.Backlog])
	}
	// Persisted to disk: card present, promoted line gone from scratch.
	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, "- [ ] line two") {
		t.Fatalf("promoted card not saved:\n%s", s)
	}
	if strings.Contains(s, "line two\n") && strings.Contains(s, "## Scratchpad\nline one\nline three") == false {
		// scratch section should be exactly the two remaining lines
		t.Fatalf("scratch not rewritten correctly:\n%s", s)
	}
}

func TestPromoteBlankLineIsNoop(t *testing.T) {
	m, _ := newTestModel(t)
	m = send(m, special(tea.KeyTab))
	m = typeScratch(m, "   ") // whitespace-only line
	m = send(m, ctrlKey('p'))
	if len(m.board.Cols[board.Backlog]) != 0 {
		t.Fatalf("blank line should not promote: %#v", m.board.Cols[board.Backlog])
	}
}

func TestFrameRendersWithoutPanic(t *testing.T) {
	m, _ := newTestModel(t)
	m = addCard(m, "render me")
	m = send(m, runeKey('l')) // focus a different column
	out := m.frame()
	for _, want := range []string{"BACKLOG", "IN PROGRESS", "DONE", "SCRATCHPAD", "render me"} {
		if !strings.Contains(out, want) {
			t.Fatalf("frame missing %q; got:\n%s", want, out)
		}
	}
	// Also exercise the very small terminal path (should not panic).
	m.w, m.h = 20, 8
	m.layout()
	_ = m.frame()
}

func TestScratchModeRoundTrip(t *testing.T) {
	m, _ := newTestModel(t)
	m = send(m, special(tea.KeyTab)) // enter scratch
	if m.mode != modeScratch {
		t.Fatalf("tab should enter scratch, got %d", m.mode)
	}
	m = addCardChars(m, "note")      // types into the textarea
	m = send(m, special(tea.KeyTab)) // save & return
	if m.mode != modeBoard {
		t.Fatalf("tab should return to board, got %d", m.mode)
	}
	if !strings.Contains(m.board.Scratch, "note") {
		t.Fatalf("scratch not committed: %q", m.board.Scratch)
	}
	if m.scratchDirty {
		t.Fatalf("scratchDirty should be cleared after save")
	}
}
