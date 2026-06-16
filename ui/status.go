package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"wifisync/config"
	"wifisync/logger"
	"wifisync/monitor"
	"wifisync/syncer"
)

const logTailSize = 50

// StatusWindow manages the content of the WifiSync main window.
type StatusWindow struct {
	cfg         *config.Config
	cfgPath     string
	log         *logger.Logger
	mon         *monitor.Monitor
	syn         *syncer.Syncer
	notify      func(title, content string) // may be nil
	setTrayIcon func(fyne.Resource)         // may be nil
	trayIcons   TrayIcons

	win fyne.Window

	mu          sync.Mutex
	lastEvent   monitor.Event
	syncRunning bool
	syncCancel  context.CancelFunc
	animCancel  context.CancelFunc
	shownBSSIDs map[string]bool
	logEntries  []logger.LogEntry

	connectionLabel *widget.Label
	lastSyncLabel   *widget.Label
	nextSyncLabel   *widget.Label
	syncNowBtn      *widget.Button
	logList         *widget.List
}

func NewStatusWindow(cfg *config.Config, cfgPath string, log *logger.Logger, mon *monitor.Monitor, syn *syncer.Syncer, notify func(title, content string), setTrayIcon func(fyne.Resource), icons TrayIcons) *StatusWindow {
	return &StatusWindow{
		cfg:         cfg,
		cfgPath:     cfgPath,
		log:         log,
		mon:         mon,
		syn:         syn,
		notify:      notify,
		setTrayIcon: setTrayIcon,
		trayIcons:   icons,
		shownBSSIDs: make(map[string]bool),
	}
}

// Build constructs and returns the window content.
// openSettings is called when the user presses "Open Settings".
func (sw *StatusWindow) Build(win fyne.Window, openSettings func()) fyne.CanvasObject {
	sw.win = win
	sw.connectionLabel = widget.NewLabel("Checking Wi-Fi...")
	sw.lastSyncLabel = widget.NewLabel("Never")
	sw.nextSyncLabel = widget.NewLabel("—")
	sw.syncNowBtn = widget.NewButton("Sync Now", sw.TriggerSync)
	settingsBtn := widget.NewButton("Open Settings", openSettings)

	sw.logList = widget.NewList(
		func() int {
			sw.mu.Lock()
			defer sw.mu.Unlock()
			return len(sw.logEntries)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			sw.mu.Lock()
			if id >= len(sw.logEntries) {
				sw.mu.Unlock()
				return
			}
			text := formatLogEntry(sw.logEntries[id])
			sw.mu.Unlock()
			obj.(*widget.Label).SetText(text)
		},
	)

	statusGrid := container.NewGridWithColumns(2,
		widget.NewLabel("Wi-Fi status:"), sw.connectionLabel,
		widget.NewLabel("Last sync:"), sw.lastSyncLabel,
		widget.NewLabel("Next sync:"), sw.nextSyncLabel,
	)

	top := container.NewVBox(
		widget.NewCard("Status", "", statusGrid),
		container.NewHBox(sw.syncNowBtn, settingsBtn),
		widget.NewSeparator(),
		widget.NewLabel("Activity Log"),
	)

	logScroll := container.NewVScroll(sw.logList)
	logScroll.SetMinSize(fyne.NewSize(600, 200))

	RegisterShutdownInterceptor(win, sw)

	return container.NewBorder(top, nil, nil, nil, logScroll)
}

// IsSyncRunning reports whether a sync is currently in progress.
func (sw *StatusWindow) IsSyncRunning() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.syncRunning
}

// CancelSync cancels the currently running sync, if any.
func (sw *StatusWindow) CancelSync() {
	sw.mu.Lock()
	cancel := sw.syncCancel
	sw.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Start loads the initial log and launches the monitor event loop.
func (sw *StatusWindow) Start(events <-chan monitor.Event) {
	sw.refreshLog()
	sw.updateLastSync()
	go func() {
		for event := range events {
			sw.mu.Lock()
			sw.lastEvent = event
			sw.mu.Unlock()
			sw.applyEvent(event)
		}
	}()
}

// TriggerSync starts a manual sync. No-op if a sync is already running or if
// Build() has not been called yet (e.g. tray click during first-run setup).
func (sw *StatusWindow) TriggerSync() {
	sw.mu.Lock()
	if sw.syncNowBtn == nil || sw.syncRunning {
		sw.mu.Unlock()
		return
	}
	sw.syncRunning = true
	event := sw.lastEvent
	sw.mu.Unlock()

	sw.syncNowBtn.Disable()

	go func() {
		defer func() {
			sw.mu.Lock()
			sw.syncRunning = false
			sw.mu.Unlock()
			sw.syncNowBtn.Enable()
		}()

		var network *config.NetworkEntry
		if event.Network != nil {
			network = event.Network
		} else {
			sw.cfg.RLock()
			if len(sw.cfg.TrustedNetworks) > 0 {
				network = &sw.cfg.TrustedNetworks[0]
			}
			sw.cfg.RUnlock()
		}
		if network == nil {
			network = &config.NetworkEntry{Label: "Manual"}
		}
		sw.runSync(network, false)
	}()
}

func (sw *StatusWindow) applyEvent(event monitor.Event) {
	switch event.Type {
	case monitor.EventNoTrustedNetwork:
		sw.connectionLabel.SetText("Not connected to a trusted network")
		sw.nextSyncLabel.SetText("—")
		sw.updateTrayIcon(sw.trayIcons.Idle)

	case monitor.EventTrustedConnected:
		n := event.Network
		sw.connectionLabel.SetText(fmt.Sprintf("Connected — %s (trusted)", n.SSID))
		due := n.LastSyncTime.Add(time.Duration(n.IntervalDays) * 24 * time.Hour)
		sw.nextSyncLabel.SetText(fmt.Sprintf("in %s", formatDuration(time.Until(due))))
		sw.updateTrayIcon(sw.trayIcons.Ready)

	case monitor.EventSyncEligible:
		n := event.Network
		sw.connectionLabel.SetText(fmt.Sprintf("Connected — %s (sync due)", n.SSID))
		sw.nextSyncLabel.SetText("Starting...")
		sw.updateTrayIcon(sw.trayIcons.Ready)
		sw.mu.Lock()
		alreadyRunning := sw.syncRunning
		if !alreadyRunning {
			sw.syncRunning = true
		}
		sw.mu.Unlock()
		if !alreadyRunning {
			sw.syncNowBtn.Disable()
			go func() {
				defer func() {
					sw.mu.Lock()
					sw.syncRunning = false
					sw.mu.Unlock()
					sw.syncNowBtn.Enable()
				}()
				sw.runSync(n, true)
			}()
		}

	case monitor.EventOverdue:
		sw.connectionLabel.SetText("Not connected to a trusted network")
		sw.nextSyncLabel.SetText("—")
		sw.updateTrayIcon(sw.trayIcons.Warning)
		if sw.notify != nil {
			var msg string
			if event.DaysSince == 0 {
				msg = "Backup overdue — no sync has run yet."
			} else {
				msg = fmt.Sprintf("Backup overdue — last sync was %d days ago.", event.DaysSince)
			}
			sw.notify("WifiSync", msg)
		}

	case monitor.EventUnknownAP:
		sw.connectionLabel.SetText(fmt.Sprintf("Unknown access point — %s (%s)", event.SSID, event.BSSID))
		sw.nextSyncLabel.SetText("—")
		sw.updateTrayIcon(sw.trayIcons.Idle)
		sw.mu.Lock()
		seen := sw.shownBSSIDs[event.BSSID]
		if !seen {
			sw.shownBSSIDs[event.BSSID] = true
		}
		sw.mu.Unlock()
		if !seen {
			go sw.showUnknownAPDialog(event.SSID, event.BSSID)
		}
	}

	sw.updateLastSync()
}

func (sw *StatusWindow) runSync(network *config.NetworkEntry, isAutoSync bool) {
	ctx, cancel := context.WithCancel(context.Background())
	sw.mu.Lock()
	sw.syncCancel = cancel
	sw.mu.Unlock()
	defer func() {
		cancel()
		sw.mu.Lock()
		sw.syncCancel = nil
		sw.mu.Unlock()
	}()

	sw.startTrayAnimation()

	sw.log.Log(logger.LogEntry{
		Event:        logger.EventSyncStarted,
		NetworkLabel: network.Label,
	})

	result, err := sw.syn.Run(ctx, network, nil)

	sw.stopTrayAnimation(err == nil && !result.Cancelled)

	var entry logger.LogEntry
	syncSucceeded := false
	switch {
	case err != nil:
		entry = logger.LogEntry{
			Event:        logger.EventSyncFailed,
			NetworkLabel: network.Label,
			ErrorMessage: err.Error(),
		}
	case result.Cancelled:
		entry = logger.LogEntry{
			Event:            logger.EventSyncCancelled,
			NetworkLabel:     network.Label,
			FilesCopied:      result.FilesCopied,
			FilesSkipped:     result.FilesSkipped,
			BytesTransferred: result.BytesTransferred,
			DurationMs:       result.Duration.Milliseconds(),
		}
	default:
		entry = logger.LogEntry{
			Event:            logger.EventSyncComplete,
			NetworkLabel:     network.Label,
			FilesCopied:      result.FilesCopied,
			FilesSkipped:     result.FilesSkipped,
			BytesTransferred: result.BytesTransferred,
			DurationMs:       result.Duration.Milliseconds(),
		}
		config.Save(sw.cfg, sw.cfgPath)
		syncSucceeded = true
	}

	if len(result.Errors) > 0 {
		entry.ErrorMessage = fmt.Sprintf("%d file error(s): %s", len(result.Errors), result.Errors[0])
	}

	sw.log.Log(entry)
	sw.refreshLog()
	sw.updateLastSync()

	sw.cfg.RLock()
	shutdownAfterSync := sw.cfg.ShutdownAfterSync
	sw.cfg.RUnlock()
	if isAutoSync && syncSucceeded && shutdownAfterSync {
		showShutdownCountdown(sw.win, shutdownSystem)
	}
}

func (sw *StatusWindow) updateTrayIcon(r fyne.Resource) {
	if sw.setTrayIcon != nil && r != nil {
		sw.setTrayIcon(r)
	}
}

func (sw *StatusWindow) startTrayAnimation() {
	if sw.setTrayIcon == nil || len(sw.trayIcons.Running) == 0 {
		return
	}
	animCtx, cancel := context.WithCancel(context.Background())
	sw.mu.Lock()
	if sw.animCancel != nil {
		sw.animCancel()
	}
	sw.animCancel = cancel
	sw.mu.Unlock()

	frames := sw.trayIcons.Running
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-animCtx.Done():
				return
			case <-ticker.C:
				sw.setTrayIcon(frames[i%len(frames)])
				i++
			}
		}
	}()
}

func (sw *StatusWindow) stopTrayAnimation(succeeded bool) {
	sw.mu.Lock()
	cancel := sw.animCancel
	sw.animCancel = nil
	sw.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if succeeded {
		sw.updateTrayIcon(sw.trayIcons.Ready)
	} else {
		sw.updateTrayIcon(sw.trayIcons.Warning)
	}
}

func (sw *StatusWindow) showUnknownAPDialog(ssid, bssid string) {
	defaultInterval := 7
	sw.cfg.RLock()
	for _, n := range sw.cfg.TrustedNetworks {
		if n.SSID == ssid {
			defaultInterval = n.IntervalDays
			break
		}
	}
	sw.cfg.RUnlock()

	var dlg *dialog.CustomDialog
	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf(
			"SSID %q was seen from BSSID %s,\nwhich does not match any trusted access point.\nNo sync will run.",
			ssid, bssid,
		)),
		container.NewHBox(
			widget.NewButton("Add this access point", func() {
				dlg.Hide()
				showNetworkDialog("Add Network", config.NetworkEntry{
					SSID:         ssid,
					BSSID:        bssid,
					IntervalDays: defaultInterval,
				}, sw.win, func(entry config.NetworkEntry) {
					sw.cfg.Lock()
					sw.cfg.TrustedNetworks = append(sw.cfg.TrustedNetworks, entry)
					sw.cfg.Unlock()
					config.Save(sw.cfg, sw.cfgPath)
				})
			}),
			widget.NewButton("Ignore", func() {
				sw.mon.IgnoreBSSID(bssid)
				dlg.Hide()
			}),
		),
	)
	dlg = dialog.NewCustomWithoutButtons("Unknown Access Point", content, sw.win)
	dlg.Show()
}

// HasInterruptedSync reports whether the last sync event in the log indicates
// a run that did not complete cleanly: either the app was killed mid-sync
// (last entry is sync_started with no matching completion) or the sync was
// explicitly cancelled.
func (sw *StatusWindow) HasInterruptedSync() bool {
	entries, _ := sw.log.Tail(20)
	for i := len(entries) - 1; i >= 0; i-- {
		switch entries[i].Event {
		case logger.EventSyncStarted, logger.EventSyncCancelled:
			return true
		case logger.EventSyncComplete, logger.EventSyncFailed:
			return false
		}
	}
	return false
}

// ShowResumePrompt asks the user whether to run the previously interrupted sync.
func (sw *StatusWindow) ShowResumePrompt() {
	dialog.ShowConfirm(
		"Incomplete Backup",
		"Your last backup was interrupted or cancelled.\nWould you like to run it now?",
		func(ok bool) {
			if ok {
				sw.TriggerSync()
			}
		},
		sw.win,
	)
}

func (sw *StatusWindow) refreshLog() {
	entries, _ := sw.log.Tail(logTailSize)
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	sw.mu.Lock()
	sw.logEntries = entries
	sw.mu.Unlock()
	sw.logList.Refresh()
}

func (sw *StatusWindow) updateLastSync() {
	entries, _ := sw.log.Tail(logTailSize)
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		ts := e.Timestamp.Format("2006-01-02 15:04")
		switch e.Event {
		case logger.EventSyncComplete:
			sw.lastSyncLabel.SetText(fmt.Sprintf("✓ %s — %d files, %s", ts, e.FilesCopied, formatBytes(e.BytesTransferred)))
			return
		case logger.EventSyncFailed:
			sw.lastSyncLabel.SetText(fmt.Sprintf("✗ %s — failed", ts))
			return
		case logger.EventSyncCancelled:
			sw.lastSyncLabel.SetText(fmt.Sprintf("⚠ %s — cancelled", ts))
			return
		}
	}
	sw.lastSyncLabel.SetText("Never")
}

func formatLogEntry(e logger.LogEntry) string {
	ts := e.Timestamp.Format("15:04:05")
	switch e.Event {
	case logger.EventSyncStarted:
		return fmt.Sprintf("%s  → Backup started on %s", ts, e.NetworkLabel)
	case logger.EventSyncComplete:
		return fmt.Sprintf("%s  ✓ Complete — %d copied, %d skipped, %s",
			ts, e.FilesCopied, e.FilesSkipped, formatBytes(e.BytesTransferred))
	case logger.EventSyncFailed:
		return fmt.Sprintf("%s  ✗ Failed — %s", ts, e.ErrorMessage)
	case logger.EventSyncCancelled:
		return fmt.Sprintf("%s  ⚠ Cancelled — %d files copied", ts, e.FilesCopied)
	}
	return fmt.Sprintf("%s  %s", ts, e.Event)
}

func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(1024), 0
	for n := b / 1024; n >= 1024; n /= 1024 {
		div *= 1024
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd", int(d.Hours())/24)
}
