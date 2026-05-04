/**
 * Daemon Logs View — Monitor daemon health and view real-time logs
 */

const DaemonLogsView = {
  name: "daemon-logs",
  title: "Daemon Logs",
  logStream: null,
  maxLogs: 200,

  /**
   * Render daemon logs view
   */
  async render() {
    const state = InkerState.get();
    const { daemon } = state;

    const daemonStatus = daemon.running ? "🟢 Running" : "🔴 Stopped";
    const daemonActions = daemon.running
      ? `
      <button id="stopDaemonBtn" class="button danger">
        ⏹️ Stop Daemon
      </button>
    `
      : `
      <button id="startDaemonBtn" class="button primary">
        ▶️ Start Daemon
      </button>
    `;

    return `
      <div class="view-header">
        <h2>Daemon Status & Logs</h2>
        <p>Monitor background sync process health and activity</p>
      </div>

      <div class="card" style="display: flex; align-items: center; justify-content: space-between;">
        <div>
          <div class="card-title">Daemon Process</div>
          <p class="card-meta">
            ${daemonStatus}
            ${daemon.pid ? `• PID: ${daemon.pid}` : ""}
          </p>
        </div>
        <div style="display: flex; gap: 8px;">
          ${daemonActions}
          <button id="clearLogsBtn" class="button secondary">
            🗑️ Clear
          </button>
        </div>
      </div>

      <div style="margin-top: 16px;">
        <div class="settings-group-title">Live Logs</div>
        <div
          id="logContainer"
          style="
            background: rgba(19, 34, 56, 0.95);
            border-radius: 12px;
            padding: 12px;
            font-family: 'Monaco', 'Courier New', monospace;
            font-size: 0.8rem;
            line-height: 1.5;
            color: #e0e7ff;
            max-height: 300px;
            overflow-y: auto;
            border: 1px solid var(--line);
          "
        >
          <p style="margin: 0; color: #94a3b8;">Loading logs...</p>
        </div>
      </div>
    `;
  },

  /**
   * Load and display logs
   */
  async loadLogs() {
    const container = document.getElementById("logContainer");
    if (!container) return;

    try {
      const logs = await inkerApp.getDaemonLogs(this.maxLogs);

      if (!logs || logs.length === 0) {
        container.innerHTML = '<p style="margin: 0; color: #94a3b8;">No logs yet</p>';
        return;
      }

      container.innerHTML = logs
        .map(
          (entry) => `
        <div style="margin-bottom: 4px;">
          <span class="log-level ${entry.level ? entry.level.toLowerCase() : 'info'}">
            ${entry.level || "INFO"}
          </span>
          <span style="color: #94a3b8;">${new Date(entry.timestamp).toLocaleTimeString()}</span>
          <span style="margin-left: 8px;">${this._escapeHtml(entry.message)}</span>
        </div>
      `
        )
        .join("");

      // Auto-scroll to bottom
      container.scrollTop = container.scrollHeight;
    } catch (err) {
      container.innerHTML = `<p style="margin: 0; color: #fca5a5;">Failed to load logs: ${err.message}</p>`;
    }
  },

  /**
   * Attach event listeners and start log streaming
   */
  async attach() {
    await this.loadLogs();

    const startBtn = document.getElementById("startDaemonBtn");
    const stopBtn = document.getElementById("stopDaemonBtn");
    const clearBtn = document.getElementById("clearLogsBtn");

    startBtn?.addEventListener("click", async () => {
      startBtn.disabled = true;
      startBtn.textContent = "⏳ Starting...";
      try {
        await inkerApp.syncRepositories([]);
        await InkerState.updateDaemon({ running: true });
        location.reload();
      } catch (err) {
        console.error("Start failed:", err);
        startBtn.disabled = false;
        startBtn.textContent = "▶️ Start Daemon";
      }
    });

    stopBtn?.addEventListener("click", async () => {
      stopBtn.disabled = true;
      stopBtn.textContent = "⏳ Stopping...";
      try {
        await inkerApp.syncRepositories([]);
        await InkerState.updateDaemon({ running: false });
        location.reload();
      } catch (err) {
        console.error("Stop failed:", err);
        stopBtn.disabled = false;
        stopBtn.textContent = "⏹️ Stop Daemon";
      }
    });

    clearBtn?.addEventListener("click", () => {
      const container = document.getElementById("logContainer");
      if (container) {
        container.innerHTML = '<p style="margin: 0; color: #94a3b8;">Logs cleared</p>';
      }
    });

    // Listen for new log entries
    inkerApp.onLogEntry?.((entry) => {
      this.addLogEntry(entry);
    });
  },

  /**
   * Add a new log entry to the display
   */
  addLogEntry(entry) {
    const container = document.getElementById("logContainer");
    if (!container) return;

    if (container.textContent.includes("Loading logs")) {
      container.innerHTML = "";
    }

    const logDiv = document.createElement("div");
    logDiv.style.marginBottom = "4px";
    logDiv.innerHTML = `
      <span class="log-level ${entry.level ? entry.level.toLowerCase() : 'info'}">
        ${entry.level || "INFO"}
      </span>
      <span style="color: #94a3b8;">${new Date(entry.timestamp).toLocaleTimeString()}</span>
      <span style="margin-left: 8px;">${this._escapeHtml(entry.message)}</span>
    `;

    container.appendChild(logDiv);

    // Keep only maxLogs entries
    while (container.children.length > this.maxLogs) {
      container.removeChild(container.firstChild);
    }

    // Auto-scroll to bottom
    container.scrollTop = container.scrollHeight;
  },

  /**
   * Escape HTML special characters
   */
  _escapeHtml(text) {
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
  },

  /**
   * Cleanup
   */
  detach() {
    // Event listeners automatically removed when view is hidden
  },
};
