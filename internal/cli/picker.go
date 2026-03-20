package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/openclaw/wsfold/internal/wsfold"
	"github.com/sahilm/fuzzy"
)

var errPickerCancelled = errors.New("selection cancelled")

type pickerFunc func(app *wsfold.App, cwd string, command string, stdout io.Writer, stderr io.Writer) (string, error)

var runPicker pickerFunc = runBubbleTeaPicker

func runBubbleTeaPicker(app *wsfold.App, cwd string, command string, stdout io.Writer, stderr io.Writer) (string, error) {
	candidates, err := app.Complete(cwd, command, "")
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no candidates available for %s", command)
	}

	model := newPickerModel(command, candidates)
	program := tea.NewProgram(
		model,
		tea.WithInput(os.Stdin),
		tea.WithOutput(stdout),
		tea.WithAltScreen(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return "", err
	}

	result, ok := finalModel.(pickerModel)
	if !ok {
		return "", fmt.Errorf("unexpected picker model type %T", finalModel)
	}
	if result.err != nil {
		return "", result.err
	}
	if result.selected == nil {
		return "", fmt.Errorf("no selection made")
	}
	return result.selected.Value, nil
}

type pickerItem struct {
	candidate wsfold.CompletionCandidate
	search    string
}

type pickerModel struct {
	command  string
	input    textinput.Model
	items    []pickerItem
	filtered []pickerItem
	cursor   int
	selected *wsfold.CompletionCandidate
	err      error
}

func newPickerModel(command string, candidates []wsfold.CompletionCandidate) pickerModel {
	input := textinput.New()
	input.Placeholder = "Type to fuzzy filter repositories"
	input.Prompt = "filter> "
	input.Focus()
	input.CharLimit = 256
	input.Width = 48

	items := make([]pickerItem, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, pickerItem{
			candidate: candidate,
			search:    strings.TrimSpace(candidate.Value + " " + candidate.Description),
		})
	}

	model := pickerModel{
		command: command,
		input:   input,
		items:   items,
	}
	model.refresh()
	return model
}

func (m pickerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.err = errPickerCancelled
			return m, tea.Quit
		case "up", "ctrl+p":
			if len(m.filtered) > 0 && m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+n", "tab":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "shift+tab":
			if len(m.filtered) > 0 && m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "enter":
			if len(m.filtered) == 0 {
				return m, nil
			}
			selected := m.filtered[m.cursor].candidate
			m.selected = &selected
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refresh()
	return m, cmd
}

func (m *pickerModel) refresh() {
	query := strings.TrimSpace(m.input.Value())
	if query == "" {
		m.filtered = append(m.filtered[:0], m.items...)
		if m.cursor >= len(m.filtered) {
			m.cursor = max(0, len(m.filtered)-1)
		}
		return
	}

	searchable := make([]string, 0, len(m.items))
	for _, item := range m.items {
		searchable = append(searchable, item.search)
	}

	matches := fuzzy.Find(query, searchable)
	filtered := make([]pickerItem, 0, len(matches))
	for _, match := range matches {
		filtered = append(filtered, m.items[match.Index])
	}

	m.filtered = filtered
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
}

func (m pickerModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	markerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	emptyMarkerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	lines := []string{
		titleStyle.Render(pickerTitle(m.command)),
		m.input.View(),
		"",
	}

	if len(m.filtered) == 0 {
		lines = append(lines, hintStyle.Render("No matches"))
	} else {
		start, end := visibleRange(m.cursor, len(m.filtered), 10)
		for i := start; i < end; i++ {
			item := m.filtered[i].candidate
			prefix := "  "
			marker := emptyMarkerStyle.Render(" ")
			if item.Attached {
				marker = markerStyle.Render("✓")
			}
			render := fmt.Sprintf("%s %s", marker, item.Value)
			if item.Description != "" {
				render = fmt.Sprintf("%s  %s", render, descStyle.Render(item.Description))
			}
			if i == m.cursor {
				prefix = "> "
				render = selectedStyle.Render(render)
			}
			lines = append(lines, prefix+render)
		}
	}

	lines = append(lines, "", hintStyle.Render("Enter select, Esc cancel, type to fuzzy filter"))
	return strings.Join(lines, "\n")
}

func pickerTitle(command string) string {
	switch command {
	case "summon":
		return "Select trusted repository"
	case "summon-untrusted":
		return "Select external repository"
	case "dismiss":
		return "Select repository to dismiss"
	default:
		return "Select repository"
	}
}

func visibleRange(cursor int, total int, maxItems int) (int, int) {
	if total <= maxItems {
		return 0, total
	}

	start := cursor - maxItems/2
	if start < 0 {
		start = 0
	}
	end := start + maxItems
	if end > total {
		end = total
		start = end - maxItems
	}
	return start, end
}
