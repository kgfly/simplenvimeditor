//go:build linux

package editorapp

/*
#cgo pkg-config: x11 wayland-client

// Declarations only; defined in attention_x11.c and attention_wayland.c.
void snv_x11_set_attention(unsigned long window, int on);
void snv_wl_request_attention(void *display, void *surface);
*/
import "C"

import (
	"sync"

	gioapp "gioui.org/app"
)

// x11AttentionMu serializes use of attention_x11.c's private connection.
var x11AttentionMu sync.Mutex

// requestAttention marks the window urgent. How that shows depends on the
// desktop: a blinking or highlighted taskbar entry, or GNOME's "... is
// ready" notification.
func requestAttention(view gioapp.ViewEvent) {
	switch e := view.(type) {
	case gioapp.X11ViewEvent:
		setX11Attention(e, true)
	case gioapp.WaylandViewEvent:
		if e.Display != nil && e.Surface != nil {
			C.snv_wl_request_attention(e.Display, e.Surface)
		}
	}
}

// clearAttention drops X11's urgency hints; window managers are meant to on
// focus, but not all do. Wayland compositors clear theirs themselves.
func clearAttention(view gioapp.ViewEvent) {
	if e, ok := view.(gioapp.X11ViewEvent); ok {
		setX11Attention(e, false)
	}
}

func setX11Attention(e gioapp.X11ViewEvent, on bool) {
	if e.Window == 0 {
		return
	}
	flag := C.int(0)
	if on {
		flag = 1
	}
	x11AttentionMu.Lock()
	defer x11AttentionMu.Unlock()
	C.snv_x11_set_attention(C.ulong(e.Window), flag)
}
