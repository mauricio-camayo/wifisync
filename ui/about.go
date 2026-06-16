package ui

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func ShowAboutDialog(version string, win fyne.Window) {
	githubURL, _ := url.Parse("https://github.com/mauricio-camayo/wifisync")

	content := container.NewVBox(
		widget.NewRichTextFromMarkdown("# WifiSync v"+version),
		widget.NewLabel("Automatic Wi-Fi backup for Windows.\nSyncs your files to a network folder whenever\nyou connect to a trusted access point."),
		widget.NewSeparator(),
		widget.NewLabel("Author: Mauricio Camayo"),
		widget.NewHyperlink("github.com/mauricio-camayo/wifisync", githubURL),
	)

	dialog.NewCustom("About WifiSync", "Close", content, win).Show()
}
