package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/joey/board/internal/board"
	"github.com/joey/board/internal/theme"
)

var (
	keyStyle    = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(theme.Muted)
	warnStyle   = lipgloss.NewStyle().Foreground(theme.Ink).Background(theme.Danger).Bold(true)
	accentStyle = lipgloss.NewStyle().Foreground(theme.Ink).Background(theme.Accent).Bold(true)
)

// columnHue is each column's status color: blue queued → orange active → green done.
var columnHue = [board.NumCols]color.Color{
	board.Backlog:    theme.BacklogHue,
	board.InProgress: theme.InProgressHue,
	board.Done:       theme.DoneHue,
}

// columnSoftHue is the lighter tint used to highlight the selected card.
var columnSoftHue = [board.NumCols]color.Color{
	board.Backlog:    theme.BacklogSoft,
	board.InProgress: theme.InProgressSoft,
	board.Done:       theme.DoneSoft,
}

func hintKey(s string) string { return keyStyle.Render(s) }
func dim(s string) string     { return dimStyle.Render(s) }

// statusBorder is a box's border color: its own hue when focused, else a dim
// neutral so only the focused pane lights up.
func statusBorder(hue color.Color, focused bool) color.Color {
	if focused {
		return hue
	}
	return theme.Slate
}

// frame assembles the whole screen: title bar, three columns, scratch pane, and
// the status/help bar, sized to fill the terminal exactly.
func (m Model) frame() string {
	if m.w == 0 || m.h == 0 {
		return "starting board…"
	}

	scratchRows := m.scratchHeight() + 3 // label line + rounded border (2)
	colsRows := m.h - 1 /*title*/ - 1 /*status*/ - scratchRows
	if colsRows < 6 {
		colsRows = 6
	}
	boxContentH := colsRows - 2 // rounded border

	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderTitle(),
		m.renderColumns(boxContentH),
		m.renderScratch(),
		m.renderStatus(),
	)
}

func (m Model) renderTitle() string {
	badge := accentStyle.Padding(0, 1).Render("◆ board")
	path := dimStyle.Render("  " + m.path)
	total := 0
	for c := 0; c < board.NumCols; c++ {
		total += len(m.board.Cols[c])
	}
	right := dimStyle.Render(fmt.Sprintf("%d cards ", total))
	return padLR(m.w, badge+path, right)
}

func (m Model) renderColumns(contentH int) string {
	inner := (m.w - 6) / 3 // three boxes, each with a 2-col border
	if inner < 8 {
		inner = 8
	}
	boxes := make([]string, board.NumCols)
	for c := 0; c < board.NumCols; c++ {
		boxes[c] = m.renderColumn(board.Column(c), inner, contentH)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, boxes...)
}

func (m Model) renderColumn(c board.Column, inner, contentH int) string {
	focused := m.focused == c && m.mode != modeScratch
	lines := []string{m.renderHeader(c, inner, focused)}

	cards := m.board.Cols[c]
	avail := contentH - 1 // the header takes one line
	if avail < 1 {
		avail = 1
	}
	if len(cards) <= avail {
		for i := range cards {
			lines = append(lines, m.renderCard(c, i, inner, focused))
		}
	} else {
		vis := avail - 1 // reserve a line for the overflow marker
		if vis < 1 {
			vis = 1
		}
		start := scrollStart(len(cards), m.selected[c], vis)
		for i := start; i < start+vis; i++ {
			lines = append(lines, m.renderCard(c, i, inner, focused))
		}
		lines = append(lines, dimStyle.Width(inner).Render(fmt.Sprintf("  … %d more", len(cards)-vis)))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(statusBorder(columnHue[c], focused)).
		Width(inner).
		Height(contentH).
		Render(strings.Join(lines, "\n"))
}

func (m Model) renderHeader(c board.Column, inner int, focused bool) string {
	hue := columnHue[c]
	label := fmt.Sprintf(" %s  %d", strings.ToUpper(board.ColumnNames[c]), len(m.board.Cols[c]))
	st := lipgloss.NewStyle().Width(inner).Bold(true)
	if focused {
		st = st.Foreground(theme.Ink).Background(hue) // filled band shows focus
	} else {
		st = st.Foreground(hue) // colored text keeps identity when idle
	}
	return st.Render(truncate(label, inner))
}

func (m Model) renderCard(c board.Column, i, inner int, colFocused bool) string {
	hue := columnHue[c]
	text := m.board.Cols[c][i]

	// Selected card: a gentle wash in a lighter tint of the column's hue.
	if colFocused && m.selected[c] == i {
		marker := "• "
		if c == board.Done {
			marker = "✓ "
		}
		line := truncate(marker+text, inner-1)
		return lipgloss.NewStyle().Width(inner).
			Background(columnSoftHue[c]).Foreground(theme.Ink).Bold(true).
			Render(" " + line)
	}

	// Done cards: green marker + text, reinforcing completion.
	if c == board.Done {
		line := truncate("✓ "+text, inner-1)
		return lipgloss.NewStyle().Width(inner).Foreground(theme.DoneHue).Render(" " + line)
	}

	// Other cards: a hue-tinted bullet with neutral text for readability.
	bullet := lipgloss.NewStyle().Foreground(hue).Render("• ")
	body := lipgloss.NewStyle().Foreground(theme.Bone).Render(truncate(text, inner-3))
	return lipgloss.NewStyle().Width(inner).Render(" " + bullet + body)
}

func (m Model) renderScratch() string {
	focused := m.mode == modeScratch
	hue := theme.ScratchHue

	labelStyle := lipgloss.NewStyle().Bold(true)
	if focused {
		labelStyle = labelStyle.Foreground(theme.Ink).Background(hue) // filled yellow band
	} else {
		labelStyle = labelStyle.Foreground(hue) // yellow text keeps its identity
	}
	hint := dim("  tab to edit")
	if focused {
		hint = dim("  esc/tab to save & return")
	}
	head := labelStyle.Render(" SCRATCHPAD ") + hint

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(statusBorder(hue, focused)).
		Width(m.w - 2).
		Render(head + "\n" + m.scratch.View())
}

func (m Model) renderStatus() string {
	switch m.mode {
	case modeAdd:
		return m.promptLabel(" add → "+board.ColumnNames[m.focused]+" ") + " " + m.input.View()
	case modeEdit:
		return m.promptLabel(" edit ") + " " + m.input.View()
	}

	var right string
	switch {
	case m.scratchConflict:
		right = warnStyle.Render(" scratch changed on disk — r to reload ")
	case m.status != "":
		right = accentStyle.Render(" " + m.status + " ")
	}

	if m.mode == modeScratch {
		left := hintKey("esc/tab") + dim(":save  ") +
			hintKey("^p") + dim(":line→card  ") +
			hintKey("type") + dim(":edit notes")
		return padLR(m.w, left, right)
	}

	left := strings.Join([]string{
		hintKey("a") + dim(":add"),
		hintKey("e") + dim(":edit"),
		hintKey("h/l") + dim(":col"),
		hintKey("j/k") + dim(":card"),
		hintKey("H/L") + dim(":move"),
		hintKey("spc") + dim(":done"),
		hintKey("d") + dim(":del"),
		hintKey("tab") + dim(":notes"),
		hintKey("q") + dim(":quit"),
	}, " ")
	return padLR(m.w, left, right)
}

// promptLabel renders a filled band in the focused column's hue, so the
// add/edit prompt is visually tied to the column it targets.
func (m Model) promptLabel(s string) string {
	return lipgloss.NewStyle().
		Foreground(theme.Ink).Background(columnHue[m.focused]).Bold(true).
		Render(s)
}

// --- small helpers ---

// padLR places left and right on one line separated by filler, capped to width.
func padLR(w int, left, right string) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return lipgloss.NewStyle().MaxWidth(w).Render(line)
}

// truncate shortens s to at most w display columns, adding an ellipsis. It is
// rune-based, which is exact for the common ASCII case.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}

// scrollStart returns the first visible index so that sel stays in a window of
// vis rows.
func scrollStart(n, sel, vis int) int {
	if n <= vis {
		return 0
	}
	start := sel - vis/2
	if start < 0 {
		start = 0
	}
	if start > n-vis {
		start = n - vis
	}
	return start
}
