package cli

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/openclaw/wsfold/internal/wsfold"
)

func TestPickerModelSupportsMultiSelect(t *testing.T) {
	model := newPickerModel("summon", []wsfold.CompletionCandidate{
		{Value: "alpha"},
		{Value: "beta"},
		{Value: "gamma"},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)
	if !model.selected["alpha"] {
		t.Fatalf("expected current row to be selected after space, got %#v", model.selected)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(pickerModel)

	selected := model.selectedValues()
	if len(selected) != 2 || selected[0] != "alpha" || selected[1] != "beta" {
		t.Fatalf("unexpected selected values after multi-select confirm: %#v", selected)
	}
}

func TestPickerModelEnterSelectsCurrentWhenNothingMarked(t *testing.T) {
	model := newPickerModel("dismiss", []wsfold.CompletionCandidate{
		{Value: "alpha"},
		{Value: "beta"},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(pickerModel)

	selected := model.selectedValues()
	if len(selected) != 1 || selected[0] != "beta" {
		t.Fatalf("unexpected selected values after single confirm: %#v", selected)
	}
}
