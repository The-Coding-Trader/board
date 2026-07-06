# board

A tiny three-column kanban **TUI** — Backlog · In Progress · Done — with a
scratchpad, meant to run in a pane next to your Claude Code sessions.

The board is stored as a **plain Markdown file** (`./.board.md` in whatever
directory you launch it from). That's the whole trick: because it's human-readable
Markdown, a Claude Code session working in the same directory can read your
backlog, add cards, or move them — and the running TUI picks up those changes
live (it re-reads the file about once a second). You get a shared board without a
server, a database, or any glue.

```
 ◆ board   ./.board.md                                                    6 cards
╭────────────────────────────╮╭────────────────────────────╮╭────────────────────────────╮
│ BACKLOG  3                 ││ IN PROGRESS  1             ││ DONE  2                    │
│ • wire up scratch pane     ││ • column rendering         ││ ✓ project scaffold         │
│ • add color theme          ││                            ││ ✓ markdown store + tests   │
│ • handle [auth] bug        ││                            ││                            │
╰────────────────────────────╯╰────────────────────────────╯╰────────────────────────────╯
╭──────────────────────────────────────────────────────────────────────────────────────────╮
│ SCRATCHPAD  tab to edit                                                                    │
│ remember: atomic writes so Claude never sees a torn file                                   │
╰──────────────────────────────────────────────────────────────────────────────────────────╯
 a:add e:edit h/l:col j/k:card H/L:move spc:done d:del tab:notes q:quit
```

## Run

Go is provided by [mise](https://mise.jdx.dev/) (see `mise.toml`).

```sh
make run              # run in the current directory (uses ./.board.md)
make build            # build ./board
make install          # install `board` onto your PATH (GOBIN / GOPATH/bin)
```

Once installed, just run `board` in any project directory and it reads/writes
`./.board.md` there — the same directory your Claude Code session is working in.

```sh
board                 # ./.board.md in the current directory
board -file notes.md  # or point it at any file
```

## Keys

| Key | Action |
| --- | --- |
| `←/→` or `h/l` | focus previous / next column |
| `↑/↓` or `k/j` | select previous / next card |
| `g` / `G` | jump to first / last card |
| `a` or `n` | add a card to the focused column |
| `e` or `Enter` | edit the selected card |
| `H` / `L` (or `<` / `>`) | move the card to the previous / next column |
| `Space` | send the card to **Done** (from Done, back to Backlog) |
| `J` / `K` | reorder the card down / up within its column |
| `d` or `x` | delete the selected card |
| `Tab` or `s` | jump into the scratchpad; `Tab`/`Esc` there saves and returns |
| `Ctrl-P` (in scratchpad) | promote the line under the cursor into a Backlog card |
| `r` | reload from disk now |
| `q` or `Ctrl-C` | quit (saves first) |

In the scratchpad, jot a thought on its own line and hit `Ctrl-P` to turn it into
a Backlog card — the line moves out of your notes and onto the board.

Every change is saved immediately (atomically), so the file on disk always
reflects what you see.

## The file format

`.board.md` is just Markdown with four fixed `##` sections:

```markdown
## Backlog
- [ ] a card

## In Progress
- [ ] another card

## Done
- [x] a finished card

## Scratchpad
free-form notes — this section is kept verbatim, so it may safely contain
lines that look like ## headers or - [ ] cards
```

Notes:

- A card's **column is decided by its section**, not its checkbox. On save,
  Done cards are written as `- [x]` and the rest as `- [ ]`. (So if Claude
  ticks a box in Backlog, the card stays in Backlog — move it with `Space` or
  `L` to actually complete it.)
- The **Scratchpad is always last** and captured raw to end-of-file.
- Any preamble above the first header and any unknown `## Sections` you (or
  Claude) add are **preserved** across saves — board won't delete content it
  doesn't model.
- While you're actively typing in the scratchpad, an external change to the
  scratch text won't clobber your edits; you'll see a hint and can reload with
  `r` after tabbing out.

## Layout

```
cmd/board/main.go          entrypoint: resolve ./.board.md, load, run
internal/board/store.go    the data core — Markdown parse/serialize + atomic save
internal/ui/               Bubble Tea model, key routing, column rendering
internal/theme/theme.go    the "Reef" teal palette
```

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) v2,
[Bubbles](https://github.com/charmbracelet/bubbles) v2, and
[Lip Gloss](https://github.com/charmbracelet/lipgloss) v2.

```sh
make test    # store round-trip/idempotency tests + headless model-update tests
make vet
```

## License

[MIT](LICENSE) © Joey Beninghove
