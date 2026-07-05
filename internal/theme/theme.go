// Package theme defines board's visual identity. Each kanban column carries a
// status color that reads like a traffic light — a cool blue Backlog (queued),
// a warm orange In Progress (in motion), and a green Done (complete) — so you
// can tell columns apart at a glance. The scratchpad gets its own sunny yellow.
// A restrained teal is the app's chrome accent (title bar, help keys, prompts),
// deliberately outside the status hues so it frames the board rather than
// competing with it.
package theme

import "charm.land/lipgloss/v2"

// Neutral canvas — charcoal-to-bone grays for chrome and card text.
var (
	Ink      = lipgloss.Color("#0E0E0E")
	Charcoal = lipgloss.Color("#1A1A1A")
	Slate    = lipgloss.Color("#2A2A2A")
	Muted    = lipgloss.Color("#6B6B6B")
	Fog      = lipgloss.Color("#9A9A9A")
	Bone     = lipgloss.Color("#E6E6E6")
)

// Per-column status hues: queued → active → done.
var (
	BacklogHue    = lipgloss.Color("#6E9EF0") // periwinkle blue — queued
	InProgressHue = lipgloss.Color("#F2914B") // warm orange — in motion
	DoneHue       = lipgloss.Color("#5CC98A") // green — complete
	ScratchHue    = lipgloss.Color("#FFD23F") // sunny yellow — notes
)

// Softer tints of the column hues, used to highlight the selected card so the
// cursor reads as a gentle wash of the column's color rather than a saturated
// block.
var (
	BacklogSoft    = lipgloss.Color("#A8C5F6")
	InProgressSoft = lipgloss.Color("#F7BD93")
	DoneSoft       = lipgloss.Color("#9DDEB9")
)

// Chrome / semantic accents.
var (
	Accent = lipgloss.Color("#1FB8A0") // teal — title bar, help keys, status
	Danger = lipgloss.Color("#E5654B") // delete / conflict hints
)
