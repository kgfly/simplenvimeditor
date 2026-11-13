//go:build !darwin && !windows && !linux

package editorapp

import gioapp "gioui.org/app"

// Platforms without a native attention request of their own.
func requestAttention(gioapp.ViewEvent) {}
func clearAttention(gioapp.ViewEvent)   {}
