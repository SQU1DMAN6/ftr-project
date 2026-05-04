const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("electronAPI", {
  // State management
  getInitialState() {
    return ipcRenderer.invoke("inker:get-initial-state");
  },

  updateState(updates) {
    return ipcRenderer.invoke("inker:update-state", updates);
  },

  // Daemon health
  daemonHealthCheck() {
    return ipcRenderer.invoke("inker:daemon-health-check");
  },

  // Sync operations
  syncRepositories(repoIds) {
    return ipcRenderer.invoke("inker:sync-repositories", repoIds);
  },

  getSyncStatus() {
    return ipcRenderer.invoke("inker:get-sync-status");
  },

  // Conflict management
  getConflicts() {
    return ipcRenderer.invoke("inker:get-conflicts");
  },

  resolveConflict(filePath, resolution) {
    return ipcRenderer.invoke("inker:resolve-conflict", filePath, resolution);
  },

  // Logs
  getDaemonLogs(limit) {
    return ipcRenderer.invoke("inker:get-daemon-logs", limit);
  },

  // Event listeners
  onHealthUpdate(callback) {
    ipcRenderer.on("daemon:health-update", (event, data) => callback(data));
  },

  onSyncProgress(callback) {
    ipcRenderer.on("daemon:sync-progress", (event, data) => callback(data));
  },

  onLogEntry(callback) {
    ipcRenderer.on("daemon:log-entry", (event, data) => callback(data));
  },

  onStateChange(callback) {
    ipcRenderer.on("daemon:state-change", (event, data) => callback(data));
  },
});

