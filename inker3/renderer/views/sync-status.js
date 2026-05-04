/**
 * Sync Status View — Real-time sync activity and progress
 */

const SyncStatusView = {
  name: "sync-status",
  title: "Sync Status",
  updateInterval: null,

  /**
   * Render sync status view
   */
  async render() {
    const state = InkerState.get();
    const { daemon, repositories } = state;
    const syncing = repositories.filter((r) => r.syncStatus === "syncing");
    const idle = repositories.filter((r) => r.syncStatus === "idle");
    const error = repositories.filter((r) => r.syncStatus === "error");

    let daemonStatus = "Stopped";
    let daemonColor = "error";
    if (daemon.running) {
      daemonStatus = "Running";
      daemonColor = "healthy";
    }

    return `
      <div class="view-header">
        <h2>Sync Status</h2>
        <p>Monitor real-time repository synchronization</p>
      </div>

      <div class="card">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <div>
            <div class="card-title">Daemon Health</div>
            <p class="card-meta">Background sync process status</p>
          </div>
          <span class="status-indicator ${daemonColor}">${daemonStatus}</span>
        </div>
      </div>

      <div style="display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px; margin-top: 12px;">
        <div class="card">
          <div class="card-title">${syncing.length}</div>
          <p class="card-meta">Syncing repositories</p>
        </div>
        <div class="card">
          <div class="card-title">${idle.length}</div>
          <p class="card-meta">Idle repositories</p>
        </div>
        <div class="card">
          <div class="card-title">${error.length}</div>
          <p class="card-meta">Errors detected</p>
        </div>
      </div>

      <div style="margin-top: 16px;">
        <div class="settings-group-title">Active Repositories</div>
        <div id="repoStatusList"></div>
      </div>

      ${
        repositories.length === 0
          ? `
        <div class="empty-state">
          <div class="empty-state-icon">🔍</div>
          <p class="empty-state-title">No repositories selected</p>
          <p class="empty-state-text">Go to "Repo Search" to add repositories for sync</p>
        </div>
      `
          : ""
      }
    `;
  },

  /**
   * Render repository status items
   */
  renderRepoStatus() {
    const state = InkerState.get();
    const list = document.getElementById("repoStatusList");

    if (!list) return;

    const repos = state.repositories.filter((r) => r.selected);

    if (repos.length === 0) {
      list.innerHTML = `
        <div class="empty-state" style="padding: 20px;">
          <p class="empty-state-text">No repositories selected for sync</p>
        </div>
      `;
      return;
    }

    list.innerHTML = repos
      .map(
        (repo) => `
      <div class="card">
        <div style="display: flex; align-items: start; justify-content: space-between; margin-bottom: 10px;">
          <div>
            <p style="margin: 0; font-weight: 500;">${repo.owner}/${repo.name}</p>
            <p style="margin: 4px 0 0; font-size: 0.85rem; color: var(--muted);">
              Branch: <strong>${repo.branch}</strong> • Latency: ${repo.latencyMS}ms
            </p>
          </div>
          <span class="status-indicator ${
            repo.syncStatus === "syncing"
              ? "syncing"
              : repo.syncStatus === "error"
                ? "error"
                : "idle"
          }">
            ${repo.syncStatus === "syncing" ? "⟳ Syncing" : repo.syncStatus === "error" ? "✗ Error" : "✓ Idle"}
          </span>
        </div>
        ${
          repo.syncStatus === "syncing"
            ? `
          <div class="progress-bar">
            <div class="progress-fill" style="width: ${Math.random() * 100}%"></div>
          </div>
        `
            : ""
        }
        <p class="card-meta" style="margin-top: 8px;">
          Last sync: ${repo.lastSync ? new Date(repo.lastSync).toLocaleString() : "Never"}
        </p>
      </div>
    `
      )
      .join("");
  },

  /**
   * Attach event listeners and start updates
   */
  async attach() {
    this.renderRepoStatus();

    // Poll for status updates every 2 seconds
    this.updateInterval = setInterval(async () => {
      try {
        const status = await inkerApp.getSyncStatus();
        if (status) {
          // Update daemon status if changed
          if (status.daemon) {
            await InkerState.updateDaemon(status.daemon);
          }
          // Update repos if changed
          if (status.repositories) {
            await InkerState.updateRepositories(status.repositories);
          }
        }
      } catch (err) {
        console.error("Failed to get sync status:", err);
      }
    }, 2000);

    // Listen for real-time updates
    inkerApp.onSyncProgress?.((update) => {
      if (update.repoId) {
        InkerState.updateRepository(update.repoId, {
          syncStatus: update.status,
          lastSync: new Date().toISOString(),
        });
        this.renderRepoStatus();
      }
    });
  },

  /**
   * Cleanup
   */
  detach() {
    if (this.updateInterval) {
      clearInterval(this.updateInterval);
      this.updateInterval = null;
    }
  },
};
