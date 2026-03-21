package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/openclaw/wsfold/internal/wsfold"
	"github.com/sahilm/fuzzy"
)

var errPickerCancelled = errors.New("selection cancelled")

const minFuzzyMatchScore = -20
const pickerVisibleItems = 20

type pickerFunc func(app *wsfold.App, cwd string, command string, stdout io.Writer, stderr io.Writer) ([]string, error)

var runPicker pickerFunc = runBubbleTeaPicker

func runBubbleTeaPicker(app *wsfold.App, cwd string, command string, stdout io.Writer, stderr io.Writer) ([]string, error) {
	model, err := buildPickerModel(app, cwd, command)
	if err != nil {
		return nil, err
	}
	program := tea.NewProgram(
		model,
		tea.WithInput(os.Stdin),
		tea.WithOutput(stdout),
		tea.WithAltScreen(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return nil, err
	}

	result, ok := finalModel.(pickerModel)
	if !ok {
		return nil, fmt.Errorf("unexpected picker model type %T", finalModel)
	}
	if result.err != nil {
		return nil, result.err
	}
	return result.selectedValues(), nil
}

func buildPickerModel(app *wsfold.App, cwd string, command string) (pickerModel, error) {
	if command == "summon" {
		state, err := app.TrustedSummonPickerState(cwd)
		if err != nil {
			return pickerModel{}, err
		}

		model := newPickerModel(command, state.Candidates)
		model.status = state.Status
		model.refreshing = state.Refreshing
		if state.Refreshing {
			model.initCmd = refreshTrustedSummonPickerCmd(app, cwd)
		}
		return model, nil
	}

	candidates, err := app.Complete(cwd, command, "")
	if err != nil {
		return pickerModel{}, err
	}
	if len(candidates) == 0 {
		return pickerModel{}, fmt.Errorf("no candidates available for %s", command)
	}
	return newPickerModel(command, candidates), nil
}

type pickerItem struct {
	candidate wsfold.CompletionCandidate
	search    string
}

type trustedSummonRefreshMsg struct {
	state wsfold.TrustedSummonPickerState
	err   error
}

type pickerModel struct {
	command    string
	input      textinput.Model
	items      []pickerItem
	filtered   []pickerItem
	cursor     int
	selected   map[string]bool
	err        error
	status     string
	refreshing bool
	initCmd    tea.Cmd
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
			search:    pickerSearchText(candidate),
		})
	}

	model := pickerModel{
		command:  command,
		input:    input,
		items:    items,
		selected: map[string]bool{},
	}
	if command == "summon" || command == "summon-untrusted" {
		for _, candidate := range candidates {
			if candidate.Attached {
				model.selected[candidate.Value] = true
			}
		}
	}
	model.refresh()
	return model
}

func (m pickerModel) Init() tea.Cmd {
	if m.initCmd != nil {
		return tea.Batch(textinput.Blink, m.initCmd)
	}
	return textinput.Blink
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case trustedSummonRefreshMsg:
		m.refreshing = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Remote refresh failed: %v; using cached results", msg.err)
			return m, nil
		}
		m.status = msg.state.Status
		m.replaceCandidates(msg.state.Candidates)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.err = errPickerCancelled
			return m, tea.Quit
		case "up", "ctrl+p":
			m.moveCursor(-1)
			return m, nil
		case "down", "ctrl+n", "tab":
			m.moveCursor(1)
			return m, nil
		case "shift+tab":
			m.moveCursor(-1)
			return m, nil
		case "pgdown", "ctrl+f":
			m.moveCursor(pickerVisibleItems)
			return m, nil
		case "pgup", "ctrl+b":
			m.moveCursor(-pickerVisibleItems)
			return m, nil
		case " ":
			if len(m.filtered) == 0 {
				return m, nil
			}
			current := m.filtered[m.cursor].candidate.Value
			if m.selected[current] {
				delete(m.selected, current)
			} else {
				m.selected[current] = true
			}
			return m, nil
		case "enter":
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
		if match.Score < minFuzzyMatchScore {
			continue
		}
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

func (m *pickerModel) replaceCandidates(candidates []wsfold.CompletionCandidate) {
	currentValue := ""
	if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
		currentValue = m.filtered[m.cursor].candidate.Value
	}

	items := make([]pickerItem, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, pickerItem{
			candidate: candidate,
			search:    pickerSearchText(candidate),
		})
	}
	m.items = items
	m.refresh()

	if currentValue == "" {
		return
	}
	for i, item := range m.filtered {
		if item.candidate.Value == currentValue {
			m.cursor = i
			return
		}
	}
}

func (m *pickerModel) moveCursor(delta int) {
	if len(m.filtered) == 0 || delta == 0 {
		return
	}

	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
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
	emptyMarkerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	markerStyle := lipgloss.NewStyle().Foreground(selectionMarkerColor(m.command))
	localStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	remoteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	slugStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	lines := []string{
		titleStyle.Render(pickerTitle(m.command)),
		m.input.View(),
		"",
	}

	if len(m.filtered) == 0 {
		lines = append(lines, hintStyle.Render("No matches"))
	} else {
		start, end := visibleRange(m.cursor, len(m.filtered), pickerVisibleItems)
		nameWidth, sourceWidth := pickerColumnWidths(m.filtered[start:end])
		for i := start; i < end; i++ {
			item := m.filtered[i].candidate
			prefix := "  "
			selectMarker := emptyMarkerStyle.Render("○")
			if m.selected[item.Value] {
				selectMarker = markerStyle.Render("●")
			}
			render := renderPickerRow(item, selectMarker, nameWidth, sourceWidth, localStyle, remoteStyle, slugStyle, descStyle)
			if i == m.cursor {
				prefix = "> "
				render = selectedStyle.Render(render)
			}
			lines = append(lines, prefix+render)
		}
		lines = append(lines, "", hintStyle.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.filtered))))
	}

	lines = append(lines, "", hintStyle.Render("Space toggle, Enter apply, PgUp/PgDn (Fn+Up/Fn+Down) scroll, Esc cancel, type to fuzzy filter"))
	if strings.TrimSpace(m.status) != "" {
		lines = append(lines, hintStyle.Render(m.status))
	} else if m.refreshing {
		lines = append(lines, hintStyle.Render("Refreshing trusted GitHub index..."))
	}
	return strings.Join(lines, "\n")
}

func (m pickerModel) selectedValues() []string {
	if len(m.selected) == 0 {
		return nil
	}

	values := make([]string, 0, len(m.selected))
	for _, item := range m.items {
		if m.selected[item.candidate.Value] {
			values = append(values, item.candidate.Value)
		}
	}
	return values
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

func selectionMarkerColor(command string) lipgloss.TerminalColor {
	if command == "dismiss" {
		return lipgloss.Color("196")
	}
	return lipgloss.Color("42")
}

func pickerPrimaryText(candidate wsfold.CompletionCandidate) string {
	if candidate.Name != "" {
		return candidate.Name
	}
	return candidate.Value
}

func pickerSearchText(candidate wsfold.CompletionCandidate) string {
	parts := []string{pickerPrimaryText(candidate)}

	detail := candidate.Slug
	if detail == "" {
		detail = candidate.Description
	}
	if strings.TrimSpace(detail) != "" && detail != parts[0] {
		parts = append(parts, detail)
	}

	return strings.TrimSpace(strings.Join(parts, " "))
}

func refreshTrustedSummonPickerCmd(app *wsfold.App, cwd string) tea.Cmd {
	return func() tea.Msg {
		state, err := app.RefreshTrustedSummonPickerState(cwd)
		return trustedSummonRefreshMsg{state: state, err: err}
	}
}

func renderPickerRow(
	candidate wsfold.CompletionCandidate,
	selectMarker string,
	nameWidth int,
	sourceWidth int,
	localStyle lipgloss.Style,
	remoteStyle lipgloss.Style,
	slugStyle lipgloss.Style,
	descStyle lipgloss.Style,
) string {
	name := lipgloss.NewStyle().Width(nameWidth).Render(truncateText(pickerPrimaryText(candidate), nameWidth))
	row := fmt.Sprintf("%s %s", selectMarker, name)

	if candidate.Source != "" {
		sourceText := lipgloss.NewStyle().Width(sourceWidth).Render(string(candidate.Source))
		row = fmt.Sprintf("%s  %s", row, renderSourceMarkerText(candidate.Source, sourceText, localStyle, remoteStyle))
	}

	detail := candidate.Slug
	if detail == "" {
		detail = candidate.Description
	}
	if detail != "" {
		detail = truncateText(detail, 48)
		if candidate.Slug != "" {
			row = fmt.Sprintf("%s  %s", row, slugStyle.Render(detail))
		} else {
			row = fmt.Sprintf("%s  %s", row, descStyle.Render(detail))
		}
	}

	return row
}

func pickerColumnWidths(items []pickerItem) (int, int) {
	nameWidth := 0
	sourceWidth := 0
	for _, item := range items {
		nameWidth = max(nameWidth, min(displayWidth(pickerPrimaryText(item.candidate)), 28))
		sourceWidth = max(sourceWidth, displayWidth(string(item.candidate.Source)))
	}
	if nameWidth == 0 {
		nameWidth = 1
	}
	if sourceWidth == 0 {
		sourceWidth = len("remote")
	}
	return nameWidth, sourceWidth
}

func renderSourceMarkerText(source wsfold.CompletionSource, text string, localStyle lipgloss.Style, remoteStyle lipgloss.Style) string {
	switch source {
	case wsfold.CompletionSourceLocal:
		return localStyle.Render(text)
	case wsfold.CompletionSourceRemote:
		return remoteStyle.Render(text)
	default:
		return text
	}
}

func truncateText(text string, width int) string {
	if width <= 0 || displayWidth(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}

	runes := []rune(text)
	for len(runes) > 0 {
		candidate := string(runes) + "…"
		if displayWidth(candidate) <= width {
			return candidate
		}
		runes = runes[:len(runes)-1]
	}
	return "…"
}

func displayWidth(text string) int {
	return utf8.RuneCountInString(text)
}
