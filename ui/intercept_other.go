//go:build !windows

package ui

import "fyne.io/fyne/v2"

func RegisterShutdownInterceptor(win fyne.Window, sw *StatusWindow) {}
