package editorapp

import (
	"testing"

	gioapp "gioui.org/app"
)

// recordAttention swaps the platform hooks for counters.
func recordAttention(t *testing.T) (requests, clears *int) {
	t.Helper()
	requests, clears = new(int), new(int)
	oldRequest, oldClear := attentionRequest, attentionClear
	attentionRequest = func(gioapp.ViewEvent) { *requests++ }
	attentionClear = func(gioapp.ViewEvent) { *clears++ }
	t.Cleanup(func() { attentionRequest, attentionClear = oldRequest, oldClear })
	return requests, clears
}

func TestAttentionOnlyWhenEnabledAndUnfocused(t *testing.T) {
	for _, tc := range []struct {
		name             string
		enabled, focused bool
		want             int
	}{
		{"disabled", false, false, 0},
		{"focused", true, true, 0},
		{"unfocused", true, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests, _ := recordAttention(t)
			a := attention{enabled: tc.enabled, valid: true}
			a.setFocused(tc.focused)
			a.bell()
			if *requests != tc.want {
				t.Fatalf("requests = %d, want %d", *requests, tc.want)
			}
		})
	}
}

func TestAttentionArmedOnlyWhileABellWouldRaise(t *testing.T) {
	recordAttention(t)
	if (&attention{valid: true}).armed() {
		t.Fatal("armed while disabled")
	}
	a := attention{enabled: true, valid: true}
	a.setFocused(true)
	if a.armed() {
		t.Fatal("armed while focused")
	}
	a.setFocused(false)
	if !a.armed() {
		t.Fatal("not armed while unfocused")
	}
	a.bell()
	if a.armed() {
		t.Fatal("armed while attention is already raised")
	}
	a.setFocused(true)
	a.setFocused(false)
	if !a.armed() {
		t.Fatal("not armed after focus cleared the request")
	}
}

func TestAttentionNeedsAValidView(t *testing.T) {
	requests, _ := recordAttention(t)
	a := attention{enabled: true}
	a.bell()
	if *requests != 0 {
		t.Fatalf("requests = %d without a view, want 0", *requests)
	}
}

func TestAttentionClearedOnceOnFocus(t *testing.T) {
	requests, clears := recordAttention(t)
	a := attention{enabled: true, valid: true}

	a.setFocused(true)
	if *clears != 0 {
		t.Fatalf("clears = %d with nothing raised, want 0", *clears)
	}
	a.setFocused(false)
	a.bell()
	a.bell()
	a.setFocused(true)
	a.setFocused(false)
	a.setFocused(true)
	if *requests != 2 || *clears != 1 {
		t.Fatalf("requests, clears = %d, %d; want 2, 1", *requests, *clears)
	}
}
