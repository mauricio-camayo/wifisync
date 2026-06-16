package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"wifisync/config"
)

// OpenSettingsWindow creates and shows the Settings window.
// It returns the window so the caller can track whether it is already open.
func OpenSettingsWindow(a fyne.App, cfg *config.Config, cfgPath string) fyne.Window {
	win := a.NewWindow("WifiSync — Settings")

	networksTab := buildNetworksContent(cfg, cfgPath, win)

	syncTab := buildSyncContent(cfg, cfgPath, win)

	tabs := container.NewAppTabs(
		container.NewTabItem("Networks", networksTab),
		container.NewTabItem("Sync", syncTab),
	)

	win.SetContent(tabs)
	win.Resize(fyne.NewSize(700, 480))
	win.Show()
	return win
}
