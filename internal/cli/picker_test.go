package cli

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/openclaw/wsfold/internal/wsfold"
)

func TestPickerModelEnterSelectsCurrentRowInSingleMode(t *testing.T) {
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
		t.Fatalf("expected enter to select current row in single mode, got %#v", selected)
	}
}

func TestPickerModelSpaceEnablesMultiSelectAndTogglesItems(t *testing.T) {
	model := newPickerModel("dismiss", []wsfold.CompletionCandidate{
		{Value: "alpha"},
		{Value: "beta"},
		{Value: "gamma"},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)
	if !model.multiSelect {
		t.Fatalf("expected space to enable multi-select")
	}
	if len(model.selected) != 0 {
		t.Fatalf("did not expect entering multi-select to auto-select current row, got %#v", model.selected)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)
	if !model.selected["alpha"] {
		t.Fatalf("expected current row to become selected after toggling in multi-select, got %#v", model.selected)
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

func TestPickerModelEnterDoesNothingInMultiSelectWithoutSelections(t *testing.T) {
	model := newPickerModel("dismiss", []wsfold.CompletionCandidate{
		{Value: "alpha"},
		{Value: "beta"},
	})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)
	if !model.multiSelect {
		t.Fatalf("expected picker to enter multi-select mode")
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(pickerModel)
	if cmd != nil {
		t.Fatalf("did not expect enter to quit when multi-select has no selections")
	}
	if model.err != nil {
		t.Fatalf("did not expect picker error on inert enter, got %v", model.err)
	}
	if len(model.selectedValues()) != 0 {
		t.Fatalf("did not expect any selections after inert enter, got %#v", model.selectedValues())
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
	if model.multiSelect {
		t.Fatalf("did not expect dismiss picker to start in multi-select mode")
	}
}

func TestPickerModelRendersSourceMarkersAndStatus(t *testing.T) {
	model := newPickerModel("summon", []wsfold.CompletionCandidate{
		{Value: "service", Name: "service", Slug: "acme/service", Source: wsfold.CompletionSourceLocal},
		{Value: "acme/worker", Name: "worker", Slug: "acme/worker", Source: wsfold.CompletionSourceRemote},
	})
	model.status = "remote index unavailable: gh is not installed"

	view := model.View()
	for _, expected := range []string{"service", "local", "acme/service", "worker", "remote", "Choose a trusted repository to include in your workspace [Single mode]", "gh is not installed"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected picker view to contain %q, got:\n%s", expected, view)
		}
	}
}

func TestPickerModelRefreshMessageMergesCandidatesAndPreservesMultiSelectState(t *testing.T) {
	model := newPickerModel("dismiss", []wsfold.CompletionCandidate{
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
	if !model.multiSelect {
		t.Fatalf("expected selection to switch picker into multi-select mode")
	}
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
	if model.filtered[model.cursor].candidate.Value != "acme/worker" {
		t.Fatalf("expected cursor to stay on previous candidate, got %#v", model.filtered[model.cursor].candidate)
	}
}

func TestPickerModelShowsSelectedItemsInSeparateSectionWhileFiltering(t *testing.T) {
	model := newPickerModel("summon", []wsfold.CompletionCandidate{
		{Value: "alpha", Name: "alpha", Attached: true},
		{Value: "beta", Name: "beta", Attached: true},
		{Value: "gamma", Name: "gamma"},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = updated.(pickerModel)

	if len(model.filtered) != 1 {
		t.Fatalf("expected filtered list to contain only matches, got %#v", model.filtered)
	}
	if model.filtered[0].candidate.Value != "gamma" {
		t.Fatalf("expected matching item to remain in place, got %#v", model.filtered)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor to point at the visible match, got %d", model.cursor)
	}
	view := stripANSI(model.View())
	if !strings.Contains(view, "Choose a trusted repository to include in your workspace [Multi mode]") {
		t.Fatalf("expected multi-select title badge, got:\n%s", view)
	}
	if !strings.Contains(view, "Selected (2)") {
		t.Fatalf("expected separate selected section, got:\n%s", view)
	}
}

func TestPickerModelPreservesCursorWhenCurrentFilteredItemStillExists(t *testing.T) {
	model := newPickerModel("summon", []wsfold.CompletionCandidate{
		{Value: "alpha", Name: "alpha", Attached: true},
		{Value: "beta", Name: "beta", Attached: true},
		{Value: "gamma", Name: "gamma"},
		{Value: "gamut", Name: "gamut"},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = updated.(pickerModel)
	for i, item := range model.filtered {
		if item.candidate.Value == "gamut" {
			model.cursor = i
			break
		}
	}
	if model.filtered[model.cursor].candidate.Value != "gamut" {
		t.Fatalf("expected gamut to be present before narrowing filter, got %#v", model.filtered)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	model = updated.(pickerModel)
	if model.filtered[model.cursor].candidate.Value != "gamut" {
		t.Fatalf("expected cursor to stay on existing filtered item after refresh, got %#v", model.filtered[model.cursor].candidate)
	}
}

func TestPickerModelDeselectedNonMatchingItemsDisappearFromFilteredList(t *testing.T) {
	model := newPickerModel("dismiss", []wsfold.CompletionCandidate{
		{Value: "alpha", Name: "alpha"},
		{Value: "gamma", Name: "gamma"},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = updated.(pickerModel)
	if len(model.filtered) != 1 || model.filtered[0].candidate.Value != "gamma" {
		t.Fatalf("expected non-matching selected item to stay out of results, got %#v", model.filtered)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)
	if len(model.filtered) != 1 || model.filtered[0].candidate.Value != "gamma" {
		t.Fatalf("expected filtered results to stay stable after deselection, got %#v", model.filtered)
	}
}

func TestPickerModelDoesNotReorderSelectedItemsWhenPaging(t *testing.T) {
	candidates := []wsfold.CompletionCandidate{
		{Value: "alpha", Name: "alpha"},
		{Value: "beta", Name: "beta"},
	}
	for i := 0; i < 25; i++ {
		candidates = append(candidates, wsfold.CompletionCandidate{
			Value: fmt.Sprintf("repo-%02d", i),
			Name:  fmt.Sprintf("repo-%02d", i),
		})
	}

	model := newPickerModel("dismiss", candidates)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)
	for i := 0; i < 5; i++ {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(pickerModel)
	}
	if model.filtered[model.cursor].candidate.Value != "repo-03" {
		t.Fatalf("expected cursor to reach repo-03 before selection, got %#v", model.filtered[model.cursor].candidate)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)

	if model.filtered[0].candidate.Value != "alpha" || model.filtered[1].candidate.Value != "beta" {
		t.Fatalf("did not expect selected item to pin before paging, got %#v", model.filtered[:6])
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = updated.(pickerModel)

	if model.filtered[0].candidate.Value != "alpha" {
		t.Fatalf("expected paging to preserve original order, got %#v", model.filtered[:6])
	}
}

func TestPickerModelDoesNotReorderSelectedItemsWhenScrollingPastVisibleWindow(t *testing.T) {
	candidates := make([]wsfold.CompletionCandidate, 0, 30)
	for i := 0; i < 30; i++ {
		candidates = append(candidates, wsfold.CompletionCandidate{
			Value: fmt.Sprintf("repo-%02d", i),
			Name:  fmt.Sprintf("repo-%02d", i),
		})
	}

	model := newPickerModel("dismiss", candidates)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)

	for i := 0; i <= pickerVisibleItems; i++ {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(pickerModel)
	}
	if model.filtered[0].candidate.Value != "repo-00" {
		t.Fatalf("expected scrolling to preserve original order, got %#v", model.filtered[:4])
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

func TestPickerModelSearchUsesOnlyVisibleFields(t *testing.T) {
	model := newPickerModel("summon", []wsfold.CompletionCandidate{
		{Value: "service", Name: "service", Slug: "acme/service", Source: wsfold.CompletionSourceLocal},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	model = updated.(pickerModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	model = updated.(pickerModel)

	if len(model.filtered) != 0 {
		t.Fatalf("expected source marker to be excluded from search, got %#v", model.filtered)
	}
}

func TestPickerModelFiltersVeryLowScoreMatches(t *testing.T) {
	model := newPickerModel("summon", []wsfold.CompletionCandidate{
		{Value: "mikhail-yaskou/piskel", Name: "piskel", Slug: "mikhail-yaskou/piskel", Source: wsfold.CompletionSourceLocal},
		{Value: "mikhail-yaskou/vscode-as-mcp-server-with-approvals", Name: "vscode-as-mcp-server-with-approvals", Slug: "mikhail-yaskou/vscode-as-mcp-server-with-approvals", Source: wsfold.CompletionSourceLocal},
	})

	for _, r := range "piksel" {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated.(pickerModel)
	}

	if len(model.filtered) != 1 {
		t.Fatalf("expected low-score fuzzy match to be filtered out, got %#v", model.filtered)
	}
	if model.filtered[0].candidate.Name != "piskel" {
		t.Fatalf("expected relevant fuzzy match to remain, got %#v", model.filtered[0].candidate)
	}
}

func TestPickerModelSupportsPageNavigation(t *testing.T) {
	candidates := make([]wsfold.CompletionCandidate, 0, 30)
	for i := 0; i < 30; i++ {
		candidates = append(candidates, wsfold.CompletionCandidate{
			Value: fmt.Sprintf("repo-%02d", i),
		})
	}

	model := newPickerModel("summon", candidates)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = updated.(pickerModel)
	if model.cursor != 20 {
		t.Fatalf("expected page down to jump to row 20, got %d", model.cursor)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	model = updated.(pickerModel)
	if model.cursor != 0 {
		t.Fatalf("expected page up to jump back to top, got %d", model.cursor)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	model = updated.(pickerModel)
	if model.cursor != 20 {
		t.Fatalf("expected ctrl+f to page down, got %d", model.cursor)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	model = updated.(pickerModel)
	if model.cursor != 0 {
		t.Fatalf("expected ctrl+b to page up, got %d", model.cursor)
	}
}

func TestPickerModelShowsTwentyVisibleRows(t *testing.T) {
	candidates := make([]wsfold.CompletionCandidate, 0, 25)
	for i := 0; i < 25; i++ {
		candidates = append(candidates, wsfold.CompletionCandidate{
			Value: fmt.Sprintf("repo-%02d", i),
		})
	}

	model := newPickerModel("summon", candidates)
	view := stripANSI(model.View())

	rowCount := 0
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "repo-") {
			rowCount++
		}
	}
	if rowCount != 20 {
		t.Fatalf("expected 20 visible rows, got %d\n%s", rowCount, view)
	}
	if !strings.Contains(view, "Showing 1-20 of 25") {
		t.Fatalf("expected pagination indicator, got:\n%s", view)
	}
	if !strings.Contains(view, "Enter select, Space multi-select, Esc cancel") {
		t.Fatalf("expected single-select hint, got:\n%s", view)
	}
	if strings.Contains(view, "PgUp/PgDn") || strings.Contains(view, "fuzzy filter") {
		t.Fatalf("did not expect paging or fuzzy-filter hint in view, got:\n%s", view)
	}
}

func TestPickerModelUsesMultiSelectHintWhenSelectionsExist(t *testing.T) {
	model := newPickerModel("summon", []wsfold.CompletionCandidate{
		{Value: "alpha", Name: "alpha", Attached: true},
		{Value: "beta", Name: "beta"},
	})

	view := stripANSI(model.View())
	if !strings.Contains(view, "Space toggle, Enter apply, Esc cancel") {
		t.Fatalf("expected multi-select hint for preselected summon picker, got:\n%s", view)
	}
	if !strings.Contains(view, "Choose a trusted repository to include in your workspace [Multi mode]") {
		t.Fatalf("expected multi-select title badge for preselected summon picker, got:\n%s", view)
	}
	if !strings.Contains(view, "Selected (1)") {
		t.Fatalf("expected selected section for preselected summon picker, got:\n%s", view)
	}
}

func TestPickerModelUsesNoEnterHintForEmptyMultiSelect(t *testing.T) {
	model := newPickerModel("dismiss", []wsfold.CompletionCandidate{
		{Value: "alpha", Name: "alpha"},
		{Value: "beta", Name: "beta"},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = updated.(pickerModel)

	view := stripANSI(model.View())
	if !strings.Contains(view, "Space toggle, Esc cancel") {
		t.Fatalf("expected empty multi-select hint, got:\n%s", view)
	}
	if strings.Contains(view, "Enter apply") {
		t.Fatalf("did not expect Enter apply hint without selections, got:\n%s", view)
	}
}

func stripANSI(text string) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(text, "")
}
