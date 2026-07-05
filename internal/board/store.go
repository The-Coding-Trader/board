// Package board is the data core: it models a three-column kanban board plus a
// free-form scratchpad, and reads/writes it as a human-readable Markdown file so
// another editor (for example a Claude Code session running in the same
// directory) can view and change the same board. The Markdown file is the single
// source of truth; the TUI keeps an in-memory copy and re-reads the file when it
// changes on disk.
package board

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Column identifies one of the three fixed columns. The zero value is Backlog.
type Column int

const (
	Backlog Column = iota
	InProgress
	Done

	// NumCols is the number of kanban columns.
	NumCols = 3
)

// ColumnNames are the exact Markdown "## " headers, in serialized order.
var ColumnNames = [NumCols]string{"Backlog", "In Progress", "Done"}

// scratchHeader is the fixed title of the free-form notes section, always
// serialized last so its body may safely contain "##" or "- [ ]" lines.
const scratchHeader = "Scratchpad"

// Section is an unrecognized "## X" block. board does not model its contents but
// preserves them verbatim so it never destroys text another editor wrote.
type Section struct {
	Title string
	Body  string
}

// Board is the whole document: three ordered card columns, the raw scratchpad
// text, any preamble before the first header, and any unknown sections. The last
// two exist purely to round-trip foreign edits without data loss.
type Board struct {
	Cols     [NumCols][]string
	Scratch  string
	Preamble string
	Extra    []Section
}

var (
	headerRe = regexp.MustCompile(`^## (.+)$`)
	cardRe   = regexp.MustCompile(`^\s*- \[([ xX])\] (.*)$`)
)

// Load reads and parses the board file. A missing file yields an empty Board
// (not an error) so the first run starts clean and creates the file on Save.
func Load(path string) (*Board, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Board{}, nil
		}
		return nil, err
	}
	return Parse(string(data)), nil
}

// Parse turns Markdown text into a Board. For canonically-serialized input it is
// the inverse of (*Board).String.
func Parse(text string) *Board {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")

	b := &Board{}
	var preamble []string
	seenHeader := false

	// The currently open section (meaningful only once seenHeader is true).
	type kind int
	const (
		kNone kind = iota
		kColumn
		kExtra
	)
	curKind := kNone
	var curCol Column
	var extraTitle string
	var extraBody []string

	flushExtra := func() {
		if curKind == kExtra {
			b.Extra = append(b.Extra, Section{
				Title: extraTitle,
				Body:  strings.Trim(strings.Join(extraBody, "\n"), "\n"),
			})
			extraBody = nil
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if m := headerRe.FindStringSubmatch(line); m != nil {
			flushExtra()
			seenHeader = true
			title := strings.TrimSpace(m[1])
			switch normTitle(title) {
			case "scratchpad":
				// Scratchpad is terminal: everything to EOF is raw scratch text.
				b.Scratch = strings.Trim(strings.Join(lines[i+1:], "\n"), "\n")
				b.Preamble = strings.Trim(strings.Join(preamble, "\n"), "\n")
				return b
			case "backlog":
				curKind, curCol = kColumn, Backlog
			case "in progress":
				curKind, curCol = kColumn, InProgress
			case "done":
				curKind, curCol = kColumn, Done
			default:
				curKind, extraTitle = kExtra, title
			}
			continue
		}

		if !seenHeader {
			preamble = append(preamble, line)
			continue
		}

		switch curKind {
		case kColumn:
			if m := cardRe.FindStringSubmatch(line); m != nil {
				if txt := strings.TrimSpace(m[2]); txt != "" {
					b.Cols[curCol] = append(b.Cols[curCol], txt)
				}
			}
			// Non-card lines inside a column are not modeled and are dropped.
		case kExtra:
			extraBody = append(extraBody, line)
		}
	}
	flushExtra()

	b.Preamble = strings.Trim(strings.Join(preamble, "\n"), "\n")
	return b
}

// String serializes the board to canonical Markdown. It is deterministic and
// idempotent — String(Parse(String(b))) == String(b) — so a no-op save produces
// byte-identical output and never trips the file-change poller.
func (b *Board) String() string {
	var sb strings.Builder

	if b.Preamble != "" {
		sb.WriteString(b.Preamble)
		sb.WriteString("\n\n")
	}

	for c := Column(0); c < NumCols; c++ {
		sb.WriteString("## ")
		sb.WriteString(ColumnNames[c])
		sb.WriteByte('\n')
		box := " "
		if c == Done {
			box = "x"
		}
		for _, card := range b.Cols[c] {
			sb.WriteString("- [")
			sb.WriteString(box)
			sb.WriteString("] ")
			sb.WriteString(card)
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n') // one blank line between sections
	}

	for _, s := range b.Extra {
		sb.WriteString("## ")
		sb.WriteString(s.Title)
		sb.WriteByte('\n')
		if s.Body != "" {
			sb.WriteString(s.Body)
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')
	}

	sb.WriteString("## ")
	sb.WriteString(scratchHeader)
	sb.WriteByte('\n')
	if b.Scratch != "" {
		sb.WriteString(b.Scratch)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// Save writes the board to path atomically: it writes a uniquely-named temp file
// in the same directory, fsyncs it, then renames it over the target. A concurrent
// reader (e.g. Claude Code) therefore sees either the whole old file or the whole
// new file, never a torn write.
func Save(path string, b *Board) error {
	data := []byte(b.String())
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".board-*.md.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil { // CreateTemp makes 0600
		return err
	}
	return os.Rename(tmpName, path)
}

// --- pure mutations (the model calls these, then updates its own selection) ---

// Add appends a card to a column and returns its index, or -1 if text is blank.
func (b *Board) Add(c Column, text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return -1
	}
	b.Cols[c] = append(b.Cols[c], text)
	return len(b.Cols[c]) - 1
}

// Set replaces the text of card i in column c; a blank replacement is ignored.
func (b *Board) Set(c Column, i int, text string) {
	text = strings.TrimSpace(text)
	if text == "" || !b.valid(c, i) {
		return
	}
	b.Cols[c][i] = text
}

// Delete removes card i from column c.
func (b *Board) Delete(c Column, i int) {
	if !b.valid(c, i) {
		return
	}
	b.Cols[c] = append(b.Cols[c][:i], b.Cols[c][i+1:]...)
}

// MoveCard moves card i from one column to another (appending to the target) and
// returns its new index in the target, or -1 if the source index is invalid.
func (b *Board) MoveCard(from Column, i int, to Column) int {
	if !b.valid(from, i) || from == to {
		return -1
	}
	card := b.Cols[from][i]
	b.Delete(from, i)
	b.Cols[to] = append(b.Cols[to], card)
	return len(b.Cols[to]) - 1
}

// Swap exchanges cards i and j within a column (used to reorder).
func (b *Board) Swap(c Column, i, j int) {
	if !b.valid(c, i) || !b.valid(c, j) {
		return
	}
	b.Cols[c][i], b.Cols[c][j] = b.Cols[c][j], b.Cols[c][i]
}

func (b *Board) valid(c Column, i int) bool {
	return c >= 0 && c < NumCols && i >= 0 && i < len(b.Cols[c])
}

// normTitle lowercases a header title and collapses internal whitespace so
// "In  Progress" and "in progress" both match the canonical column name.
func normTitle(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}
