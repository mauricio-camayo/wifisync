package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed images/idle.png
var _iconIdle []byte

//go:embed images/ready.png
var _iconReady []byte

//go:embed images/warning.png
var _iconWarning []byte

//go:embed images/icon/running-0.png
var _iconRun0 []byte

//go:embed images/icon/running-1.png
var _iconRun1 []byte

//go:embed images/icon/running-2.png
var _iconRun2 []byte

//go:embed images/icon/running-3.png
var _iconRun3 []byte

func loadIcons() (idle, ready, warning fyne.Resource, running []fyne.Resource) {
	idle = fyne.NewStaticResource("idle.png", _iconIdle)
	ready = fyne.NewStaticResource("ready.png", _iconReady)
	warning = fyne.NewStaticResource("warning.png", _iconWarning)
	running = []fyne.Resource{
		fyne.NewStaticResource("running-0.png", _iconRun0),
		fyne.NewStaticResource("running-1.png", _iconRun1),
		fyne.NewStaticResource("running-2.png", _iconRun2),
		fyne.NewStaticResource("running-3.png", _iconRun3),
	}
	return
}
