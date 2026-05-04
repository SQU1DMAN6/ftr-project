/**
 * Main App Controller — Handles view switching, state management, and initialization
 */

const AppController = {
  currentView: null,
  views: [SettingsView, SyncStatusView, RepoSearchView, ConflictReviewView, DaemonLogsView],

  /**
   * Initialize the app
   */
  async init() {
    console.log("🚀 Inker 3.0 initializing...");

    try {
      // Initialize state
      await InkerState.init();
      console.log("✓ State loaded");

      // Setup tab switcher
      this.setupTabSwitcher();
      console.log("✓ Tab switcher ready");

      // Load initial view
      const activeTab = InkerState.getProperty("ui.activeTab") || "sync-status";
      await this.switchView(activeTab);
      console.log("✓ Initial view loaded");

      // Start health check polling
      this.startHealthCheck();
      console.log("✓ Health check started");
    } catch (err) {
      console.error("✗ Initialization failed:", err);
      this.showError(err);
    }
  },

  /**
   * Setup tab switcher event listeners
   */
  setupTabSwitcher() {
    const buttons = document.querySelectorAll(".tab-button");
    buttons.forEach((btn) => {
      btn.addEventListener("click", () => {
        const tab = btn.dataset.tab;
        this.switchView(tab);
      });
    });
  },

  /**
   * Switch to a different view
   */
  async switchView(tabName) {
    // Validate tab
    const view = this.views.find((v) => v.name === tabName);
    if (!view) {
      console.error(`View not found: ${tabName}`);
      return;
    }

    // Detach current view
    if (this.currentView?.detach) {
      this.currentView.detach();
    }

    // Update active tab button
    document.querySelectorAll(".tab-button").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.tab === tabName);
    });

    // Render new view
    const contentArea = document.getElementById("contentArea");
    const html = await view.render();
    contentArea.innerHTML = html;

    // Attach view listeners
    if (view.attach) {
      await view.attach();
    }

    // Update state
    this.currentView = view;
    await InkerState.setActiveTab(tabName);

    console.log(`📍 Switched to ${tabName}`);
  },

  /**
   * Periodic health check
   */
  startHealthCheck() {
    setInterval(async () => {
      try {
        const health = await inkerApp.getDaemonHealth();
        if (health) {
          await InkerState.updateDaemon(health);

          // Update health indicator if sync-status is active
          if (this.currentView?.name === "sync-status") {
            const indicator = document.querySelector(".status-indicator");
            if (indicator) {
              indicator.className = `status-indicator ${health.running ? "healthy" : "error"}`;
              indicator.textContent = health.running ? "🟢 Running" : "🔴 Stopped";
            }
          }
        }
      } catch (err) {
        // Silently fail — health check is non-critical
      }
    }, 10000); // Every 10 seconds
  },

  /**
   * Show error message
   */
  showError(err) {
    const contentArea = document.getElementById("contentArea");
    contentArea.innerHTML = `
      <div style="padding: 40px; text-align: center;">
        <div style="font-size: 3rem; margin-bottom: 12px;">❌</div>
        <h2 style="margin: 0 0 8px; font-size: 1.3rem;">Initialization Failed</h2>
        <p style="margin: 0 0 16px; color: var(--muted);">${err.message}</p>
        <button
          onclick="location.reload()"
          style="
            padding: 10px 16px;
            border: 0;
            border-radius: 8px;
            background: linear-gradient(135deg, var(--accent), #38bdf8);
            color: white;
            cursor: pointer;
            font-weight: 500;
          "
        >
          🔄 Retry
        </button>
      </div>
    `;
  },
};

// Boot the app when DOM is ready
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", () => AppController.init());
} else {
  AppController.init();
}

// Handle tab change events from other views
window.addEventListener("tab-change", (e) => {
  AppController.switchView(e.detail.tab);
});

