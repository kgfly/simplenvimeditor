package editorapp

import (
	"sync"
	"time"

	gioapp "gioui.org/app"
)

// attention turns Nvim's bell into an OS request for the user's attention
// (editor.attention_on_bell): a Dock bounce and badge on macOS, a flashing
// taskbar button on Windows, an urgent window on Linux. It only fires while
// the window is unfocused, and whatever the OS leaves behind is cleared once
// the window is focused again.
//
// Bells arrive on pumpRedraw's goroutine while focus and the native view
// change on the window's event loop, hence the lock. The platform hooks
// (requestAttention, clearAttention) are safe to call from any goroutine.
type attention struct {
	mu      sync.Mutex
	enabled bool
	focused bool
	raised  bool
	view    gioapp.ViewEvent
	valid   bool // view.Valid() when it was recorded
}

// setView records the native window the next request should target.
func (t *attention) setView(view gioapp.ViewEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.view = view
	t.valid = view.Valid()
	if !t.valid {
		t.raised = false
	}
}

// setFocused tracks window focus, clearing an outstanding request on focus.
func (t *attention) setFocused(focused bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.focused = focused
	if focused && t.raised {
		t.raised = false
		attentionClear(t.view)
	}
}

// bell requests attention if the feature is on and the window isn't focused.
func (t *attention) bell() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.enabled || t.focused || !t.valid {
		return
	}
	t.raised = true
	attentionRequest(t.view)
}

// armed reports whether a bell arriving now would request attention.
func (t *attention) armed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.enabled && !t.focused && t.valid && !t.raised
}

// The platform hooks, as variables so tests can observe them.
var (
	attentionRequest = requestAttention
	attentionClear   = clearAttention
)

// bellFlushInterval is how often flushBells asks Nvim for held-back events.
const bellFlushInterval = 500 * time.Millisecond

// flushBells delivers bells Nvim would otherwise sit on. Nvim queues a
// :terminal bell with the other UI events and only sends it on the next
// screen update, so a lone `\a` from an agent's stop hook can wait until
// the user comes back, which defeats the point. While a bell would raise
// attention, Nvim is asked to flush every bellFlushInterval.
func (a *App) flushBells() {
	tick := time.NewTicker(bellFlushInterval)
	defer tick.Stop()
	for {
		select {
		case <-a.proc.Exited:
			return
		case <-tick.C:
			if a.attention.armed() {
				a.proc.FlushUI()
			}
		}
	}
}
