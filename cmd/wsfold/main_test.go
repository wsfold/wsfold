package main

import (
	"errors"
	"testing"
)

func TestFormatCLIErrorAddsCommonMarker(t *testing.T) {
	got := formatCLIError(errors.New(`repo ref "sdf" is not classified as trusted`))
	wantPrefix := ansiRed + ansiBold + "✗" + ansiReset + " Error: "
	if got != wantPrefix+`repo ref "sdf" is not classified as trusted` {
		t.Fatalf("unexpected formatted error: %q", got)
	}
}

func TestFormatCLIErrorDoesNotDuplicateExistingMarker(t *testing.T) {
	input := errors.New(ansiRed + ansiBold + "✗" + ansiReset + " repository is not part of the current workspace composition: dsa")
	got := formatCLIError(input)
	want := ansiRed + ansiBold + "✗" + ansiReset + " Error: repository is not part of the current workspace composition: dsa"
	if got != want {
		t.Fatalf("unexpected formatted error: %q", got)
	}
}
