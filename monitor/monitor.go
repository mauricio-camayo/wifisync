package monitor

import (
	"sync"
	"time"

	"wifisync/config"
)

type EventType int

const (
	EventSyncEligible    EventType = iota // trusted network matched, interval elapsed
	EventTrustedConnected                 // trusted network matched, interval not yet elapsed
	EventUnknownAP                        // SSID matched but BSSID did not
	EventNoTrustedNetwork                 // no match at all
	EventOverdue                          // notify-if-overdue triggered; fired at most once per session
)

type Event struct {
	Type      EventType
	Network   *config.NetworkEntry // set for EventSyncEligible and EventTrustedConnected
	SSID      string               // set for EventUnknownAP
	BSSID     string               // set for EventUnknownAP
	DaysSince int                  // set for EventOverdue: days since the most recent sync (0 = never synced)
}

// WifiInfo holds the currently connected access point's identity.
type WifiInfo struct {
	SSID  string
	BSSID string // normalised to lowercase
}

type Monitor struct {
	cfg          *config.Config
	events       chan Event
	stop         chan struct{}
	ignored      map[string]bool // BSSIDs suppressed for this session
	overdueFired bool            // true once EventOverdue has been emitted this session
	mu           sync.Mutex
}

func New(cfg *config.Config) *Monitor {
	return &Monitor{
		cfg:     cfg,
		events:  make(chan Event, 16),
		stop:    make(chan struct{}),
		ignored: make(map[string]bool),
	}
}

// Start launches the polling goroutine and returns the event channel.
// The first poll runs immediately before the first tick.
func (m *Monitor) Start() <-chan Event {
	go m.loop()
	return m.events
}

// Stop shuts down the polling goroutine.
func (m *Monitor) Stop() {
	close(m.stop)
}

// IgnoreBSSID suppresses further EventUnknownAP events for the given BSSID
// for the remainder of the session.
func (m *Monitor) IgnoreBSSID(bssid string) {
	m.mu.Lock()
	m.ignored[bssid] = true
	m.mu.Unlock()
}

func (m *Monitor) loop() {
	m.cfg.RLock()
	interval := time.Duration(m.cfg.PollingIntervalSecs) * time.Second
	m.cfg.RUnlock()
	if interval <= 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	m.poll()
	for {
		select {
		case <-ticker.C:
			m.poll()
		case <-m.stop:
			return
		}
	}
}

func (m *Monitor) poll() {
	info := getWifiInfo()

	// RLock covers the full body so emitNoTrustedOrOverdue and checkOverdue
	// can read cfg fields without acquiring the lock a second time.
	m.cfg.RLock()
	defer m.cfg.RUnlock()

	if info.SSID == "" {
		m.emitNoTrustedOrOverdue()
		return
	}

	m.mu.Lock()
	ignored := make(map[string]bool, len(m.ignored))
	for k, v := range m.ignored {
		ignored[k] = v
	}
	m.mu.Unlock()

	var ssidOnlyMatch *config.NetworkEntry
	for i := range m.cfg.TrustedNetworks {
		n := &m.cfg.TrustedNetworks[i]
		if n.SSID != info.SSID {
			continue
		}
		if n.BSSID == info.BSSID {
			if n.SyncDue() {
				m.emit(Event{Type: EventSyncEligible, Network: n})
			} else {
				m.emit(Event{Type: EventTrustedConnected, Network: n})
			}
			return
		}
		// SSID matches but BSSID does not — keep looking for a full match first
		ssidOnlyMatch = n
	}

	if ssidOnlyMatch != nil {
		if !ignored[info.BSSID] {
			m.emit(Event{Type: EventUnknownAP, SSID: info.SSID, BSSID: info.BSSID})
		}
		return
	}

	m.emitNoTrustedOrOverdue()
}

// emitNoTrustedOrOverdue emits EventOverdue (once per session) if every trusted
// network is past its interval+grace window, otherwise emits EventNoTrustedNetwork.
func (m *Monitor) emitNoTrustedOrOverdue() {
	if m.cfg.NotifyIfOverdue && !m.overdueFired && len(m.cfg.TrustedNetworks) > 0 {
		if days, overdue := m.checkOverdue(); overdue {
			m.overdueFired = true
			m.emit(Event{Type: EventOverdue, DaysSince: days})
			return
		}
	}
	m.emit(Event{Type: EventNoTrustedNetwork})
}

// checkOverdue returns (daysSince, true) when every trusted network's last sync
// is older than its interval + grace period. daysSince is days since the most
// recent sync (0 if no sync has ever run).
func (m *Monitor) checkOverdue() (daysSince int, overdue bool) {
	grace := time.Duration(m.cfg.GracePeriodDays) * 24 * time.Hour
	now := time.Now()
	var mostRecent time.Time
	for _, n := range m.cfg.TrustedNetworks {
		threshold := time.Duration(n.IntervalDays)*24*time.Hour + grace
		if now.Sub(n.LastSyncTime) <= threshold {
			return 0, false
		}
		if !n.LastSyncTime.IsZero() && n.LastSyncTime.After(mostRecent) {
			mostRecent = n.LastSyncTime
		}
	}
	if !mostRecent.IsZero() {
		daysSince = int(now.Sub(mostRecent).Hours() / 24)
	}
	return daysSince, true
}

func (m *Monitor) emit(e Event) {
	select {
	case m.events <- e:
	default:
		// drop if consumer is not keeping up
	}
}
