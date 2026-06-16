package ui

import "fyne.io/fyne/v2"

// TrayIcons holds the four tray icon states. Pass a zero value (or omit) to
// disable tray icon updates entirely.
type TrayIcons struct {
	Idle    fyne.Resource
	Ready   fyne.Resource
	Warning fyne.Resource
	Running []fyne.Resource // frames cycled while a sync is in progress
}
