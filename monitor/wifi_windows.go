//go:build windows

package monitor

import (
	"os/exec"
	"strings"
	"syscall"
)

// getWifiInfo reads the active Wi-Fi connection from netsh on Windows.
func getWifiInfo() WifiInfo {
	cmd := exec.Command("netsh", "wlan", "show", "interfaces")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return WifiInfo{}
	}
	var info WifiInfo
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// netsh lines have the form:  Key                   : value
		// Split on " : " to avoid mishandling colons in the BSSID value.
		idx := strings.Index(line, " : ")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+3:])
		switch key {
		case "SSID":
			info.SSID = val
		case "BSSID":
			info.BSSID = strings.ToLower(val)
		}
	}
	return info
}
