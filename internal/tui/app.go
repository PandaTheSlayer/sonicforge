package tui

import (
	"fmt"
	"strings"

	"sonicforge/internal/engine"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	stateBrowse state = iota
	stateDone
)

type Model struct {
	candidates []engine.Candidate
	approved   []engine.Candidate
	cursor     int
	state      state
	width      int
	height     int
}

func New(candidates []engine.Candidate) Model {
	return Model{
		candidates: candidates,
		state:      stateBrowse,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateBrowse:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.candidates)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			// approve current candidate
			m.approved = append(m.approved, m.candidates[m.cursor])
			m.candidates = append(m.candidates[:m.cursor], m.candidates[m.cursor+1:]...)
			if m.cursor >= len(m.candidates) {
				m.cursor = len(m.candidates) - 1
			}
			if len(m.candidates) == 0 {
				m.state = stateDone
			}
		case "x":
			// skip current candidate
			m.candidates = append(m.candidates[:m.cursor], m.candidates[m.cursor+1:]...)
			if m.cursor >= len(m.candidates) {
				m.cursor = len(m.candidates) - 1
			}
			if len(m.candidates) == 0 {
				m.state = stateDone
			}
		case "a":
			// approve all remaining
			m.approved = append(m.approved, m.candidates...)
			m.candidates = nil
			m.cursor = 0
			m.state = stateDone
		case "d":
			// done — go to summary
			m.state = stateDone
		}
	case stateDone:
		switch msg.String() {
		case "q", "ctrl+c", "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	switch m.state {
	case stateBrowse:
		return m.viewBrowse()
	case stateDone:
		return m.viewDone()
	}
	return ""
}

// Approved returns the list of approved candidates after TUI exits.
func (m Model) Approved() []engine.Candidate {
	return m.approved
}

// --- styles ---

var (
	titleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	selectedStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	normalStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	approvedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	helpStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	lastfmStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))  // green
	lbCFStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))  // blue
	lbRadioStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // yellow
	counterStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// --- views ---

func (m Model) viewBrowse() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("sonicforge — browse candidates"))
	b.WriteString("  ")
	b.WriteString(counterStyle.Render(fmt.Sprintf("[%d/%d]", m.cursor+1, len(m.candidates))))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d remaining · %d approved", len(m.candidates), len(m.approved))))
	b.WriteString("\n\n")

	// show a window of candidates around cursor
	start := m.cursor - 5
	if start < 0 {
		start = 0
	}
	end := start + 15
	if end > len(m.candidates) {
		end = len(m.candidates)
	}

	for i := start; i < end; i++ {
		c := m.candidates[i]
		line := fmt.Sprintf("%s — %s (%.2f)", c.ArtistName, c.TrackName, c.CompositeScore)
		sourceTag := sourceLabel(c.Source)

		if i == m.cursor {
			b.WriteString("  ▸ " + selectedStyle.Render(line) + " " + sourceTag)
		} else {
			b.WriteString("    " + normalStyle.Render(line) + " " + sourceTag)
		}
		b.WriteString("\n")
	}

	// detail for selected candidate
	if m.cursor >= 0 && m.cursor < len(m.candidates) {
		c := m.candidates[m.cursor]
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  similarity: %.2f · tag overlap: %.2f · source: %s",
			c.SimilarityScore, c.TagOverlapScore, c.Source)))
		b.WriteString("\n")
		if len(c.Tags) > 0 {
			tags := c.Tags
			if len(tags) > 8 {
				tags = tags[:8]
			}
			b.WriteString(dimStyle.Render("  tags: " + strings.Join(tags, ", ")))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  ↑/k up · ↓/j down · enter approve · x skip · a approve all · d done · q quit"))

	return b.String()
}

func sourceLabel(source string) string {
	switch source {
	case "lastfm-similar":
		return lastfmStyle.Render("[last.fm]")
	case "lb-cf":
		return lbCFStyle.Render("[lb-cf]")
	case "lb-radio-tag":
		return lbRadioStyle.Render("[lb-radio]")
	case "lb-radio-artist":
		return lbRadioStyle.Render("[lb-artist]")
	default:
		return dimStyle.Render("[" + source + "]")
	}
}

func (m Model) viewDone() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("sonicforge — approved tracks"))
	b.WriteString("\n\n")

	if len(m.approved) == 0 {
		b.WriteString("  No tracks approved.\n")
	} else {
		for i, c := range m.approved {
			b.WriteString(approvedStyle.Render(fmt.Sprintf("  %d. %s — %s", i+1, c.ArtistName, c.TrackName)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("  %d tracks · press enter to create playlist · q to quit", len(m.approved))))

	return b.String()
}
