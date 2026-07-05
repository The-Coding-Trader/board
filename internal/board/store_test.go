package board

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRoundTripPopulated(t *testing.T) {
	src := "## Backlog\n" +
		"- [ ] wire up scratch pane\n" +
		"- [ ] fix [auth] bug\n" +
		"\n" +
		"## In Progress\n" +
		"- [ ] column render\n" +
		"\n" +
		"## Done\n" +
		"- [x] project scaffold\n" +
		"\n" +
		"## Scratchpad\n" +
		"a note\nanother note\n"

	b := Parse(src)

	want := [NumCols][]string{
		{"wire up scratch pane", "fix [auth] bug"},
		{"column render"},
		{"project scaffold"},
	}
	if !reflect.DeepEqual(b.Cols, want) {
		t.Fatalf("cols = %#v, want %#v", b.Cols, want)
	}
	if b.Scratch != "a note\nanother note" {
		t.Fatalf("scratch = %q", b.Scratch)
	}
	if got := b.String(); got != src {
		t.Fatalf("round-trip mismatch:\n got: %q\nwant: %q", got, src)
	}
}

func TestEmptyBoard(t *testing.T) {
	b := &Board{}
	want := "## Backlog\n\n## In Progress\n\n## Done\n\n## Scratchpad\n"
	if got := b.String(); got != want {
		t.Fatalf("empty board = %q, want %q", got, want)
	}
	// Parsing it back yields an equivalent empty board.
	got := Parse(want)
	if got.String() != want {
		t.Fatalf("empty round-trip = %q", got.String())
	}
}

func TestBracketsInCards(t *testing.T) {
	b := Parse("## Backlog\n- [ ] handle array[0] and [nested [brackets]]\n\n## Scratchpad\n")
	if len(b.Cols[Backlog]) != 1 || b.Cols[Backlog][0] != "handle array[0] and [nested [brackets]]" {
		t.Fatalf("bracket card lost: %#v", b.Cols[Backlog])
	}
}

func TestScratchIsRawToEOF(t *testing.T) {
	// Scratch text may legally contain lines that look like headers or cards.
	scratch := "## Not A Real Header\n- [ ] not a real card\nplain text"
	src := "## Backlog\n\n## In Progress\n\n## Done\n\n## Scratchpad\n" + scratch + "\n"
	b := Parse(src)
	if len(b.Cols[Backlog]) != 0 {
		t.Fatalf("scratch header leaked into backlog: %#v", b.Cols[Backlog])
	}
	if b.Scratch != scratch {
		t.Fatalf("scratch = %q, want %q", b.Scratch, scratch)
	}
	if b.String() != src {
		t.Fatalf("scratch round-trip mismatch:\n got %q\nwant %q", b.String(), src)
	}
}

func TestPreambleAndUnknownSectionsPreserved(t *testing.T) {
	src := "# My Board\n" +
		"\n" +
		"## Backlog\n" +
		"- [ ] task\n" +
		"\n" +
		"## Notes\n" +
		"free text a\nfree text b\n" +
		"\n" +
		"## In Progress\n" +
		"\n" +
		"## Done\n" +
		"\n" +
		"## Scratchpad\n"

	b := Parse(src)
	if b.Preamble != "# My Board" {
		t.Fatalf("preamble = %q", b.Preamble)
	}
	if len(b.Extra) != 1 || b.Extra[0].Title != "Notes" || b.Extra[0].Body != "free text a\nfree text b" {
		t.Fatalf("extra = %#v", b.Extra)
	}
	// Serialization is stable even though Extra moves before Scratchpad.
	if got := b.String(); Parse(got).String() != got {
		t.Fatalf("not idempotent with preamble/extra:\n%q", got)
	}
}

func TestCheckboxNormalizedBySection(t *testing.T) {
	// A card checked in Backlog stays in Backlog and serializes back to "[ ]";
	// column, not checkbox, is canonical. A Done card serializes to "[x]".
	b := Parse("## Backlog\n- [x] checked in wrong column\n\n## Done\n- [ ] finished\n\n## Scratchpad\n")
	got := b.String()
	if !strings.Contains(got, "## Backlog\n- [ ] checked in wrong column") {
		t.Fatalf("backlog checkbox not normalized to [ ]:\n%s", got)
	}
	if !strings.Contains(got, "## Done\n- [x] finished") {
		t.Fatalf("done checkbox not normalized to [x]:\n%s", got)
	}
}

func TestIdempotency(t *testing.T) {
	inputs := []string{
		"",
		"## Backlog\n- [ ] a\n\n## In Progress\n\n## Done\n\n## Scratchpad\n",
		"junk before\n## Backlog\n- [ ] a\n- [x] b was checked\n\n## Done\n- [x] c\n\n## Scratchpad\nnotes\n\nmore notes\n",
		"## Scratchpad\nonly scratch, no columns\n",
	}
	for _, in := range inputs {
		once := Parse(in).String()
		twice := Parse(once).String()
		if once != twice {
			t.Fatalf("not idempotent for %q:\n once: %q\ntwice: %q", in, once, twice)
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	b, err := Load(filepath.Join(t.TempDir(), "does-not-exist.md"))
	if err != nil {
		t.Fatalf("Load missing file err = %v", err)
	}
	if b.String() != "## Backlog\n\n## In Progress\n\n## Done\n\n## Scratchpad\n" {
		t.Fatalf("missing file did not yield empty board: %q", b.String())
	}
}

func TestSaveIsAtomicAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".board.md")
	b := &Board{}
	b.Add(Backlog, "hello")
	b.Scratch = "some notes"

	if err := Save(path, b); err != nil {
		t.Fatalf("Save err = %v", err)
	}
	// File exists with expected mode and content.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat err = %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v, want 0644", info.Mode().Perm())
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("reload err = %v", err)
	}
	if got.String() != b.String() {
		t.Fatalf("reload mismatch:\n got %q\nwant %q", got.String(), b.String())
	}
	// No temp files left behind in the directory.
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".board-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestMutations(t *testing.T) {
	b := &Board{}
	i := b.Add(Backlog, "  trimmed  ")
	if i != 0 || b.Cols[Backlog][0] != "trimmed" {
		t.Fatalf("Add trim failed: %#v", b.Cols[Backlog])
	}
	if b.Add(Backlog, "   ") != -1 {
		t.Fatalf("blank Add should return -1")
	}
	b.Add(Backlog, "second")

	// Move first backlog card to In Progress.
	ni := b.MoveCard(Backlog, 0, InProgress)
	if ni != 0 || len(b.Cols[Backlog]) != 1 || b.Cols[InProgress][0] != "trimmed" {
		t.Fatalf("MoveCard failed: bl=%#v ip=%#v", b.Cols[Backlog], b.Cols[InProgress])
	}

	// Reorder within a column.
	b.Add(InProgress, "x")
	b.Swap(InProgress, 0, 1)
	if b.Cols[InProgress][0] != "x" {
		t.Fatalf("Swap failed: %#v", b.Cols[InProgress])
	}

	// Edit + delete.
	b.Set(InProgress, 0, "renamed")
	if b.Cols[InProgress][0] != "renamed" {
		t.Fatalf("Set failed: %#v", b.Cols[InProgress])
	}
	b.Delete(InProgress, 0)
	if len(b.Cols[InProgress]) != 1 || b.Cols[InProgress][0] != "trimmed" {
		t.Fatalf("Delete failed: %#v", b.Cols[InProgress])
	}

	// Out-of-range ops are no-ops, not panics.
	b.Delete(Backlog, 99)
	b.Swap(Backlog, 0, 99)
	b.Set(Backlog, -1, "nope")
	if b.MoveCard(Done, 0, Backlog) != -1 {
		t.Fatalf("MoveCard from empty column should return -1")
	}
}
