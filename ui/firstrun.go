package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"wifisync/config"
)

// BuildFirstRunPanel returns the guided setup content for the main window.
// onComplete is called after the user clicks "Finish Setup" with all required
// fields filled. Callers should swap the window content to the status view.
func BuildFirstRunPanel(cfg *config.Config, cfgPath string, win fyne.Window, onComplete func()) fyne.CanvasObject {
	networkStatus := widget.NewLabel("")
	sourceStatus := widget.NewLabel("")
	destStatus := widget.NewLabel("")
	networkList := container.NewVBox()

	// finishBtn and checkReady reference each other; declare first, assign below.
	var finishBtn *widget.Button

	checkReady := func() {
		if finishBtn == nil {
			return
		}
		if len(cfg.TrustedNetworks) > 0 && cfg.SourceFolder != "" && cfg.DestFolder != "" {
			finishBtn.Enable()
		} else {
			finishBtn.Disable()
		}
	}

	refreshNetworks := func() {
		networkList.RemoveAll()
		for _, n := range cfg.TrustedNetworks {
			networkList.Add(widget.NewLabel(fmt.Sprintf("  • %s  (%s)", n.Label, n.SSID)))
		}
		networkList.Refresh()
		if len(cfg.TrustedNetworks) > 0 {
			networkStatus.SetText(fmt.Sprintf("✓  %d network(s) added", len(cfg.TrustedNetworks)))
		} else {
			networkStatus.SetText("⚠  No trusted networks configured")
		}
		checkReady()
	}

	// ── Step 1: Trusted network ───────────────────────────────────────

	addNetworkBtn := widget.NewButton("+ Add Network", func() {
		showNetworkDialog("Add Trusted Network", config.NetworkEntry{IntervalDays: 7}, win, func(entry config.NetworkEntry) {
			cfg.TrustedNetworks = append(cfg.TrustedNetworks, entry)
			refreshNetworks()
		})
	})

	// ── Step 2: Source folder ─────────────────────────────────────────

	sourceEntry := widget.NewEntry()
	sourceEntry.SetText(cfg.SourceFolder)
	sourceEntry.OnChanged = func(s string) {
		cfg.SourceFolder = strings.TrimSpace(s)
		if cfg.SourceFolder != "" {
			sourceStatus.SetText("✓  " + cfg.SourceFolder)
		} else {
			sourceStatus.SetText("⚠  Not configured")
		}
		checkReady()
	}
	sourceRow := container.NewBorder(nil, nil, nil,
		widget.NewButton("Browse…", func() {
			dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
				if err != nil || uri == nil {
					return
				}
				sourceEntry.SetText(filepath.FromSlash(uri.Path()))
			}, win)
		}),
		sourceEntry,
	)

	// ── Step 3: Destination folder ────────────────────────────────────

	destEntry := widget.NewEntry()
	destEntry.SetText(cfg.DestFolder)
	destEntry.SetPlaceHolder(`e.g. \\server\share\backup`)
	destEntry.OnChanged = func(s string) {
		cfg.DestFolder = strings.TrimSpace(s)
		if cfg.DestFolder != "" {
			destStatus.SetText("✓  " + cfg.DestFolder)
		} else {
			destStatus.SetText("⚠  Not configured")
		}
		checkReady()
	}
	destRow := container.NewBorder(nil, nil, nil,
		widget.NewButton("Browse…", func() {
			dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
				if err != nil || uri == nil {
					return
				}
				destEntry.SetText(filepath.FromSlash(uri.Path()))
			}, win)
		}),
		destEntry,
	)

	// ── Step 4: Optional ─────────────────────────────────────────────

	shutdownCheck := widget.NewCheck("Shut down after each successful auto-sync", func(checked bool) {
		cfg.ShutdownAfterSync = checked
	})
	shutdownCheck.SetChecked(cfg.ShutdownAfterSync)

	// ── Finish button ─────────────────────────────────────────────────

	finishBtn = widget.NewButton("Finish Setup", func() {
		config.Save(cfg, cfgPath)
		onComplete()
	})
	finishBtn.Importance = widget.HighImportance

	// ── Initialise status labels from any pre-existing cfg values ─────

	refreshNetworks()
	if cfg.SourceFolder != "" {
		sourceStatus.SetText("✓  " + cfg.SourceFolder)
	} else {
		sourceStatus.SetText("⚠  Not configured")
	}
	if cfg.DestFolder != "" {
		destStatus.SetText("✓  " + cfg.DestFolder)
	} else {
		destStatus.SetText("⚠  Not configured")
	}
	checkReady()

	// ── Layout ────────────────────────────────────────────────────────

	heading := boldLabel("Welcome to WifiSync")
	heading.Alignment = fyne.TextAlignCenter

	step1 := widget.NewCard("Step 1 — Trusted Network",
		"WifiSync backs up only when connected to a network you trust.",
		container.NewVBox(networkStatus, networkList, addNetworkBtn),
	)
	step2 := widget.NewCard("Step 2 — Source Folder",
		"The folder on this computer to back up.",
		container.NewVBox(sourceRow, sourceStatus),
	)
	step3 := widget.NewCard("Step 3 — Destination Folder",
		"The network folder where backups will be stored.",
		container.NewVBox(destRow, destStatus),
	)
	step4 := widget.NewCard("Step 4 — Options (optional)", "",
		shutdownCheck,
	)

	return container.NewVScroll(container.NewVBox(
		container.NewPadded(heading),
		widget.NewLabel("Complete the steps below to start automatic backups."),
		widget.NewSeparator(),
		step1, step2, step3, step4,
		container.NewCenter(container.NewPadded(finishBtn)),
	))
}
