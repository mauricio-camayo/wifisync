package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type NetworkEntry struct {
	Label        string    `json:"label"`
	SSID         string    `json:"ssid"`
	BSSID        string    `json:"bssid"`
	IntervalDays int       `json:"interval_days"`
	LastSyncTime time.Time `json:"last_sync_time"`
}

type Config struct {
	mu sync.RWMutex // unexported, excluded from JSON automatically

	TrustedNetworks        []NetworkEntry `json:"trusted_networks"`
	SourceFolder           string         `json:"source_folder"`
	DestFolder             string         `json:"dest_folder"`
	ShutdownAfterSync      bool           `json:"shutdown_after_sync"`
	NotifyIfOverdue        bool           `json:"notify_if_overdue"`
	GracePeriodDays        int            `json:"grace_period_days"`
	PollingIntervalSecs    int            `json:"polling_interval_secs"`
	PerFileCopyTimeoutMins int            `json:"per_file_copy_timeout_mins"`
}

func (c *Config) Lock()    { c.mu.Lock() }
func (c *Config) Unlock()  { c.mu.Unlock() }
func (c *Config) RLock()   { c.mu.RLock() }
func (c *Config) RUnlock() { c.mu.RUnlock() }

func DefaultConfig() *Config {
	return &Config{
		TrustedNetworks:        []NetworkEntry{},
		GracePeriodDays:        3,
		PollingIntervalSecs:    60,
		PerFileCopyTimeoutMins: 5,
	}
}

// Path returns the platform config file path.
// On Windows this is %APPDATA%\WifiSync\config.json;
// on other platforms (dev) it falls back to the temp dir.
func Path() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "WifiSync", "config.json")
}

// Load reads the config file at path. If the file does not exist it returns
// a default config (first-run case). Missing or zero-value fields are filled
// with defaults after unmarshalling.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	applyDefaults(cfg)
	return cfg, nil
}

// Save writes cfg to path atomically via a temp file + rename.
// The parent directory is created if it does not exist.
func Save(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	cfg.RLock()
	data, err := json.MarshalIndent(cfg, "", "  ")
	cfg.RUnlock()
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func applyDefaults(cfg *Config) {
	if cfg.PollingIntervalSecs == 0 {
		cfg.PollingIntervalSecs = 60
	}
	if cfg.PerFileCopyTimeoutMins == 0 {
		cfg.PerFileCopyTimeoutMins = 5
	}
	if cfg.GracePeriodDays == 0 {
		cfg.GracePeriodDays = 3
	}
	if cfg.TrustedNetworks == nil {
		cfg.TrustedNetworks = []NetworkEntry{}
	}
}

// SyncDue reports whether the given network entry is due for a sync.
func (n *NetworkEntry) SyncDue() bool {
	if n.IntervalDays <= 0 {
		return true
	}
	return time.Since(n.LastSyncTime) >= time.Duration(n.IntervalDays)*24*time.Hour
}
