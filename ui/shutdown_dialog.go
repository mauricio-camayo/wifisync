package ui

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const shutdownCountdownSecs = 30

// showShutdownCountdown shows a dialog counting down to system shutdown.
// onShutdown is called if the user does not cancel within shutdownCountdownSecs seconds.
func showShutdownCountdown(win fyne.Window, onShutdown func()) {
	ctx, cancel := context.WithCancel(context.Background())

	countLabel := widget.NewLabel(fmt.Sprintf("Shutting down in %d seconds...", shutdownCountdownSecs))

	dlg := dialog.NewCustom(
		"Backup Complete",
		"Cancel",
		container.NewCenter(countLabel),
		win,
	)
	dlg.SetOnClosed(cancel)
	dlg.Show()

	go func() {
		defer cancel()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for remaining := shutdownCountdownSecs; remaining > 0; remaining-- {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				countLabel.SetText(fmt.Sprintf("Shutting down in %d seconds...", remaining-1))
			}
		}
		if ctx.Err() == nil {
			dlg.Hide()
			onShutdown()
		}
	}()
}
