package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"wifisync/config"
)

func buildSyncContent(cfg *config.Config, cfgPath string, win fyne.Window) fyne.CanvasObject {
	// Source folder
	sourceEntry := widget.NewEntry()
	sourceEntry.SetText(cfg.SourceFolder)
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

	// Destination folder (also accepts UNC paths typed manually)
	destEntry := widget.NewEntry()
	destEntry.SetText(cfg.DestFolder)
	destEntry.SetPlaceHolder(`e.g. D:\Backup  or  \\server\share\backup`)
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

	// Shutdown after sync
	shutdownCheck := widget.NewCheck("", nil)
	shutdownCheck.SetChecked(cfg.ShutdownAfterSync)

	// Grace period — enabled only when overdue notify is on
	graceDays := cfg.GracePeriodDays
	if graceDays == 0 {
		graceDays = 3
	}
	graceEntry := widget.NewEntry()
	graceEntry.SetText(strconv.Itoa(graceDays))
	if !cfg.NotifyIfOverdue {
		graceEntry.Disable()
	}

	overdueCheck := widget.NewCheck("", func(checked bool) {
		if checked {
			graceEntry.Enable()
		} else {
			graceEntry.Disable()
		}
	})
	overdueCheck.SetChecked(cfg.NotifyIfOverdue)

	// Polling interval and per-file timeout
	pollingEntry := widget.NewEntry()
	pollingEntry.SetText(strconv.Itoa(cfg.PollingIntervalSecs))

	timeoutEntry := widget.NewEntry()
	timeoutEntry.SetText(strconv.Itoa(cfg.PerFileCopyTimeoutMins))

	form := widget.NewForm(
		widget.NewFormItem("Source folder", sourceRow),
		widget.NewFormItem("Destination folder", destRow),
		widget.NewFormItem("Shutdown after sync", shutdownCheck),
		widget.NewFormItem("Notify if overdue", overdueCheck),
		widget.NewFormItem("Grace period (days)", graceEntry),
		widget.NewFormItem("Polling interval (s, 15–300)", pollingEntry),
		widget.NewFormItem("Per-file timeout (min, 1–60)", timeoutEntry),
	)

	saveBtn := widget.NewButton("Save Settings", func() {
		polling, err := parseRangeInt(pollingEntry.Text, 15, 300)
		if err != nil {
			dialog.ShowError(fmt.Errorf("polling interval: %w", err), win)
			return
		}
		timeout, err := parseRangeInt(timeoutEntry.Text, 1, 60)
		if err != nil {
			dialog.ShowError(fmt.Errorf("per-file timeout: %w", err), win)
			return
		}
		if overdueCheck.Checked {
			if _, err := parseRangeInt(graceEntry.Text, 1, 365); err != nil {
				dialog.ShowError(fmt.Errorf("grace period: %w", err), win)
				return
			}
		}

		cfg.Lock()
		cfg.SourceFolder = strings.TrimSpace(sourceEntry.Text)
		cfg.DestFolder = strings.TrimSpace(destEntry.Text)
		cfg.ShutdownAfterSync = shutdownCheck.Checked
		cfg.NotifyIfOverdue = overdueCheck.Checked
		cfg.PollingIntervalSecs = polling
		cfg.PerFileCopyTimeoutMins = timeout
		if overdueCheck.Checked {
			g, _ := parseRangeInt(graceEntry.Text, 1, 365)
			cfg.GracePeriodDays = g
		}
		cfg.Unlock()

		if err := config.Save(cfg, cfgPath); err != nil {
			dialog.ShowError(fmt.Errorf("save failed: %w", err), win)
			return
		}
		dialog.ShowInformation("Saved", "Sync settings saved successfully.", win)
	})

	return container.NewVScroll(
		container.NewVBox(form, container.NewPadded(saveBtn)),
	)
}

// parseRangeInt parses s as an integer and checks it falls within [min, max].
func parseRangeInt(s string, min, max int) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("must be a number")
	}
	if n < min || n > max {
		return 0, fmt.Errorf("must be between %d and %d", min, max)
	}
	return n, nil
}
