//go:build darwin

package editorapp

/*
#cgo CFLAGS: -x objective-c -fmodules -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

// Declarations only; defined in attention_darwin.c.
void snv_request_attention(void);
void snv_clear_attention(void);
*/
import "C"

import gioapp "gioui.org/app"

// requestAttention bounces the Dock icon once and badges it. The app has a
// single window, so the request is application-wide.
func requestAttention(gioapp.ViewEvent) { C.snv_request_attention() }

// clearAttention removes the Dock badge; the bounce stops by itself.
func clearAttention(gioapp.ViewEvent) { C.snv_clear_attention() }
