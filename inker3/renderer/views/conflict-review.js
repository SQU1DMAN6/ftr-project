/**
 * Conflict Review View — Resolve sync conflicts
 */

const ConflictReviewView = {
  name: "conflicts",
  title: "Conflicts",
  pollInterval: null,

  /**
   * Render conflict review view
   */
  async render() {
    return `
      <div class="view-header">
        <h2>Conflict Review</h2>
        <p>Review and resolve files with conflicting edits</p>
      </div>

      <div id="conflictList"></div>

      <div style="margin-top: 20px; padding: 16px; background: rgba(220, 38, 38, 0.05); border-radius: 12px;">
        <p class="card-meta">
          ⚠️ <strong>How conflicts happen:</strong> When both local and remote changes are made to the same file
          before sync completes. You can resolve them here by choosing which version to keep.
        </p>
      </div>
    `;
  },

  /**
   * Render conflict list
   */
  async renderConflicts() {
    const conflictList = document.getElementById("conflictList");
    if (!conflictList) return;

    try {
      const conflicts = await inkerApp.getConflicts();

      if (!conflicts || conflicts.length === 0) {
        conflictList.innerHTML = `
          <div class="empty-state">
            <div class="empty-state-icon">✓</div>
            <p class="empty-state-title">No conflicts</p>
            <p class="empty-state-text">Your repositories are in sync</p>
          </div>
        `;
        return;
      }

      conflictList.innerHTML = conflicts
        .map(
          (conflict) => `
        <div class="conflict-item">
          <div style="display: flex; align-items: start; justify-content: space-between;">
            <div style="flex: 1;">
              <p class="list-item-title">${conflict.fileName}</p>
              <p class="conflict-path">${conflict.filePath}</p>
              <p class="list-item-meta" style="margin-top: 8px;">
                Repository: ${conflict.owner}/${conflict.repo}
              </p>
            </div>
          </div>

          <div style="margin-top: 12px; display: flex; gap: 8px;">
            <button
              class="button secondary"
              data-conflict-id="${conflict.id}"
              data-resolution="keep-local"
              style="flex: 1;"
            >
              📌 Keep Local
            </button>
            <button
              class="button secondary"
              data-conflict-id="${conflict.id}"
              data-resolution="accept-remote"
              style="flex: 1;"
            >
              ⬇️ Accept Remote
            </button>
            <button
              class="button secondary"
              data-conflict-id="${conflict.id}"
              data-resolution="view-diff"
              style="flex: 1;"
            >
              🔍 View Diff
            </button>
          </div>

          <p class="card-meta" style="margin-top: 8px;">
            Last modified: ${new Date(conflict.modified).toLocaleString()}
          </p>
        </div>
      `
        )
        .join("");

      // Attach resolution handlers
      document.querySelectorAll("[data-resolution]").forEach((btn) => {
        btn.addEventListener("click", async (e) => {
          const conflictId = e.target.dataset.conflictId;
          const resolution = e.target.dataset.resolution;

          if (resolution === "view-diff") {
            alert("Diff viewer coming soon!");
            return;
          }

          e.target.disabled = true;
          e.target.textContent = "⏳ Resolving...";

          try {
            await inkerApp.resolveConflict(conflictId, resolution);
            await this.renderConflicts();
          } catch (err) {
            console.error("Resolution failed:", err);
            e.target.disabled = false;
            e.target.textContent = resolution === "keep-local" ? "📌 Keep Local" : "⬇️ Accept Remote";
          }
        });
      });
    } catch (err) {
      console.error("Failed to load conflicts:", err);
      conflictList.innerHTML = `
        <div class="empty-state">
          <div class="empty-state-icon">❌</div>
          <p class="empty-state-title">Failed to load conflicts</p>
          <p class="empty-state-text">${err.message}</p>
        </div>
      `;
    }
  },

  /**
   * Attach event listeners
   */
  async attach() {
    await this.renderConflicts();

    // Poll for new conflicts every 5 seconds
    this.pollInterval = setInterval(() => {
      this.renderConflicts();
    }, 5000);

    // Listen for conflict updates
    inkerApp.onStateChange?.((state) => {
      if (state.conflicts) {
        this.renderConflicts();
      }
    });
  },

  /**
   * Cleanup
   */
  detach() {
    if (this.pollInterval) {
      clearInterval(this.pollInterval);
      this.pollInterval = null;
    }
  },
};
