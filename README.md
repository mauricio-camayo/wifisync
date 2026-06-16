# WifiSync

A Windows desktop backup utility that watches your Wi-Fi connection and automatically syncs your files to a network folder whenever you connect to a trusted network.

Once configured it runs silently in the system tray — no manual intervention needed for routine backups.

---

## How it works

1. You configure one or more **trusted networks** (identified by both SSID and access point MAC address) and a **source → destination** folder pair.
2. WifiSync polls your active Wi-Fi connection in the background (default: every 60 seconds).
3. When a trusted network is detected and the minimum sync interval has elapsed, it copies new or changed files from your source folder to the destination.
4. Optionally, the machine can shut down automatically after a successful sync.

---

## Features

- **Automatic sync on trusted Wi-Fi** — triggers without user interaction
- **SSID + BSSID matching** — prevents syncing on rogue access points that share a known network name
- **Per-network sync intervals** — each trusted network has its own cooldown (e.g. every 7 days)
- **Incremental copy** — skips files that haven't changed (compares modification time and size)
- **Shutdown after sync** — optional graceful shutdown once backup completes
- **Shutdown interception** — if a system shutdown is triggered mid-sync, prompts the user to wait or abort
- **Overdue notification** — alerts you if no sync has run within the expected window
- **Unknown access point warning** — notifies when an SSID matches but the BSSID doesn't, with an option to add it
- **Activity log** — scrollable history of sync events in the main window (`%APPDATA%\WifiSync\sync.log`)
- **Runs at startup** — registers itself in the Windows startup registry key

---

## System tray

| Icon | Meaning |
|---|---|
| Grey | Running — no trusted network connected |
| Green | Trusted network connected, sync eligible |
| Blue (Animated) | Sync in progress |
| Yellow | Last sync failed, or backup overdue |

Right-click the tray icon for **Open**, **Sync Now**, and **Exit**.

---

## Configuration

All settings are stored locally at `%APPDATA%\WifiSync\config.json`. No account, no cloud, no telemetry.

### Networks tab
Add trusted Wi-Fi networks by scanning visible access points or entering SSID and BSSID manually. Each entry has a label, SSID, BSSID, and minimum sync interval in days.

### Sync tab
| Setting | Default | Range |
|---|---|---|
| Source folder | — | Local path |
| Destination folder | — | Local path or UNC (`\\server\share\backup`) |
| Shutdown after sync | Off | — |
| Notify if overdue | Off | — |
| Overdue grace period | 3 days | 1–365 |
| Polling interval | 60 s | 15–300 s |
| Per-file copy timeout | 5 min | 1–60 min |

---

## Building

See [INSTALL.md](INSTALL.md) for full instructions. Quick reference:

**From Linux/Mac (recommended, requires Docker):**
```
go install github.com/fyne-io/fyne-cross@latest
~/go/bin/fyne-cross windows -arch amd64
```

**Natively on Windows (requires Go + TDM-GCC):**
```
go build -ldflags="-H windowsgui -s -w" -o wifisync.exe .
```

---

## Requirements

- Windows 10 or later
- Wi-Fi adapter
- Network destination reachable via mapped drive or UNC path (authentication handled by your Windows session)

---

## Out of scope (v1)

macOS/Linux support, bidirectional sync, file exclusion rules, encryption, bandwidth throttling, scheduling by time of day, multiple source folders.
