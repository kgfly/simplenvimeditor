//go:build linux

// Urgency for X11 windows (editor.attention_on_bell).
//
// The requests come from Go goroutines while Gio owns its Xlib connection on
// another thread, so this keeps a private connection, as drop_x11.c does.
// Errors on it (say, the window vanished between the request and the call)
// are swallowed instead of reaching Xlib's default handler, which exits.
//
// Both the ICCCM urgency hint and EWMH _NET_WM_STATE_DEMANDS_ATTENTION are
// set, because panels and window managers differ in which one they honour.
// Both are removed again when the window is focused.

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <string.h>

static Display *snv_attn_dpy;
static XErrorHandler snv_attn_prev_handler;

static int snv_attn_error(Display *dpy, XErrorEvent *err) {
    if (dpy == snv_attn_dpy) {
        return 0;
    }
    if (snv_attn_prev_handler != NULL) {
        return snv_attn_prev_handler(dpy, err);
    }
    return 0;
}

// snv_x11_set_attention is called with a Go mutex held, so the lazily opened
// connection is never used from two threads at once.
void snv_x11_set_attention(unsigned long window, int on) {
    if (snv_attn_dpy == NULL) {
        snv_attn_dpy = XOpenDisplay(NULL);
        if (snv_attn_dpy == NULL) {
            return;
        }
        snv_attn_prev_handler = XSetErrorHandler(snv_attn_error);
    }
    Display *d = snv_attn_dpy;
    Window w = (Window)window;

    XWMHints *hints = XGetWMHints(d, w);
    XWMHints empty;
    memset(&empty, 0, sizeof(empty));
    XWMHints *h = hints != NULL ? hints : &empty;
    if (on) {
        h->flags |= XUrgencyHint;
    } else {
        h->flags &= ~XUrgencyHint;
    }
    XSetWMHints(d, w, h);
    if (hints != NULL) {
        XFree(hints);
    }

    XEvent ev;
    memset(&ev, 0, sizeof(ev));
    ev.xclient.type = ClientMessage;
    ev.xclient.window = w;
    ev.xclient.message_type = XInternAtom(d, "_NET_WM_STATE", False);
    ev.xclient.format = 32;
    ev.xclient.data.l[0] = on ? 1 : 0; // _NET_WM_STATE_ADD / _REMOVE
    ev.xclient.data.l[1] = XInternAtom(d, "_NET_WM_STATE_DEMANDS_ATTENTION", False);
    ev.xclient.data.l[3] = 1; // source: normal application
    XSendEvent(d, DefaultRootWindow(d), False,
               SubstructureRedirectMask | SubstructureNotifyMask, &ev);
    XSync(d, False);
}
