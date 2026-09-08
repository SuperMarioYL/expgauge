// Package ui renders the live exposure badge as a bubbletea TUI.
// It shows wall-clock time, mutation count, token burn, and progress
// bars toward each threshold. When a threshold is crossed, it blocks
// on a human-review prompt (y/n to continue).
package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SuperMarioYL/expgauge/internal/metrics"
	"github.com/SuperMarioYL/expgauge/internal/threshold"
)

const (
	padWidth = 44
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D3CFF"))
	okStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#27AE60")).Bold(true)
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F39C12")).Bold(true)
	exceedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E74C3C")).Bold(true)
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	valueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	barBgStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))
	barFillStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D3CFF"))
	promptStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F39C12"))
)

// SnapshotFunc returns the current exposure. The badge polls it on each tick.
type SnapshotFunc func() metrics.Exposure

// tickMsg is sent on each refresh interval.
type tickMsg time.Time

// BadgeModel is the bubbletea model for the live exposure badge.
type BadgeModel struct {
	snapshot   SnapshotFunc
	engine     *threshold.Engine
	pid        int
	status     threshold.Result
	reviewPrompt bool
	quitting   bool
	lastExp    metrics.Exposure
	onReview   func() // called when user approves continuing
}

// NewBadge creates the badge model. onReview is called when the user
// approves continuing past a threshold pause.
func NewBadge(snap SnapshotFunc, eng *threshold.Engine, pid int, onReview func()) BadgeModel {
	return BadgeModel{
		snapshot: snap,
		engine:   eng,
		pid:      pid,
		onReview: onReview,
	}
}

func (m BadgeModel) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m BadgeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "y", "Y":
			if m.reviewPrompt {
				m.reviewPrompt = false
				if m.onReview != nil {
					m.onReview()
				}
				if m.pid > 0 {
					_ = threshold.ForceResume(m.pid)
				}
			}
		case "n", "N":
			if m.reviewPrompt {
				m.reviewPrompt = false
				m.quitting = true
				return m, tea.Quit
			}
		}
	case tickMsg:
		exp := m.snapshot()
		m.lastExp = exp
		m.status = m.engine.Evaluate(exp)
		if m.status.Status == threshold.StatusExceeded && !m.reviewPrompt {
			m.reviewPrompt = true
			if m.pid > 0 {
				_ = threshold.ForcePause(m.pid)
			}
		}
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	}
	return m, nil
}

func (m BadgeModel) View() string {
	if m.quitting {
		return ""
	}
	if m.reviewPrompt {
		return renderReviewPrompt(m.lastExp, m.status)
	}
	return renderBadge(m.lastExp, m.status)
}

// renderBadge draws the live exposure badge with progress bars.
func renderBadge(exp metrics.Exposure, res threshold.Result) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" expgauge "))
	b.WriteString(labelStyle.Render(" exposure badge"))
	b.WriteString("\n\n")

	statusLabel := res.Status.String()
	statusStr := statusLabel
	switch res.Status {
	case threshold.StatusOK:
		statusStr = okStyle.Render(statusLabel)
	case threshold.StatusWarning:
		statusStr = warnStyle.Render(statusLabel)
	case threshold.StatusExceeded:
		statusStr = exceedStyle.Render(statusLabel)
	}
	b.WriteString(fmt.Sprintf("  %s %s\n\n", labelStyle.Render("status:"), statusStr))

	b.WriteString(renderMetric("wall", formatDuration(exp.WallClockSec), res.WallClockPct))
	b.WriteString("\n")
	b.WriteString(renderMetric("mut ", fmt.Sprintf("%d", exp.Mutations), res.MutationsPct))
	b.WriteString("\n")
	b.WriteString(renderMetric("tok ", formatTokens(exp.Tokens), res.TokensPct))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("  [q] quit"))
	b.WriteString("\n")

	return b.String()
}

// renderMetric draws a single metric row with a progress bar.
func renderMetric(label, value string, pct float64) string {
	bar := renderBar(pct)
	return fmt.Sprintf("  %s %s  %s  %s",
		labelStyle.Render(label),
		valueStyle.Render(value),
		bar,
		labelStyle.Render(fmt.Sprintf("%.0f%%", pct*100)),
	)
}

// renderBar draws a 16-cell progress bar.
func renderBar(pct float64) string {
	const width = 16
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	if pct > 1 {
		pct = 1
	}
	bar := barFillStyle.Render(strings.Repeat("█", filled)) +
		barBgStyle.Render(strings.Repeat("░", width-filled))
	return bar
}

// renderReviewPrompt draws the forced-pause review prompt.
func renderReviewPrompt(exp metrics.Exposure, res threshold.Result) string {
	var b strings.Builder
	b.WriteString(exceedStyle.Render(" ⚠ THRESHOLD EXCEEDED"))
	b.WriteString("\n\n")
	b.WriteString("The agent has crossed an exposure threshold:\n\n")
	if contains(res.ExceededBy, "wall_clock") {
		b.WriteString(fmt.Sprintf("  wall-clock:  %s\n", formatDuration(exp.WallClockSec)))
	}
	if contains(res.ExceededBy, "mutations") {
		b.WriteString(fmt.Sprintf("  mutations:   %d\n", exp.Mutations))
	}
	if contains(res.ExceededBy, "tokens") {
		b.WriteString(fmt.Sprintf("  tokens:      %s\n", formatTokens(exp.Tokens)))
	}
	b.WriteString("\n")
	b.WriteString(promptStyle.Render("  Review the agent's work. Continue? [y/n]"))
	b.WriteString("\n")
	return b.String()
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// formatDuration renders seconds as h m s.
func formatDuration(sec int64) string {
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// formatTokens renders token counts with K/M suffixes.
func formatTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// Run starts the badge TUI. Blocks until the user quits or declines a review.
func Run(m BadgeModel) error {
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
