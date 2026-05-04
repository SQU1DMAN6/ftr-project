/**
 * Settings View — Configuration and preferences
 */

const SettingsView = {
  name: "settings",
  title: "Settings",

  /**
   * Render settings view
   */
  async render() {
    const state = InkerState.get();
    const { profile } = state;

    return `
      <div class="view-header">
        <h2>Settings</h2>
        <p>Configure synchronization behavior and daemon preferences</p>
      </div>

      <div class="settings-group">
        <div class="settings-group-title">Profile</div>

        <div class="card">
          <div class="form-group">
            <label class="form-label">Email/Username</label>
            <input
              type="email"
              id="profileEmail"
              class="form-input"
              value="${profile.email || ''}"
              placeholder="user@example.com"
            />
            <p class="card-meta">Your FtR/Inkdrop authentication</p>
          </div>

          <div class="form-group">
            <label class="form-label">FtRSync Mount Path</label>
            <input
              type="text"
              id="mountPath"
              class="form-input"
              value="${profile.mountPath || '~/FtRSync'}"
              placeholder="~/FtRSync"
            />
            <p class="card-meta">Local directory where repositories are mounted</p>
          </div>
        </div>
      </div>

      <div class="settings-group">
        <div class="settings-group-title">Sync Behavior</div>

        <div class="card">
          <div class="form-group">
            <label class="form-label">Boot Sync</label>
            <select id="bootSync" class="form-select">
              <option value="true" ${profile.bootSyncEnabled ? 'selected' : ''}>
                Enabled — Daemon starts on system boot
              </option>
              <option value="false" ${!profile.bootSyncEnabled ? 'selected' : ''}>
                Disabled — Manual daemon start only
              </option>
            </select>
            <p class="card-meta">Start sync daemon automatically when your system boots</p>
          </div>

          <div class="form-group">
            <label class="form-label">Conflict Resolution</label>
            <select id="conflictMode" class="form-select">
              <option value="interactive" ${profile.conflictMode === 'interactive' ? 'selected' : ''}>
                Interactive — Ask me to resolve conflicts
              </option>
              <option value="auto-local" ${profile.conflictMode === 'auto-local' ? 'selected' : ''}>
                Auto Local — Keep local changes on conflict
              </option>
              <option value="auto-remote" ${profile.conflictMode === 'auto-remote' ? 'selected' : ''}>
                Auto Remote — Accept remote changes on conflict
              </option>
            </select>
            <p class="card-meta">How to handle conflicting file edits</p>
          </div>
        </div>
      </div>

      <div class="settings-group">
        <div class="settings-group-title">Daemon Control</div>

        <div class="card">
          <div style="display: flex; gap: 8px;">
            <button id="restartDaemonBtn" class="button primary">
              🔄 Restart Daemon
            </button>
            <button id="viewLogsBtn" class="button secondary">
              📋 View Logs
            </button>
          </div>
          <p class="card-meta" style="margin-top: 12px;">
            Restart the background sync process to apply changes
          </p>
        </div>
      </div>

      <div class="settings-group">
        <div class="settings-group-title">About</div>

        <div class="card">
          <p class="card-meta">
            <strong>FtR Inker 3.0</strong><br />
            Daemon Manager for Collaborative Repository Sync<br />
            Built with Electron + Go backend
          </p>
        </div>
      </div>
    `;
  },

  /**
   * Attach event listeners
   */
  async attach() {
    const emailInput = document.getElementById("profileEmail");
    const mountPathInput = document.getElementById("mountPath");
    const bootSyncSelect = document.getElementById("bootSync");
    const conflictModeSelect = document.getElementById("conflictMode");
    const restartBtn = document.getElementById("restartDaemonBtn");
    const viewLogsBtn = document.getElementById("viewLogsBtn");

    // Save profile changes
    const saveProfile = async () => {
      await InkerState.updateProfile({
        email: emailInput.value,
        mountPath: mountPathInput.value,
        bootSyncEnabled: bootSyncSelect.value === "true",
        conflictMode: conflictModeSelect.value,
      });
    };

    emailInput?.addEventListener("change", saveProfile);
    mountPathInput?.addEventListener("change", saveProfile);
    bootSyncSelect?.addEventListener("change", saveProfile);
    conflictModeSelect?.addEventListener("change", saveProfile);

    restartBtn?.addEventListener("click", async () => {
      restartBtn.disabled = true;
      restartBtn.textContent = "⏳ Restarting...";
      try {
        await inkerApp.syncRepositories([]);
        setTimeout(() => {
          restartBtn.disabled = false;
          restartBtn.textContent = "🔄 Restart Daemon";
        }, 1000);
      } catch (err) {
        console.error("Restart failed:", err);
        restartBtn.disabled = false;
        restartBtn.textContent = "🔄 Restart Daemon";
      }
    });

    viewLogsBtn?.addEventListener("click", async () => {
      await InkerState.setActiveTab("daemon-logs");
      window.dispatchEvent(new CustomEvent("tab-change", { detail: { tab: "daemon-logs" } }));
    });
  },

  /**
   * Cleanup
   */
  detach() {
    // Event listeners automatically removed when view is hidden
  },
};
