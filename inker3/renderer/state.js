/**
 * State Manager — Persistent local state for Inker daemon manager
 * Loads/saves from ~/.config/inker/state.json
 */

const InkerState = {
  /**
   * Default state structure
   */
  defaults: {
    profile: {
      email: "",
      mountPath: "~/FtRSync",
      bootSyncEnabled: false,
      conflictMode: "interactive", // interactive | auto-local | auto-remote
    },
    repositories: [],
    daemon: {
      running: false,
      pid: null,
      lastHealthCheck: null,
    },
    ui: {
      activeTab: "sync-status",
      lastUpdated: null,
    },
  },

  /**
   * Current in-memory state
   */
  state: null,

  /**
   * Initialize state
   */
  async init() {
    const initial = await inkerApp.getInitialState();
    this.state = {
      ...this.defaults,
      ...initial,
    };
    return this.state;
  },

  /**
   * Get current state
   */
  get() {
    return this.state || this.defaults;
  },

  /**
   * Get specific property
   */
  getProperty(path) {
    return this._getNestedProperty(this.get(), path);
  },

  /**
   * Update state and persist
   */
  async set(updates) {
    this.state = {
      ...this.get(),
      ...updates,
      ui: {
        ...this.get().ui,
        lastUpdated: new Date().toISOString(),
      },
    };
    await inkerApp.updateState(this.state);
    return this.state;
  },

  /**
   * Update profile
   */
  async updateProfile(profileUpdates) {
    const current = this.get();
    return this.set({
      profile: {
        ...current.profile,
        ...profileUpdates,
      },
    });
  },

  /**
   * Update repositories list
   */
  async updateRepositories(repos) {
    return this.set({ repositories: repos });
  },

  /**
   * Add repository
   */
  async addRepository(repo) {
    const current = this.get();
    const existing = current.repositories.find((r) => r.id === repo.id);
    if (!existing) {
      return this.set({
        repositories: [...current.repositories, repo],
      });
    }
    return current;
  },

  /**
   * Update repository
   */
  async updateRepository(repoId, updates) {
    const current = this.get();
    const repos = current.repositories.map((r) =>
      r.id === repoId ? { ...r, ...updates } : r
    );
    return this.set({ repositories: repos });
  },

  /**
   * Remove repository
   */
  async removeRepository(repoId) {
    const current = this.get();
    return this.set({
      repositories: current.repositories.filter((r) => r.id !== repoId),
    });
  },

  /**
   * Update daemon status
   */
  async updateDaemon(daemonUpdates) {
    const current = this.get();
    return this.set({
      daemon: {
        ...current.daemon,
        ...daemonUpdates,
        lastHealthCheck: new Date().toISOString(),
      },
    });
  },

  /**
   * Set active tab
   */
  async setActiveTab(tabName) {
    const current = this.get();
    return this.set({
      ui: {
        ...current.ui,
        activeTab: tabName,
      },
    });
  },

  /**
   * Helper: Get nested property by dot notation
   */
  _getNestedProperty(obj, path) {
    return path.split(".").reduce((current, prop) => current?.[prop], obj);
  },
};

// Make available globally
if (typeof window !== 'undefined') {
  window.InkerState = InkerState;
}
