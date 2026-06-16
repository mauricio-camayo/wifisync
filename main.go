package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"

	"wifisync/config"
	"wifisync/logger"
	"wifisync/monitor"
	"wifisync/syncer"
	"wifisync/ui"
)

const appID = "io.wifisync.app"
const appName = "WifiSync"

func isSetupComplete(cfg *config.Config) bool {
	return len(cfg.TrustedNetworks) > 0 &&
		cfg.SourceFolder != "" &&
		cfg.DestFolder != ""
}

func main() {
	maybeInstall()

	a := app.NewWithID(appID)

	idle, ready, warning, running := loadIcons()
	a.SetIcon(ready)

	cfgPath := config.Path()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// setTrayIcon is safe to call before the tray is set up; it no-ops on non-desktop.
	setTrayIcon := func(r fyne.Resource) {
		if desk, ok := a.(desktop.App); ok {
			desk.SetSystemTrayIcon(r)
		}
	}

	log := logger.New(logger.Path())
	mon := monitor.New(cfg)
	syn := syncer.New(cfg)
	sw := ui.NewStatusWindow(cfg, cfgPath, log, mon, syn,
		func(title, content string) {
			a.SendNotification(&fyne.Notification{Title: title, Content: content})
		},
		setTrayIcon,
		ui.TrayIcons{Idle: idle, Ready: ready, Warning: warning, Running: running},
	)

	defer mon.Stop()

	var settingsWin fyne.Window
	openSettings := func() {
		if settingsWin != nil {
			settingsWin.RequestFocus()
			return
		}
		settingsWin = ui.OpenSettingsWindow(a, cfg, cfgPath)
		settingsWin.SetOnClosed(func() { settingsWin = nil })
	}

	w := a.NewWindow(appName)
	w.Resize(fyne.NewSize(720, 540))
	w.SetCloseIntercept(func() { w.Hide() })
	w.SetMainMenu(fyne.NewMainMenu(
		fyne.NewMenu("Help",
			fyne.NewMenuItem("About WifiSync", func() {
				ui.ShowAboutDialog(a.Metadata().Version, w)
			}),
		),
	))

	if desk, ok := a.(desktop.App); ok {
		desk.SetSystemTrayIcon(idle)
		desk.SetSystemTrayMenu(fyne.NewMenu(appName,
			fyne.NewMenuItem("Open", func() {
				w.Show()
				w.RequestFocus()
			}),
			fyne.NewMenuItem("Sync Now", sw.TriggerSync),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("About", func() {
				w.Show()
				ui.ShowAboutDialog(a.Metadata().Version, w)
			}),
			fyne.NewMenuItem("Exit", func() {
				a.Quit()
			}),
		))
	}

	registerAutostart()

	// startNormal builds the status view and starts the monitor.
	// Called either at startup (setup complete) or after first-run setup finishes.
	startNormal := func() {
		w.SetContent(sw.Build(w, openSettings))
		events := mon.Start()
		sw.Start(events)
	}

	if isSetupComplete(cfg) {
		startNormal()
		if sw.HasInterruptedSync() {
			w.Show()
			// Delay so the Fyne event loop is running before the dialog appears.
			time.AfterFunc(300*time.Millisecond, sw.ShowResumePrompt)
		} else {
			w.Hide()
		}
	} else {
		w.SetContent(ui.BuildFirstRunPanel(cfg, cfgPath, w, startNormal))
		w.Show()
	}

	a.Run()
}
