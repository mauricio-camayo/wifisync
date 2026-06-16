package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"wifisync/config"
	"wifisync/monitor"
)

func buildNetworksContent(cfg *config.Config, cfgPath string, win fyne.Window) fyne.CanvasObject {
	networkVBox := container.NewVBox()

	var refresh func()
	refresh = func() {
		networkVBox.RemoveAll()
		if len(cfg.TrustedNetworks) == 0 {
			networkVBox.Add(widget.NewLabel("No trusted networks configured yet."))
		}
		for i, n := range cfg.TrustedNetworks {
			i, n := i, n
			row := container.NewBorder(nil, nil, nil,
				container.NewHBox(
					widget.NewButton("Edit", func() {
						showNetworkDialog("Edit Network", n, win, func(updated config.NetworkEntry) {
							cfg.Lock()
							updated.LastSyncTime = cfg.TrustedNetworks[i].LastSyncTime
							cfg.TrustedNetworks[i] = updated
							cfg.Unlock()
							config.Save(cfg, cfgPath)
							refresh()
						})
					}),
					widget.NewButton("Remove", func() {
						confirmNetworkRemove(n.Label, win, func() {
							cfg.Lock()
							cfg.TrustedNetworks = append(
								cfg.TrustedNetworks[:i],
								cfg.TrustedNetworks[i+1:]...,
							)
							cfg.Unlock()
							config.Save(cfg, cfgPath)
							refresh()
						})
					}),
				),
				container.NewGridWithColumns(4,
					widget.NewLabel(n.Label),
					widget.NewLabel(n.SSID),
					widget.NewLabel(n.BSSID),
					widget.NewLabel(fmt.Sprintf("%d days", n.IntervalDays)),
				),
			)
			networkVBox.Add(row)
			if i < len(cfg.TrustedNetworks)-1 {
				networkVBox.Add(widget.NewSeparator())
			}
		}
		networkVBox.Refresh()
	}
	refresh()

	addBtn := widget.NewButton("+ Add Network", func() {
		defaultInterval := 7
		if len(cfg.TrustedNetworks) > 0 {
			defaultInterval = cfg.TrustedNetworks[0].IntervalDays
		}
		showNetworkDialog("Add Network", config.NetworkEntry{IntervalDays: defaultInterval}, win, func(entry config.NetworkEntry) {
			cfg.Lock()
			cfg.TrustedNetworks = append(cfg.TrustedNetworks, entry)
			cfg.Unlock()
			config.Save(cfg, cfgPath)
			refresh()
		})
	})

	header := container.NewGridWithColumns(4,
		boldLabel("Name"),
		boldLabel("SSID"),
		boldLabel("BSSID"),
		boldLabel("Interval"),
	)

	scroll := container.NewVScroll(networkVBox)
	scroll.SetMinSize(fyne.NewSize(0, 280))

	return container.NewBorder(
		container.NewVBox(header, widget.NewSeparator()),
		container.NewPadded(addBtn),
		nil, nil,
		scroll,
	)
}

func showNetworkDialog(title string, initial config.NetworkEntry, win fyne.Window, onSave func(config.NetworkEntry)) {
	scanned := monitor.ScanNetworks()

	labelEntry := widget.NewEntry()
	labelEntry.SetText(initial.Label)
	labelEntry.SetPlaceHolder("e.g. Home, Office")
	labelEntry.Validator = validation.NewRegexp(`.+`, "label cannot be empty")

	ssidEntry := widget.NewEntry()
	ssidEntry.SetText(initial.SSID)
	ssidEntry.SetPlaceHolder("Network name")

	bssidEntry := widget.NewEntry()
	bssidEntry.SetText(initial.BSSID)
	bssidEntry.SetPlaceHolder("aa:bb:cc:dd:ee:ff")

	days := initial.IntervalDays
	if days == 0 {
		days = 7
	}
	intervalEntry := widget.NewEntry()
	intervalEntry.SetText(strconv.Itoa(days))
	intervalEntry.Validator = validation.NewRegexp(`^[1-9][0-9]*$`, "must be a positive whole number")

	scanMap := make(map[string]monitor.WifiInfo)
	var scanOptions []string
	for _, n := range scanned {
		key := fmt.Sprintf("%s  (%s)", n.SSID, n.BSSID)
		scanOptions = append(scanOptions, key)
		scanMap[key] = n
	}

	var scanWidget fyne.CanvasObject
	if len(scanOptions) > 0 {
		sel := widget.NewSelect(scanOptions, func(selected string) {
			if info, ok := scanMap[selected]; ok {
				ssidEntry.SetText(info.SSID)
				bssidEntry.SetText(info.BSSID)
			}
		})
		sel.PlaceHolder = "Select to auto-fill SSID + BSSID…"
		scanWidget = sel
	} else {
		scanWidget = widget.NewLabel("No networks visible — enter SSID and BSSID manually")
	}

	items := []*widget.FormItem{
		{Text: "Label", Widget: labelEntry},
		{Text: "Scan", Widget: scanWidget},
		{Text: "SSID", Widget: ssidEntry},
		{Text: "BSSID", Widget: bssidEntry},
		{Text: "Interval (days)", Widget: intervalEntry},
	}

	dialog.ShowForm(title, "Save", "Cancel", items, func(ok bool) {
		if !ok {
			return
		}
		d, _ := strconv.Atoi(strings.TrimSpace(intervalEntry.Text))
		if d < 1 {
			d = 7
		}
		onSave(config.NetworkEntry{
			Label:        strings.TrimSpace(labelEntry.Text),
			SSID:         strings.TrimSpace(ssidEntry.Text),
			BSSID:        strings.ToLower(strings.TrimSpace(bssidEntry.Text)),
			IntervalDays: d,
		})
	}, win)
}

func confirmNetworkRemove(label string, win fyne.Window, onConfirm func()) {
	dialog.ShowConfirm(
		"Remove Network",
		fmt.Sprintf("Remove %q from trusted networks?", label),
		func(ok bool) {
			if ok {
				onConfirm()
			}
		},
		win,
	)
}

func boldLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: true}
	return l
}
