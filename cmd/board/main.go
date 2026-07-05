// Command board is a three-column kanban TUI (Backlog / In Progress / Done) with
// a scratchpad, backed by a human-readable Markdown file — ./.board.md in the
// current directory by default — so a Claude Code session running in the same
// directory can read and edit the same board.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/joey/board/internal/board"
	"github.com/joey/board/internal/ui"
)

func main() {
	file := flag.String("file", "", "board file (default: ./.board.md in the current directory)")
	flag.Parse()

	path := *file
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fatal(err)
		}
		path = filepath.Join(cwd, ".board.md")
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	b, err := board.Load(path)
	if err != nil {
		fatal(err)
	}

	var mod time.Time
	var size int64
	if fi, err := os.Stat(path); err == nil {
		mod, size = fi.ModTime(), fi.Size()
	}

	if _, err := tea.NewProgram(ui.New(b, path, mod, size)).Run(); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "board:", err)
	os.Exit(1)
}
