# WifiSync — Functional Specification

## 1. Overview

WifiSync is a **desktop backup utility** for Windows, built with Go and Fyne. It monitors the device's active Wi-Fi connection and automatically triggers a unidirectional backup sync to a designated network folder whenever a trusted network is detected and a configurable minimum interval has elapsed since the last sync.

The user configures trusted networks (identified by both SSID and MAC address), source and destination folders, sync frequency, and optional shutdown behavior. Once configured, the app runs silently in the background — no manual intervention is needed for routine backups.

---

## 2. User Roles

There is a single user role: the **Device Owner** — the person who installed the app and manages its settings. All configuration and monitoring is local to one machine; there are no accounts, servers, or cloud components.

---

## 3. Core Concepts

### 3.1 Trusted Network

A trusted network is a Wi-Fi access point the user has explicitly approved for auto-sync. A network is identified by **both** SSID and BSSID (MAC address of the access point) to prevent rogue access point attacks where an attacker creates a network with the same name.

A sync is only eligible to trigger if the currently connected network matches a trusted network entry on both fields.

### 3.2 Sync Interval

Each trusted network entry has a configurable **minimum interval** (e.g., 7 days). After a sync completes on a given network, the next sync on that same network is not eligible until the interval has elapsed. Intervals are tracked per network entry so that syncing on one network does not reset the timer for another.

### 3.3 Sync Operation

A sync is **unidirectional**: files are copied from the local source folder to the network destination folder. The operation mirrors the source folder structure. Files present in the destination but absent in the source are left untouched (no deletion). Files are compared by modification timestamp and size; unchanged files are skipped.

### 3.4 Shutdown-After-Sync

If enabled, the app will initiate a graceful Windows shutdown once the sync completes successfully. If the sync fails, the shutdown is cancelled and the user is notified.

### 3.5 Shutdown Interception During Sync

If the user initiates a system shutdown or poweroff (via the Start menu, keyboard shortcut, or any other Windows mechanism) while a sync is in progress, WifiSync intercepts the shutdown request and presents a prompt informing the user that a backup sync is currently running.

The prompt offers two choices:

- **Wait for sync** — cancel the shutdown and allow the sync to complete normally. Once the sync finishes, the machine does not shut down automatically (the user must initiate shutdown again if desired).
- **Stop sync and shut down** — immediately terminate the sync operation (no partial-state cleanup; files already copied remain at the destination), record the sync as cancelled in the activity log, and allow the shutdown to proceed.

If the user does not interact with the prompt within 60 seconds, the default action is **Stop sync and shut down** — the sync is terminated gracefully and the shutdown proceeds.

This interception applies to user-initiated shutdowns only. It does not block forced shutdowns (e.g., power button held, ACPI hard power-off, or system crash). The Shutdown-After-Sync feature (3.4) is not subject to this interception because it only fires after the sync has already completed successfully.

---

## 4. Screens & UI

### 4.1 Main Status Window

The primary window shown when the app is opened (or when clicking the tray icon). Displays the current state at a glance.

| Element | Description |
|---|---|
| **Connection status** | Current Wi-Fi SSID and whether it matches a trusted network |
| **Last sync** | Timestamp and result (success / failed / never) of the most recent sync |
| **Next sync eligible** | Countdown or date when the next sync on the current network is allowed |
| **Sync now** button | Manually trigger a sync immediately, bypassing the interval check |
| **Open Settings** button | Navigate to the Settings screen |
| **Activity log** | Scrollable log of recent sync events (last 50 entries) |

### 4.2 Settings Screen

Tabbed layout with two tabs: **Networks** and **Sync**.

#### Tab: Networks

Manages the list of trusted Wi-Fi networks.

| Element | Description |
|---|---|
| **Network list** | Table showing configured entries: Name (user label), SSID, BSSID, Interval |
| **Add network** button | Opens the Add Network dialog |
| **Edit** (per row) | Opens the Edit Network dialog pre-populated with that entry's values |
| **Remove** (per row) | Removes the entry after confirmation prompt |

**Add / Edit Network dialog fields:**

| Field | Description |
|---|---|
| Label | User-defined name for this network entry (e.g., "Home", "Office") |
| SSID | The network name; pre-populated from a scan of currently visible networks |
| BSSID | The access point MAC address; pre-populated alongside the SSID from the scan |
| Minimum interval | How long to wait after a sync before syncing again on this network (days) |

When the dialog opens, the app scans for visible networks and presents them in a dropdown. Selecting a network from the dropdown auto-fills both SSID and BSSID. The user can also type values manually.

#### Tab: Sync

Manages the source and destination folders and shutdown behavior.

| Field | Description |
|---|---|
| **Source folder** | Local folder to back up. Selected via native folder picker. |
| **Destination folder** | Network folder (UNC path or mapped drive) to sync into. Selected via native folder picker or typed manually (UNC paths are common: `\\server\share\backup`). |
| **Shutdown after sync** | Toggle. If on, the machine shuts down after a successful auto-sync. |
| **Notify if overdue** | Toggle. If on, the app shows a notification if the sync interval has passed but no trusted network has been detected for more than a configurable grace period (e.g., 3 days beyond the interval). |
| **Overdue grace period** | Number of days past the interval before the overdue notification fires. Only shown if "Notify if overdue" is enabled. |
| **Polling interval** | How often (in seconds) the app checks the current Wi-Fi connection. Default: 60. Range: 15–300 seconds. |
| **Per-file copy timeout** | Maximum time allowed to copy a single file before it is abandoned and the sync is cancelled. Default: 5 minutes. Range: 1–60 minutes. |

---

## 5. Background Behavior

### 5.1 System Tray

WifiSync runs minimized to the Windows system tray when not in focus. The tray icon indicates status:

| Icon state | Meaning |
|---|---|
| Idle (grey) | Running; no trusted network connected |
| Ready (green) | Trusted network connected; sync eligible |
| Syncing (animated) | Sync in progress |
| Warning (yellow) | Last sync failed, or overdue notification active |

Right-clicking the tray icon shows a context menu:

- Open WifiSync
- Sync Now
- Exit

### 5.2 Network Monitoring Loop

The app polls the active Wi-Fi connection at the user-configured polling interval (default: 60 seconds; configurable in Settings → Sync). On each poll:

1. Detect currently connected SSID and BSSID.
2. Compare against the trusted network list, matching on **both** SSID and BSSID.
3. If both fields match a trusted entry and the sync interval for that entry has elapsed since the last successful sync, trigger an automatic sync.
4. If the SSID matches a trusted entry but the BSSID does **not** match any entry sharing that SSID, show a notification alerting the user to the unknown access point:

   > **Unknown access point detected**
   > You are connected to "[SSID]" but from an access point not in your trusted list (BSSID: `xx:xx:xx:xx:xx:xx`). No sync will run.
   > [**Add this access point**] [**Ignore**]

   - **Add this access point** — opens the Add Network dialog pre-filled with the current SSID, the detected BSSID, and the same minimum interval as the existing entry for that SSID. The user can adjust before saving.
   - **Ignore** — dismisses the notification. The same BSSID will not trigger another notification for the remainder of the session.

   This handles both security (rogue AP on a known SSID) and legitimate enterprise scenarios where the same network name is served by multiple access points that the user may wish to add over time.

5. If no SSID match is found at all, do nothing.

### 5.3 Sync Execution

1. Validate that source folder exists and is accessible.
2. Validate that destination folder exists and is accessible (network reachability check).
3. Walk the source folder tree recursively.
4. For each file: copy to destination if it does not exist there, or if the source modification time is newer than the destination copy.
5. Record result (files copied, bytes transferred, duration, any errors) to the activity log.
6. Update the last-sync timestamp for the active trusted network entry.
7. If Shutdown After Sync is enabled and the sync completed without errors, initiate Windows shutdown.

If the sync fails mid-way, it is recorded as failed. No partial state cleanup is performed — files already copied remain in the destination.

### 5.4 Graceful Cancellation

Graceful cancellation applies whenever a sync is interrupted — whether by the user (via the shutdown interception prompt in §3.5), by a timeout, or by a loss of network connectivity mid-sync.

**Per-file timeout**

Each file copy operation has a configurable maximum duration (default: 5 minutes). If a single file has not finished copying within that time, the copy is abandoned:

1. The incomplete file at the destination is deleted if possible; if deletion fails, it is left as-is and flagged in the activity log so the user is aware of the partial file.
2. If a previous version of that file already existed at the destination before the sync started, the previous version is preserved — the app writes the new copy to a temporary name and only replaces the original on successful completion. On timeout or error, the temporary file is discarded and the original remains intact.
3. No further files are started after a per-file timeout. The sync is marked as **cancelled** (not failed) in the activity log.

**Cancellation in progress**

When cancellation is triggered (timeout or user action):

- The file currently being copied is allowed to finish its current write buffer flush, then the copy stream is closed.
- No new file copies are started.
- Files already successfully copied during this sync run remain at the destination.
- The activity log records: files copied, files skipped (unchanged), files cancelled (the timed-out file and any unstarted files), and the cancellation reason.
- The last-sync timestamp is **not** updated for the network entry — a cancelled sync does not count as a completed one, so the next poll will re-evaluate eligibility normally.

**Per-file timeout setting**

The per-file timeout is configurable. It will be added to the Settings → Sync tab alongside the other sync settings.

| Field | Description |
|---|---|
| **Per-file copy timeout** | Maximum time allowed to copy a single file before it is abandoned. Default: 5 minutes. Range: 1–60 minutes. |

### 5.5 Startup Behavior

The app registers itself to launch at Windows startup (via the user's startup registry key). On launch, it opens minimized to the tray. The main window is not shown on startup unless this is the first run (no configuration exists yet).


---

## 6. Notifications

WifiSync uses Windows desktop notifications (toast notifications) for the following events:

| Event | Notification |
|---|---|
| Sync started | "WifiSync: Backup started on [Network Label]" |
| Sync completed | "WifiSync: Backup complete — [N] files copied" |
| Sync failed | "WifiSync: Backup failed — [error summary]" |
| Overdue backup | "WifiSync: Backup overdue — last sync was [N] days ago" |
| Shutdown pending | "WifiSync: Backup complete. Shutting down in 30 seconds." (with Cancel button) |

---

## 7. First-Run Experience

On first launch (no config file found), the app opens the main window (not tray) and shows a setup prompt guiding the user through:

1. Add at least one trusted network.
2. Select the source folder.
3. Select the destination folder.
4. Optionally configure shutdown and overdue notification.

The user cannot trigger a sync (manual or automatic) until at least one trusted network, a source folder, and a destination folder are configured.

---

## 8. Configuration Storage

All settings are stored in a single local JSON config file at:

```
%APPDATA%\WifiSync\config.json
```

The activity log is stored separately:

```
%APPDATA%\WifiSync\sync.log
```

No cloud sync, no account, no telemetry.

---

## 9. Security Considerations

- **Dual-field network matching** (SSID + BSSID) prevents auto-sync on rogue networks that share a known SSID.
- The config file is stored in the user's `%APPDATA%` directory, accessible only to that Windows user account.
- No credentials are stored — access to the network destination folder relies on the Windows session's existing network authentication (domain credentials or saved network passwords).
- The app never transmits data to any external server.

---

## 10. Out of Scope (v1)

- macOS or Linux support (Windows target only)
- Multiple source folders per sync profile
- Encryption of backed-up files
- Conflict resolution or bidirectional sync
- Remote management or monitoring via web interface
- Scheduling by time of day (interval-only in v1)
- File exclusion rules (e.g., skip `*.tmp` files)
- Bandwidth throttling
- Email or SMS notifications
