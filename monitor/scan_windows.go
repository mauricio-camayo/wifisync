//go:build windows

package monitor

import (
	"os/exec"
	"strings"
	"syscall"
)

// ScanNetworks returns all visible Wi-Fi access points.
// Multiple BSSIDs for the same SSID are returned as separate entries.
func ScanNetworks() []WifiInfo {
	cmd := exec.Command("netsh", "wlan", "show", "networks", "mode=bssid")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var results []WifiInfo
	var currentSSID string

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, " : ")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+3:])

		switch {
		case strings.HasPrefix(key, "SSID "):
			currentSSID = val
		case strings.HasPrefix(key, "BSSID "):
			if currentSSID != "" {
				results = append(results, WifiInfo{
					SSID:  currentSSID,
					BSSID: strings.ToLower(val),
				})
			}
		}
	}
	return results
}
