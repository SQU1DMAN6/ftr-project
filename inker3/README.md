# Inker 3.0 Electron Shell

This directory contains the **Inker 3.0 Daemon Manager** — a compact Electron desktop shell for managing local repository synchronization with Inkdrop backends.

## Architecture Overview

Inker 3.0 is designed as a **local-first, background-oriented system** consisting of three layers:

### 1. Daemon Manager (Electron Shell) — This Directory
The GUI manages the background sync daemon without requiring a persistent window. Key features:

- **Compact window design** with tab-based views (Settings, Sync Status, Repo Search, Conflict Review, Daemon Logs)
- **Persistent state** stored to disk (`~/.config/inker/state.json`)
- **Tray integration** for quick access and status checking
- **IPC bridge** to background sync daemon process
- **Automatic updates** to sync status without polling
- **Zero configuration** for end users — just add repos and sync

### 2. Local Sync Daemon
The background service that performs the actual synchronization:

- Starts on boot (via systemd or similar)
- Manages `~/FtRSync` virtual mount/alias for selected repositories
- Watches selected repositories for local file changes
- Batches and stages changes for efficient remote upload
- Pulls remote updates with minimal latency
- Maintains local cache for fast UI responses
- Runs independently of the Electron shell (can close the UI, daemon continues)

### 3. Inkdrop Bridge
Seamless integration with Inkdrop servers:

- Session reuse from FtR CLI authentication
- Capability discovery (permissions, branches, write access)
- Efficient delta upload/download flows
- Real-time presence and live document collaboration
- Conflict detection and resolution guidance

## GUI Design Philosophy

The daemon manager window is **intentionally compact**:

- **Window size:** ~600×500px (small enough for a quick glance)
- **Tab switcher:** Bottom navigation bar with 5 primary views
- **One-click actions:** Sync, settings changes, conflict resolution
- **Auto-save:** All configuration changes persist immediately to disk
- **Minimize, not close:** Background daemon continues running when window closes
- **Status indicators:** Visual feedback for daemon health, sync progress, conflicts

### Tab Views

1. **Settings** — Configure sync behavior
   - Enable/disable boot sync
   - Select FtRSync mount path
   - Authentication (session/token management)
   - Sync frequency and conflict resolution mode
   - Local cache settings

2. **Sync Status** — Real-time sync activity
   - Selected repositories and sync status
   - File counts (pending up/down)
   - Network latency per repository
   - Last sync timestamp
   - Current operation details

3. **Repo Search & Sync** — Discover and enable repositories
   - Search Inkdrop for available repositories
   - Add repositories for local sync
   - Toggle sync on/off per repository
   - View repository metadata (owner, branch, permissions)
   - Quick actions (refresh, open in explorer)

4. **Conflict Review** — Resolve sync conflicts
   - List conflicted files
   - Side-by-side diff viewer
   - Resolution options (keep local, accept remote, merge)
   - Undo/retry functionality

5. **Daemon Status & Logs** — Monitor sync daemon health
   - Daemon process status (running, stopped, error)
   - Start/stop/restart controls
   - Real-time log streaming
   - Error indicators and recovery suggestions

## Core Functionalities

### Persistent State Management
All configuration is stored to `~/.config/inker/state.json`:
```json
{
  "profile": {
    "email": "user@example.com",
    "mountPath": "~/FtRSync",
    "bootSyncEnabled": true,
    "conflictMode": "interactive"
  },
  "repositories": [
    {
      "id": "repo-uuid",
      "owner": "username",
      "name": "repo-name",
      "selected": true,
      "branch": "main",
      "writable": true,
      "syncStatus": "idle",
      "lastSync": "2026-05-04T14:30:00Z"
    }
  ],
  "daemon": {
    "running": true,
    "pid": 12345,
    "lastHealthCheck": "2026-05-04T14:31:00Z"
  }
}
```

### IPC Communication
The Electron shell communicates with the daemon via IPC:
- `daemon:health-check` — Verify daemon is running
- `daemon:sync-repos` — Trigger sync for selected repos
- `daemon:get-status` — Get current sync progress
- `daemon:list-conflicts` — Retrieve conflicted files
- `daemon:resolve-conflict` — Apply conflict resolution
- `daemon:get-logs` — Stream daemon logs to UI

### Session Management
- Reuses FtR CLI session from `~/.config/ftr/session`
- Falls back to Inkdrop login if no session exists
- Stores refresh token for daemon use
- Automatic reauthentication on token expiry

### Delta Sync
- Tracks file modification times and hashes
- Only uploads/downloads changed files
- Uses `.inker/metadata.json` in repositories for state
- Efficient batching of operations

## File Structure

```
inker3/
├── main.js                    # Electron app entry point
├── preload.js                 # Secure IPC bridge
├── daemon.js                  # Background sync process (spawned by main.js)
├── package.json               # Dependencies
│
├── renderer/
│   ├── index.html             # Main shell layout with tab switcher
│   ├── app.js                 # Tab routing and state management
│   ├── state.js               # Local state persistence
│   ├── ipc-client.js          # IPC communication utilities
│   │
│   ├── views/
│   │   ├── settings.js        # Settings tab renderer
│   │   ├── sync-status.js     # Sync status tab renderer
│   │   ├── repo-search.js     # Repository search & sync tab
│   │   ├── conflict-review.js # Conflict resolution tab
│   │   └── daemon-logs.js     # Daemon status & logs tab
│   │
│   └── styles.css             # Compact, minimal styling
│
├── lib/
│   ├── api-client.js          # Inkdrop API communication
│   ├── sync-manager.js        # Sync coordination logic
│   └── conflict-resolver.js   # Conflict detection & resolution
│
└── BUILD/
    └── inker3.ftr             # FtR metadata for packaging
```

## API Design (IPC Protocol)

### Daemon ↔ Shell Communication

**Request-Response Pattern:**
```javascript
// Shell requests daemon status
const status = await ipc.invoke('daemon:get-status');
// Returns: { syncing: boolean, repos: [], conflicts: [], lastSync: timestamp }

// Shell triggers sync
const result = await ipc.invoke('daemon:sync-repos', { repoIds: ['repo-1', 'repo-2'] });
// Returns: { success: boolean, message: string }
```

**Event Stream Pattern:**
```javascript
// Shell subscribes to daemon logs
ipc.on('daemon:log-entry', (entry) => {
  console.log(`[${entry.level}] ${entry.message}`);
});

// Daemon sends health check every 5 seconds
ipc.on('daemon:health-update', (health) => {
  updateDaemonStatusUI(health);
});
```

## Intended Development Roadmap

### Phase 1: Foundation (Current)
- [x] Electron scaffold with tab switcher
- [ ] Persistent state to `~/.config/inker/state.json`
- [ ] Compact window styling
- [ ] Settings tab with basic config
- [ ] IPC protocol definition

### Phase 2: Daemon Integration
- [ ] Daemon process spawning and lifecycle
- [ ] Health check and status monitoring
- [ ] Sync status real-time updates
- [ ] Log streaming from daemon to UI

### Phase 3: Repository Sync
- [ ] Connect to Inkdrop for repo discovery
- [ ] Repository selection and toggle
- [ ] Delta sync implementation
- [ ] FtRSync mount point management

### Phase 4: Conflict Resolution
- [ ] Conflict detection and reporting
- [ ] Diff viewer for conflicted files
- [ ] Resolution UI and operations
- [ ] Undo/retry mechanisms

### Phase 5: Polish & Integration
- [ ] Tray icon and quick actions
- [ ] Boot/systemd integration
- [ ] Settings GUI enhancements
- [ ] Error handling and recovery

## Next Steps

1. **Implement persistent state** — Update `main.js` and create `renderer/state.js`
2. **Refactor UI to compact tabs** — Modify `renderer/index.html` and `renderer/app.js`
3. **Create IPC protocol** — Define daemon communication in `main.js` and `renderer/ipc-client.js`
4. **Stub daemon process** — Create `daemon.js` placeholder with health checks
5. **Implement settings tab** — First functional view for user configuration

