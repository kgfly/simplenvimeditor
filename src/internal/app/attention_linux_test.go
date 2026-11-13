//go:build linux

package editorapp

import (
	"testing"

	gioapp "gioui.org/app"
)

// TestAttentionSetViewTracksTheWindow checks a request needs a live window,
// and that a window recreated after one was raised can be raised again.
func TestAttentionSetViewTracksTheWindow(t *testing.T) {
	requests, _ := recordAttention(t)
	a := attention{enabled: true}
	a.setView(gioapp.X11ViewEvent{Window: 1})
	a.bell()
	if *requests != 1 || a.armed() {
		t.Fatalf("requests = %d, armed = %v after a bell; want 1, false", *requests, a.armed())
	}
	a.setView(gioapp.X11ViewEvent{})
	a.bell()
	if *requests != 1 || a.armed() {
		t.Fatalf("requests = %d, armed = %v without a window; want 1, false", *requests, a.armed())
	}
	a.setView(gioapp.X11ViewEvent{Window: 2})
	if !a.armed() {
		t.Fatal("not armed for a new window")
	}
}

// TestAttentionHooksSkipMissingHandles guards the native calls against the
// empty handles Gio sends when a window goes away, and against an X server
// that can't be reached.
func TestAttentionHooksSkipMissingHandles(t *testing.T) {
	t.Setenv("DISPLAY", "")
	for _, view := range []gioapp.ViewEvent{
		gioapp.X11ViewEvent{},
		gioapp.WaylandViewEvent{},
		gioapp.X11ViewEvent{Window: 1},
	} {
		requestAttention(view)
		clearAttention(view)
	}
}
