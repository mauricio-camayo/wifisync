//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

func shutdownSystem() {
	cmd := exec.Command("shutdown", "/s", "/t", "0")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Run()
}
