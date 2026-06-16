//go:build !windows

package monitor

// ScanNetworks returns nil on non-Windows platforms.
// The dialog will fall back to manual SSID/BSSID entry.
func ScanNetworks() []WifiInfo {
	return nil
}
