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
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(pickerModel)

	selected := model.selectedValues()
	if len(selected) != 2 || selected[0] != "alpha" || selected[1] != "beta" {
		t.Fatalf("unexpected selected values after multi-select confirm: %#v", selected)
	}
}

func TestPickerModelEnterAllowsEmptySelection(t *testing.T) {
	model := newPickerModel("dismiss", []wsfold.CompletionCandidate{
		{Value: "alpha"},
		{Value: "beta"},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(pickerModel)

	selected := model.selectedValues()
	if len(selected) != 0 {
		t.Fatalf("expected empty selected values after confirm without toggles: %#v", selected)
	}
}

func TestPickerModelPreselectsAttachedReposForSummon(t *testing.T) {
	model := newPickerModel("summon", []wsfold.CompletionCandidate{
		{Value: "alpha", Attached: true},
		{Value: "beta", Attached: false},
	})

	if !model.selected["alpha"] {
		t.Fatalf("expected attached repo to start selected, got %#v", model.selected)
	}
	if model.selected["beta"] {
		t.Fatalf("did not expect unattached repo to start selected, got %#v", model.selected)
	}
}

func TestPickerModelDoesNotPreselectDismissCandidates(t *testing.T) {
	model := newPickerModel("dismiss", []wsfold.CompletionCandidate{
		{Value: "alpha", Attached: true},
	})

	if len(model.selected) != 0 {
		t.Fatalf("did not expect dismiss picker to preselect entries, got %#v", model.selected)
	}
}
