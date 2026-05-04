/**
 * Repo Search & Sync View — Discover and enable repositories
 */

const RepoSearchView = {
  name: "repo-search",
  title: "Repo Search",

  /**
   * Render repo search view
   */
  async render() {
    const state = InkerState.get();
    const { repositories } = state;

    return `
      <div class="view-header">
        <h2>Repository Search & Sync</h2>
        <p>Discover and enable repositories for local synchronization</p>
      </div>

      <div class="card">
        <input
          type="search"
          id="repoSearchInput"
          class="form-input"
          placeholder="Search by owner, name, or branch..."
          style="margin: 0;"
        />
      </div>

      <div style="margin-top: 12px;">
        <div class="settings-group-title">
          ${repositories.length} Selected Repository${repositories.length !== 1 ? "ies" : ""}
        </div>
        <div id="repoList"></div>
      </div>

      <div style="margin-top: 20px; padding: 16px; background: rgba(37, 99, 235, 0.05); border-radius: 12px;">
        <p class="card-meta">
          💡 <strong>Tip:</strong> Select repositories to enable automatic synchronization.
          Each repository will be mounted in your FtRSync folder.
        </p>
      </div>
    `;
  },

  /**
   * Render repository list
   */
  renderRepoList(filterText = "") {
    const state = InkerState.get();
    const list = document.getElementById("repoList");

    if (!list) return;

    const repos = state.repositories;
    const normalized = (filterText || "").toLowerCase();

    const filtered = repos.filter((repo) => {
      if (!normalized) return true;
      return [repo.owner, repo.name, repo.branch].join(" ").toLowerCase().includes(normalized);
    });

    if (filtered.length === 0) {
      list.innerHTML = `
        <div class="empty-state" style="padding: 20px;">
          <div class="empty-state-icon">🔍</div>
          <p class="empty-state-title">No repositories found</p>
          <p class="empty-state-text">${
            repos.length === 0 ? "Add repositories via Inkdrop" : "Try a different search term"
          }</p>
        </div>
      `;
      return;
    }

    list.innerHTML = filtered
      .map(
        (repo, idx) => `
      <div class="card" style="display: flex; align-items: center; justify-content: space-between; padding: 12px 16px;">
        <div style="flex: 1;">
          <p style="margin: 0; font-weight: 500;">
            ${repo.owner}/<strong>${repo.name}</strong>
          </p>
          <p style="margin: 4px 0 0; font-size: 0.84rem; color: var(--muted);">
            Branch: ${repo.branch} • 
            Access: ${repo.writable ? "Read/Write" : "Read-only"} •
            Latency: ${repo.latencyMS}ms
          </p>
        </div>
        <label style="display: flex; align-items: center; cursor: pointer;">
          <input
            type="checkbox"
            class="repo-toggle-checkbox"
            data-repo-id="${repo.id}"
            ${repo.selected ? "checked" : ""}
            style="width: 20px; height: 20px; cursor: pointer;"
          />
        </label>
      </div>
    `
      )
      .join("");

    // Attach checkbox listeners
    document.querySelectorAll(".repo-toggle-checkbox").forEach((checkbox) => {
      checkbox.addEventListener("change", async (e) => {
        const repoId = e.target.dataset.repoId;
        await InkerState.updateRepository(repoId, {
          selected: e.target.checked,
        });
      });
    });
  },

  /**
   * Attach event listeners
   */
  async attach() {
    this.renderRepoList();

    const searchInput = document.getElementById("repoSearchInput");
    searchInput?.addEventListener("input", (e) => {
      this.renderRepoList(e.target.value);
    });
  },

  /**
   * Cleanup
   */
  detach() {
    // Event listeners automatically removed when view is hidden
  },
};
