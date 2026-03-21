package cli

import (
	"regexp"
	"strings"
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

func TestPickerModelRendersSourceMarkersAndStatus(t *testing.T) {
	model := newPickerModel("summon", []wsfold.CompletionCandidate{
		{Value: "service", Name: "service", Slug: "acme/service", Source: wsfold.CompletionSourceLocal},
		{Value: "acme/worker", Name: "worker", Slug: "acme/worker", Source: wsfold.CompletionSourceRemote},
	})
	model.status = "remote index unavailable: gh is not installed"

	view := model.View()
	for _, expected := range []string{"service", "local", "acme/service", "worker", "remote", "gh is not installed"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected picker view to contain %q, got:\n%s", expected, view)
		}
	}
}

func TestPickerModelRefreshMessageMergesCandidatesAndPreservesState(t *testing.T) {
	model := newPickerModel("summon", []wsfold.CompletionCandidate{
		{Value: "service", Name: "service", Slug: "acme/service", Source: wsfold.CompletionSourceLocal},
		{Value: "acme/worker", Name: "worker", Slug: "acme/worker", Source: wsfold.CompletionSourceRemote},
	})
	model.refreshing = true

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)
	if !model.selected["acme/worker"] {
		t.Fatalf("expected worker to remain selected, got %#v", model.selected)
	}

	updated, _ = model.Update(trustedSummonRefreshMsg{
		state: wsfold.TrustedSummonPickerState{
			Candidates: []wsfold.CompletionCandidate{
				{Value: "service", Name: "service", Slug: "acme/service", Source: wsfold.CompletionSourceLocal},
				{Value: "acme/worker", Name: "worker", Slug: "acme/worker", Source: wsfold.CompletionSourceRemote},
				{Value: "acme/worker-api", Name: "worker-api", Slug: "acme/worker-api", Source: wsfold.CompletionSourceRemote},
			},
		},
	})
	model = updated.(pickerModel)

	if model.input.Value() != "wo" {
		t.Fatalf("expected filter query to be preserved, got %q", model.input.Value())
	}
	if !model.selected["acme/worker"] {
		t.Fatalf("expected previous selection to survive refresh, got %#v", model.selected)
	}
	if len(model.filtered) != 2 {
		t.Fatalf("expected refreshed results to respect current filter, got %#v", model.filtered)
	}
	if model.filtered[0].candidate.Value != "acme/worker" {
		t.Fatalf("expected cursor to stay on previous candidate, got %#v", model.filtered[0].candidate)
	}
}

func TestPickerModelAlignsColumnsForSummonRows(t *testing.T) {
	model := newPickerModel("summon", []wsfold.CompletionCandidate{
		{Value: "mikhail-yaskou/assistant", Name: "assistant", Slug: "mikhail-yaskou/assistant", Source: wsfold.CompletionSourceLocal},
		{Value: "atilarum/observability", Name: "observability", Slug: "atilarum/observability", Source: wsfold.CompletionSourceRemote},
	})

	view := stripANSI(model.View())
	lines := strings.Split(view, "\n")

	var rowLines []string
	for _, line := range lines {
		if strings.Contains(line, "assistant") || strings.Contains(line, "observability") {
			rowLines = append(rowLines, line)
		}
	}
	if len(rowLines) != 2 {
		t.Fatalf("expected 2 picker rows, got %d in view:\n%s", len(rowLines), view)
	}

	firstSource := strings.Index(rowLines[0], "local")
	secondSource := strings.Index(rowLines[1], "remote")
	if firstSource == -1 || secondSource == -1 {
		t.Fatalf("expected source markers in rows:\n%s", strings.Join(rowLines, "\n"))
	}
	if firstSource != secondSource {
		t.Fatalf("expected aligned source column, got:\n%s", strings.Join(rowLines, "\n"))
	}
}

func stripANSI(text string) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(text, "")
}
