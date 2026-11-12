//go:build windows

package editorapp

import (
	"unsafe"

	gioapp "gioui.org/app"
)

var procFlashWindowEx = user32.NewProc("FlashWindowEx")

const (
	flashwTray       = 0x00000002
	flashwTimerNoFG  = 0x0000000C
	flashwStop       = 0x00000000
	flashwTrayNoFG   = flashwTray | flashwTimerNoFG
	flashwCountUntil = 0 // with FLASHW_TIMERNOFG: until the window is foreground
)

// flashWInfo mirrors FLASHWINFO.
type flashWInfo struct {
	cbSize    uint32
	hwnd      uintptr
	dwFlags   uint32
	uCount    uint32
	dwTimeout uint32
}

func flashWindow(view gioapp.ViewEvent, flags uint32) {
	ev, ok := view.(gioapp.Win32ViewEvent)
	if !ok || ev.HWND == 0 {
		return
	}
	info := flashWInfo{
		hwnd:    ev.HWND,
		dwFlags: flags,
		uCount:  flashwCountUntil,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	procFlashWindowEx.Call(uintptr(unsafe.Pointer(&info)))
}

// requestAttention flashes the taskbar button a few times; it then stays
// highlighted until the window comes to the foreground, when Windows
// clears it.
func requestAttention(view gioapp.ViewEvent) { flashWindow(view, flashwTrayNoFG) }

// clearAttention stops the flash in case focus arrived without the window
// becoming the foreground one.
func clearAttention(view gioapp.ViewEvent) { flashWindow(view, flashwStop) }
