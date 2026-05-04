const { app, BrowserWindow, ipcMain } = require("electron");
const path = require("node:path");
const fs = require("node:fs");
const os = require("node:os");

// State persistence
const STATE_DIR = path.join(os.homedir(), ".config", "inker");
const STATE_FILE = path.join(STATE_DIR, "state.json");

// Ensure state directory exists
if (!fs.existsSync(STATE_DIR)) {
  fs.mkdirSync(STATE_DIR, { recursive: true });
}

// Default state
const DEFAULT_STATE = {
  profile: {
    email: "",
    mountPath: "~/FtRSync",
    bootSyncEnabled: false,
    conflictMode: "interactive",
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
};

// Load or create state
function loadState() {
  try {
    if (fs.existsSync(STATE_FILE)) {
      const data = fs.readFileSync(STATE_FILE, "utf-8");
      return JSON.parse(data);
    }
  } catch (err) {
    console.error("Failed to load state:", err);
  }
  return DEFAULT_STATE;
}

// Save state to disk
function saveState(state) {
  try {
    fs.writeFileSync(STATE_FILE, JSON.stringify(state, null, 2), "utf-8");
    console.log("✓ State saved to", STATE_FILE);
  } catch (err) {
    console.error("Failed to save state:", err);
  }
}

// Load initial state
let appState = loadState();

function createMainWindow() {
  const mainWindow = new BrowserWindow({
    width: 600,
    height: 700,
    minWidth: 500,
    minHeight: 600,
    title: "FtR Inker 3.0 — Daemon Manager",
    backgroundColor: "#f4f8ff",
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  });

  mainWindow.loadFile(path.join(__dirname, "renderer", "index.html"));

  // Save window state on close
  mainWindow.on("close", () => {
    saveState(appState);
  });

  return mainWindow;
}

app.whenReady().then(() => {
  // IPC Handlers
  ipcMain.handle("inker:get-initial-state", async () => {
    return appState;
  });

  ipcMain.handle("inker:update-state", async (event, updates) => {
    appState = {
      ...appState,
      ...updates,
    };
    saveState(appState);
    return appState;
  });

  ipcMain.handle("inker:daemon-health-check", async () => {
    // Mock implementation — replace with real daemon communication
    return {
      running: appState.daemon.running,
      pid: appState.daemon.pid,
      lastHealthCheck: new Date().toISOString(),
    };
  });

  ipcMain.handle("inker:sync-repositories", async (event, repoIds) => {
    // Mock implementation — replace with real daemon communication
    console.log("Sync requested for repos:", repoIds);
    return { success: true, message: "Sync initiated" };
  });

  ipcMain.handle("inker:get-sync-status", async () => {
    // Mock implementation — replace with real daemon communication
    return {
      daemon: appState.daemon,
      repositories: appState.repositories,
    };
  });

  ipcMain.handle("inker:get-conflicts", async () => {
    // Mock implementation — replace with real daemon communication
    return [];
  });

  ipcMain.handle("inker:resolve-conflict", async (event, filePath, resolution) => {
    // Mock implementation — replace with real daemon communication
    console.log(`Resolved ${filePath} with ${resolution}`);
    return { success: true };
  });

  ipcMain.handle("inker:get-daemon-logs", async (event, limit = 100) => {
    // Mock implementation — replace with real daemon communication
    return [
      {
        timestamp: new Date().toISOString(),
        level: "INFO",
        message: "Daemon started successfully",
      },
      {
        timestamp: new Date().toISOString(),
        level: "INFO",
        message: "Connecting to Inkdrop...",
      },
    ];
  });

  const mainWindow = createMainWindow();

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createMainWindow();
    }
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
});
