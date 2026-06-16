//go:build !windows

package monitor

// getWifiInfo returns empty on non-Windows platforms (dev/CI builds).
// The monitor will emit EventNoTrustedNetwork on every poll.
func getWifiInfo() WifiInfo {
	return WifiInfo{}
}
