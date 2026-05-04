/**
 * IPC Client — Safe communication bridge with Electron main process
 * Provides daemon communication and state synchronization
 */

const inkerApp = {
  /**
   * Request initial state from main process
   */
  async getInitialState() {
    return window.electronAPI?.getInitialState?.() || {};
  },

  /**
   * Update state in main process (persists to disk)
   */
  async updateState(updates) {
    return window.electronAPI?.updateState?.(updates);
  },

  /**
   * Check daemon health status
   */
  async getDaemonHealth() {
    return window.electronAPI?.daemonHealthCheck?.();
  },

  /**
   * Trigger sync for specific repositories
   */
  async syncRepositories(repoIds) {
    return window.electronAPI?.syncRepositories?.(repoIds);
  },

  /**
   * Get current sync progress
   */
  async getSyncStatus() {
    return window.electronAPI?.getSyncStatus?.();
  },

  /**
   * List conflicted files
   */
  async getConflicts() {
    return window.electronAPI?.getConflicts?.();
  },

  /**
   * Resolve a file conflict
   */
  async resolveConflict(filePath, resolution) {
    return window.electronAPI?.resolveConflict?.(filePath, resolution);
  },

  /**
   * Get daemon logs
   */
  async getDaemonLogs(limit = 100) {
    return window.electronAPI?.getDaemonLogs?.(limit);
  },

  /**
   * Subscribe to daemon health updates
   */
  onHealthUpdate(callback) {
    window.electronAPI?.onHealthUpdate?.(callback);
  },

  /**
   * Subscribe to sync progress updates
   */
  onSyncProgress(callback) {
    window.electronAPI?.onSyncProgress?.(callback);
  },

  /**
   * Subscribe to new log entries
   */
  onLogEntry(callback) {
    window.electronAPI?.onLogEntry?.(callback);
  },

  /**
   * Subscribe to state changes
   */
  onStateChange(callback) {
    window.electronAPI?.onStateChange?.(callback);
  },
};

// Make available globally
if (typeof window !== 'undefined') {
  window.inkerApp = inkerApp;
}
