import "./style.css";
import "@xterm/xterm/css/xterm.css";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { SearchAddon } from "@xterm/addon-search";

type Server = {
  id: string;
  teamId?: string;
  name: string;
  group?: string;
  host: string;
  port: number;
  user: string;
  shell: string;
  useTmux?: boolean;
  tmuxSession?: string;
  identity?: string;
  jumpServerId?: string;
  fingerprint?: string;
  favorite?: boolean;
  passwordSaved?: boolean;
  requireBiometric?: boolean;
};

type Team = {
  id: string;
  name: string;
};

type Preferences = {
  cloudUrl?: string;
  defaultUser: string;
  defaultPort: number;
  defaultShell: string;
  defaultIdentity?: string;
  logActivity: boolean;
  scrollback: number;
  theme: "light" | "black" | "glass";
  uiScale: number;
  terminalFontSize: number;
  terminalFontFamily: "system-mono" | "cascadia" | "jetbrains" | "source-code";
  terminalLineHeight: number;
  autoConnectTabs: boolean;
  reopenActiveSession: boolean;
  persistTerminalHistory: boolean;
};

type Config = { servers: Server[]; teams: Team[]; preferences: Preferences };
type CloudState = {
  cloudUrl: string;
  signedIn: boolean;
  user: { id: string; displayName: string; email: string; avatarUrl: string };
  providers: Record<"google" | "apple", boolean>;
  error?: string;
};
type ServerReadiness = { ready: boolean; hasKey: boolean; hasPassword: boolean; hasAgent: boolean; message: string };
type CloudTab = { id: string; serverId: string; title: string; manualTitle?: boolean; restore?: boolean; lastPath?: string; position?: number };
type CloudWorkspace = { revision: number; servers: unknown[]; teams: unknown[]; tabs: CloudTab[]; updatedAt: string };
type CloudSyncResult = { config: Config; workspace: CloudWorkspace; readiness: Record<string, ServerReadiness> };
type Platform = "darwin" | "windows" | "linux";
type PublicKeyInfo = {
  path: string;
  privatePath?: string;
  name: string;
  fingerprint: string;
};
type ConnectionState = "idle" | "connecting" | "connected" | "error";
type ConnectionRequest = {
  tabId: string;
  password: string;
  rememberPassword: boolean;
  requireBiometric: boolean;
};
type RemoteFile = {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  mode: string;
  modTime: number;
};
type TunnelInfo = {
  id: string;
  sessionId: string;
  local: string;
  remoteHost: string;
  remotePort: number;
};
type OutputLine = { kind: string; text: string };
type TerminalTab = {
  id: string;
  serverId: string;
  title: string;
  manualTitle: boolean;
  connection: ConnectionState;
  output: OutputLine[];
  remotePath: string;
  terminal?: Terminal;
  fitAddon?: FitAddon;
  searchAddon?: SearchAddon;
  host?: HTMLDivElement;
  inputQueue: string;
  sendingInput: boolean;
  resizeTimer?: number;
  bookmarks: { id: string; label: string; line: number }[];
  lastRequest?: ConnectionRequest;
  reconnectAttempts: number;
  reconnectTimer?: number;
  pendingTerminalData: string;
  terminalWriteScheduled: boolean;
  restoreSession: boolean;
  restoredTranscript: string;
  commandBuffer: string;
  commandCursor: number;
  commandHistory: string[];
  commandHistoryIndex: number;
};

declare global {
  interface Window {
    go?: { main?: { App?: Record<string, (...args: unknown[]) => Promise<unknown>> } };
    runtime?: {
      WindowMinimise(): void;
      WindowToggleMaximise(): void;
      Quit(): void;
      ClipboardSetText(text: string): Promise<boolean>;
      EventsOn(name: string, callback: (...args: unknown[]) => void): () => void;
    };
  }
}

const icons = {
  plus: `<svg viewBox="0 0 24 24"><path d="M12 5v14M5 12h14"/></svg>`,
  search: `<svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="6.5"/><path d="m16 16 4 4"/></svg>`,
  settings: `<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"/><path d="M19 13.5v-3l-2.2-.8-.5-1.2 1-2.1-2.1-2.1-2.1 1-1.2-.5L10.5 2h-3L6.7 4.8l-1.2.5-2.1-1-2.1 2.1 1 2.1-.5 1.2L0 10.5v3l2.2.8.5 1.2-1 2.1 2.1 2.1 2.1-1 1.2.5.8 2.8h3l.8-2.8 1.2-.5 2.1 1 2.1-2.1-1-2.1.5-1.2z"/></svg>`,
  terminal: `<svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="16" rx="3"/><path d="m7 9 3 3-3 3m6 0h4"/></svg>`,
  server: `<svg viewBox="0 0 24 24"><rect x="4" y="3" width="16" height="7" rx="2"/><rect x="4" y="14" width="16" height="7" rx="2"/><path d="M8 7h.01M8 18h.01m4-11h5m-5 11h5"/></svg>`,
  more: `<svg viewBox="0 0 24 24"><circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/></svg>`,
  arrow: `<svg viewBox="0 0 24 24"><path d="M5 12h14m-5-5 5 5-5 5"/></svg>`,
  chevron: `<svg viewBox="0 0 24 24"><path d="m9 6 6 6-6 6"/></svg>`,
  trash: `<svg viewBox="0 0 24 24"><path d="M4 7h16M9 7V4h6v3m3 0-1 14H7L6 7m4 4v6m4-6v6"/></svg>`,
  code: `<svg viewBox="0 0 24 24"><path d="m8.5 7-5 5 5 5m7-10 5 5-5 5M14 4l-4 16"/></svg>`,
  key: `<svg viewBox="0 0 24 24"><circle cx="8" cy="15" r="4"/><path d="m11 12 8-8m-3 3 3 3m-6 0 2 2"/></svg>`,
};

const demoConfig: Config = {
  teams: [
    { id: "team-northstar", name: "Northstar" },
  ],
  servers: [
    { id: "demo-1", name: "Production", host: "api.northstar.dev", port: 22, user: "deploy", shell: "zsh", useTmux: true, tmuxSession: "sshking", identity: "~/.ssh/id_ed25519", favorite: true, passwordSaved: true, requireBiometric: true },
    { id: "demo-2", teamId: "team-northstar", name: "Staging", host: "stage.northstar.dev", port: 22, user: "ubuntu", shell: "bash", favorite: true },
    { id: "demo-3", name: "Home lab", host: "192.168.1.42", port: 22, user: "operator", shell: "fish" },
  ],
  preferences: {
    cloudUrl: "https://cloud.krilwya.fr",
    defaultUser: "admin",
    defaultPort: 22,
    defaultShell: "default",
    defaultIdentity: "~/.ssh/id_ed25519",
    logActivity: true,
    scrollback: 2000,
    theme: "glass",
    uiScale: 100,
    terminalFontSize: 14,
    terminalFontFamily: "system-mono",
    terminalLineHeight: 140,
    autoConnectTabs: true,
    reopenActiveSession: true,
    persistTerminalHistory: true,
  },
};

const state = {
  config: demoConfig,
  platform: detectedPlatform(),
  selectedId: demoConfig.servers[0]?.id ?? "",
  query: "",
  tabs: [] as TerminalTab[],
  activeTabId: "",
  splitTabId: "",
  modal: "" as "" | "server" | "team" | "account" | "settings" | "activity" | "connect" | "zed" | "ssh-key" | "files" | "tunnels" | "palette" | "trust-host-key",
  editingId: "",
  editingTeamId: "",
  pendingServerTeamId: "",
  personalExpanded: true,
  teamsExpanded: true,
  collapsedTeamIds: new Set<string>(),
  sshKeys: [] as PublicKeyInfo[],
  biometricAvailable: false,
  biometricName: "Device authentication",
  pendingConnection: null as ConnectionRequest | null,
  pendingHostFingerprint: "",
  remoteFiles: [] as RemoteFile[],
  browsingPath: "~",
  filesLoading: false,
  tunnels: [] as TunnelInfo[],
  paletteQuery: "",
  terminalSearch: false,
  terminalSearchQuery: "",
  activity: [] as string[],
  cloud: { cloudUrl: "", signedIn: false, user: { id: "", displayName: "", email: "", avatarUrl: "" }, providers: { google: false, apple: false } } as CloudState,
  cloudLoading: false,
  readiness: {} as Record<string, ServerReadiness>,
};

const app = document.querySelector<HTMLDivElement>("#app")!;
let copyToastTimer: number | undefined;
let cloudSyncTimer: number | undefined;
let cloudSyncInFlight = false;
let cloudSyncQueued = false;
let cloudHydrated = false;
const pendingCloudTabDeletes = new Set<string>();
const pendingCloudServerDeletes = new Set<string>();
const pendingCloudTeamDeletes = new Set<string>();

function backend(name: string, ...args: unknown[]): Promise<any> {
  const fn = window.go?.main?.App?.[name];
  if (fn) return fn(...args);
  return mockBackend(name, args);
}

async function mockBackend(name: string, args: unknown[]): Promise<any> {
  if (name === "GetState") {
    return {
      config: demoConfig,
      platform: detectedPlatform(),
      biometricAvailable: true,
      biometricName: detectedPlatform() === "darwin" ? "Touch ID" : "Windows Hello",
    };
  }
  if (name === "SaveServer") {
    const server = args[0] as Server;
    if (!server.id) server.id = `demo-${Date.now()}`;
    const index = state.config.servers.findIndex((item) => item.id === server.id);
    if (index >= 0) state.config.servers[index] = server;
    else state.config.servers.push(server);
    return state.config;
  }
  if (name === "DeleteServer") {
    state.config.servers = state.config.servers.filter((item) => item.id !== args[0]);
    return state.config;
  }
  if (name === "SaveTeam") {
    const team = args[0] as Team;
    if (!team.id) team.id = `team-${Date.now()}`;
    const index = state.config.teams.findIndex((item) => item.id === team.id);
    if (index >= 0) state.config.teams[index] = team;
    else state.config.teams.push(team);
    return state.config;
  }
  if (name === "DeleteTeam") {
    const teamId = String(args[0]);
    state.config.teams = state.config.teams.filter((team) => team.id !== teamId);
    state.config.servers.forEach((server) => { if (server.teamId === teamId) server.teamId = ""; });
    return state.config;
  }
  if (name === "SavePreferences") {
    state.config.preferences = args[0] as Preferences;
    return state.config;
  }
  if (name === "GetCloudState") return { cloudUrl: args[0], signedIn: false, user: {}, providers: { google: false, apple: false } };
  if (name === "LoginCloud") return { cloudUrl: args[0], signedIn: true, user: { id: "demo", displayName: "Demo User", email: "demo@example.com", avatarUrl: "" }, providers: { google: true, apple: true } };
  if (name === "LogoutCloud") return;
  if (name === "GetServerReadiness") return Object.fromEntries(state.config.servers.map((server) => [server.id, { ready: Boolean(server.identity || server.passwordSaved), hasKey: Boolean(server.identity), hasPassword: Boolean(server.passwordSaved), hasAgent: false, message: "Add a password or SSH key on this device" }]));
  if (name === "SyncCloudWorkspace") return { config: state.config, workspace: { revision: 1, servers: state.config.servers, teams: state.config.teams, tabs: args[1] ?? [], updatedAt: new Date().toISOString() }, readiness: await mockBackend("GetServerReadiness", []) };
  if (name === "GetSessionTranscript") return "";
  if (name === "ClearSessionTranscript" || name === "ClearTerminalHistory") return;
  if (name === "Connect") {
    await new Promise((resolve) => setTimeout(resolve, 650));
    const selected = state.config.servers.find((server) => server.id === args[1]);
    if (selected && !selected.fingerprint) selected.fingerprint = "SHA256:demoHostFingerprint";
    return;
  }
  if (name === "ListSSHKeys") {
    return [
      { path: "~/.ssh/id_ed25519.pub", privatePath: "~/.ssh/id_ed25519", name: "id_ed25519.pub", fingerprint: "SHA256:exampleKeyFingerprint" },
    ];
  }
  if (name === "InstallSSHKey") {
    const selected = state.config.servers.find((server) => server.id === args[0]);
    if (selected) selected.identity = args[3] ? `~/.ssh/sshking_${selected.name.toLowerCase().replace(/\W+/g, "_")}` : String(args[1]).replace(/\.pub$/i, "");
    return state.config;
  }
  if (name === "ImportSSHConfig") return state.config;
  if (name === "ListRemoteFiles") return [
    { name: "src", path: "/srv/northstar/src", isDir: true, size: 0, mode: "drwxr-xr-x", modTime: Date.now() / 1000 },
    { name: "README.md", path: "/srv/northstar/README.md", isDir: false, size: 4210, mode: "-rw-r--r--", modTime: Date.now() / 1000 },
  ];
  if (name === "StartLocalTunnel") return { id: `tunnel-${Date.now()}`, sessionId: args[0], local: `127.0.0.1:${args[1] || 49152}`, remoteHost: args[2], remotePort: args[3] };
  if (name === "GetActivity") return [
    "2026-07-30-Production.log  2026-07-30T15:22:01+02:00 [connect] deploy@api.northstar.dev",
    "2026-07-30-Production.log  2026-07-30T15:22:09+02:00 [command] git status",
  ];
}

function newTerminalTab(serverId = state.selectedId, tabId = ""): TerminalTab {
  const server = state.config.servers.find((item) => item.id === serverId);
  return {
    id: tabId || crypto.randomUUID?.() || `terminal-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    serverId,
    title: server?.name ?? "Terminal",
    manualTitle: false,
    connection: "idle",
    output: [
      { kind: "system", text: "SSHKing secure terminal · session ready" },
      { kind: "muted", text: server ? "Press Connect to open this server." : "Select a server and connect when you’re ready." },
    ],
    remotePath: "~",
    inputQueue: "",
    sendingInput: false,
    bookmarks: [],
    reconnectAttempts: 0,
    pendingTerminalData: "",
    terminalWriteScheduled: false,
    restoreSession: false,
    restoredTranscript: "",
    commandBuffer: "",
    commandCursor: 0,
    commandHistory: [],
    commandHistoryIndex: 0,
  };
}

function restoreWorkspace() {
  try {
    const saved = JSON.parse(localStorage.getItem("sshking.workspace.v1") ?? "{}") as {
      serverIds?: string[];
      tabIds?: string[];
      titles?: string[];
      manualTitles?: boolean[];
      activeIndex?: number;
      splitIndex?: number;
      sessionOpen?: boolean[];
    };
    const knownServers = new Set(state.config.servers.map((server) => server.id));
    const serverIds = (saved.serverIds ?? []).filter((id) => knownServers.has(id)).slice(0, 12);
    state.tabs = (serverIds.length ? serverIds : [state.selectedId]).map((id, index) => newTerminalTab(id, saved.tabIds?.[index] ?? ""));
    state.tabs.forEach((tab, index) => {
      const savedTitle = saved.titles?.[index]?.trim();
      if (savedTitle) tab.title = savedTitle;
      tab.manualTitle = Boolean(saved.manualTitles?.[index]);
      if (!tab.manualTitle && isTerminalProtocolTitle(tab.title)) {
        tab.title = state.config.servers.find((server) => server.id === tab.serverId)?.name ?? "Terminal";
      }
    });
    const activeIndex = Math.min(Math.max(saved.activeIndex ?? 0, 0), state.tabs.length - 1);
    state.tabs.forEach((tab, index) => {
      tab.restoreSession = saved.sessionOpen?.[index] ?? index === activeIndex;
    });
    state.activeTabId = state.tabs[activeIndex]?.id ?? "";
    state.selectedId = state.tabs[activeIndex]?.serverId ?? state.selectedId;
    if (typeof saved.splitIndex === "number" && saved.splitIndex >= 0 && saved.splitIndex < state.tabs.length && saved.splitIndex !== activeIndex) {
      state.splitTabId = state.tabs[saved.splitIndex].id;
    }
  } catch {
    state.tabs = [newTerminalTab(state.selectedId)];
    state.activeTabId = state.tabs[0].id;
  }
}

function persistWorkspace() {
  try {
    localStorage.setItem("sshking.workspace.v1", JSON.stringify({
      serverIds: state.tabs.map((tab) => tab.serverId),
      tabIds: state.tabs.map((tab) => tab.id),
      titles: state.tabs.map((tab) => tab.title),
      manualTitles: state.tabs.map((tab) => tab.manualTitle),
      activeIndex: Math.max(0, state.tabs.findIndex((tab) => tab.id === state.activeTabId)),
      splitIndex: state.tabs.findIndex((tab) => tab.id === state.splitTabId),
      sessionOpen: state.tabs.map((tab) => tab.restoreSession || tab.connection === "connected" || tab.connection === "connecting"),
    }));
  } catch {
    // Workspace persistence is a convenience; private WebViews may disable storage.
  }
}

function restoreSidebarState() {
  try {
    const saved = JSON.parse(localStorage.getItem("sshking.sidebar.v1") ?? "{}") as {
      personalExpanded?: boolean;
      teamsExpanded?: boolean;
      collapsedTeamIds?: string[];
    };
    state.personalExpanded = saved.personalExpanded ?? true;
    state.teamsExpanded = saved.teamsExpanded ?? true;
    state.collapsedTeamIds = new Set(saved.collapsedTeamIds ?? []);
  } catch {
    state.personalExpanded = true;
    state.teamsExpanded = true;
    state.collapsedTeamIds.clear();
  }
}

function persistSidebarState() {
  try {
    localStorage.setItem("sshking.sidebar.v1", JSON.stringify({
      personalExpanded: state.personalExpanded,
      teamsExpanded: state.teamsExpanded,
      collapsedTeamIds: Array.from(state.collapsedTeamIds),
    }));
  } catch {
    // Sidebar expansion is only a local convenience.
  }
}

function activeTab() {
  return state.tabs.find((tab) => tab.id === state.activeTabId) ?? state.tabs[0];
}

function visibleTabs() {
  const active = activeTab();
  const split = state.tabs.find((tab) => tab.id === state.splitTabId);
  return [active, split && split.id !== active?.id ? split : undefined].filter(Boolean) as TerminalTab[];
}

function selectTab(tabId: string, focusTerminal = true) {
  const tab = state.tabs.find((item) => item.id === tabId);
  if (!tab) return;
  state.activeTabId = tab.id;
  state.selectedId = tab.serverId;
  render();
  requestAnimationFrame(() => {
    if (focusTerminal) tab.terminal?.focus();
    if (focusTerminal && tab.restoreSession && tab.connection === "idle") void autoConnectTab(tab, true);
  });
}

function addTerminalTab(serverId = state.selectedId) {
  const tab = newTerminalTab(serverId);
  state.tabs.push(tab);
  state.activeTabId = tab.id;
  state.selectedId = tab.serverId;
  render();
  scheduleCloudSync();
  requestAnimationFrame(() => {
    tab.terminal?.focus();
    void autoConnectTab(tab);
  });
}

async function closeTerminalTab(tabId: string) {
  const index = state.tabs.findIndex((tab) => tab.id === tabId);
  if (index < 0) return;
  const [tab] = state.tabs.splice(index, 1);
  tab.restoreSession = false;
  tab.lastRequest = undefined;
  window.clearTimeout(tab.reconnectTimer);
  tab.reconnectTimer = undefined;
  if (tab.connection === "connected" || tab.connection === "connecting") {
    await backend("Disconnect", tab.id).catch(() => undefined);
  }
  await backend("ClearSessionTranscript", tab.id).catch(() => undefined);
  state.tunnels = state.tunnels.filter((tunnel) => tunnel.sessionId !== tab.id);
  tab.terminal?.dispose();
  if (state.splitTabId === tab.id) state.splitTabId = "";
  if (!state.tabs.length) state.tabs.push(newTerminalTab());
  if (state.activeTabId === tab.id) {
    const next = state.tabs[Math.min(index, state.tabs.length - 1)];
    state.activeTabId = next.id;
    state.selectedId = next.serverId;
  }
  render();
  scheduleCloudSync({ tab: tabId });
}

function toggleSplitPane() {
  let created: TerminalTab | undefined;
  if (state.splitTabId) {
    state.splitTabId = "";
  } else {
    let split = state.tabs.find((tab) => tab.id !== state.activeTabId);
    if (!split) {
      split = newTerminalTab(state.selectedId);
      state.tabs.push(split);
      created = split;
    }
    state.splitTabId = split.id;
  }
  render();
  if (created) scheduleCloudSync();
  if (created) requestAnimationFrame(() => void autoConnectTab(created));
}

async function autoConnectTab(tab: TerminalTab, force = false) {
  if ((!force && !state.config.preferences.autoConnectTabs) || !tab.serverId || tab.connection !== "idle") return;
  const server = state.config.servers.find((item) => item.id === tab.serverId);
  if (!server) return;
  if (state.readiness[server.id] && !state.readiness[server.id].ready) return;
  await attemptConnection({
    tabId: tab.id,
    password: "",
    rememberPassword: Boolean(server.passwordSaved),
    requireBiometric: Boolean(server.requireBiometric),
  }, false);
}

function render() {
  persistWorkspace();
  const active = activeTab();
  const selected = state.config.servers.find((server) => server.id === active?.serverId);
  for (const tab of state.tabs) {
    if (tab.host?.isConnected) tab.host.remove();
  }
  app.innerHTML = `
    <main class="window-shell platform-${state.platform} theme-${state.config.preferences.theme}" style="--ui-scale:${state.config.preferences.uiScale / 100}">
      <div class="ambient ambient-a"></div>
      <div class="ambient ambient-b"></div>
      <header class="titlebar">
        <div class="brand">
          <span class="brand-mark">${icons.terminal}</span>
          <span>SSHKing</span>
        </div>
        ${accountButtonMarkup()}
        ${windowControlsMarkup()}
      </header>

      <section class="workspace">
        <aside class="sidebar glass-panel">
          <div class="sidebar-heading">
            <div>
              <span class="eyebrow">Your space</span>
              <h1>Servers</h1>
            </div>
            <div class="sidebar-heading-actions">
              <button class="orb-button import-button" id="import-ssh-config" aria-label="Import SSH config" title="Import ~/.ssh/config">⇩</button>
              <button class="orb-button" id="add-server" aria-label="Add server">${icons.plus}</button>
            </div>
          </div>
          <label class="searchbox">
            ${icons.search}
            <input id="server-search" value="${escapeHtml(state.query)}" placeholder="Search servers" autocomplete="off" />
            <kbd>${state.platform === "darwin" ? "⌘ K" : "Ctrl K"}</kbd>
          </label>
          <nav class="server-list">${serverList()}</nav>
          <div class="sidebar-footer">
            <button class="footer-button" id="open-settings">
              <span class="footer-icon">${icons.settings}</span>
              <span><strong>${state.platform === "darwin" ? "Preferences" : "Settings"}</strong><small>${state.config.preferences.logActivity ? "Activity logging on" : "Private mode"}</small></span>
              ${icons.chevron}
            </button>
          </div>
        </aside>

        <section class="terminal-panel glass-panel">
          <div class="terminal-toolbar">
            <div class="server-identity">
              <span class="server-glyph">${icons.server}</span>
              <div>
                <div class="breadcrumb"><span>Servers</span><b>/</b><strong>${escapeHtml(selected?.name ?? "No server selected")}</strong></div>
                <div class="endpoint">${selected ? `${escapeHtml(selected.user)}@${escapeHtml(selected.host)}:${selected.port}` : "Add your first server to begin"}</div>
              </div>
            </div>
            <div class="toolbar-actions">
              <button class="key-button" id="setup-key" ${selected ? "" : "disabled"} aria-label="Set up SSH key">${icons.key}<span>Key</span></button>
              <button class="key-button" id="open-files" ${active?.connection === "connected" ? "" : "disabled"} aria-label="Browse remote files">${icons.server}<span>Files</span></button>
              <button class="key-button" id="open-tunnels" ${active?.connection === "connected" ? "" : "disabled"} aria-label="Manage port forwards">${icons.arrow}<span>Tunnels</span></button>
              <div class="zed-split">
                <button class="zed-button" id="open-zed-direct" ${selected ? "" : "disabled"} aria-label="Open current folder in Zed">${icons.code}<span>Zed</span></button>
                <button class="zed-options" id="open-zed-options" ${selected ? "" : "disabled"} aria-label="Zed options">${icons.more}</button>
              </div>
              <button class="icon-button" id="edit-server" ${selected ? "" : "disabled"} aria-label="Edit server">${icons.more}</button>
              <button class="connect-button ${active?.connection === "connected" ? "connected" : ""}" id="connect-button" ${selected ? "" : "disabled"}>
                <span>${active?.connection === "connected" ? "Disconnect" : active?.connection === "connecting" ? "Connecting…" : "Connect"}</span>
                ${icons.arrow}
              </button>
            </div>
          </div>

          <div class="tabbar">
            <div class="terminal-tabs">${state.tabs.map((tab) => `<button class="terminal-tab ${tab.id === state.activeTabId ? "active" : ""}" data-tab="${tab.id}" title="${escapeHtml(tab.manualTitle ? "Custom name · click to rename" : "Last command · click to set a permanent name")}"><span class="status-dot ${tab.connection}"></span><span class="tab-title" data-tab-title="${tab.id}">${escapeHtml(tab.title)}</span><i class="tab-close" data-close-tab="${tab.id}" aria-label="Close terminal">×</i></button>`).join("")}</div>
            <button class="new-tab" id="new-terminal-tab" aria-label="New terminal">${icons.plus}</button>
            <button class="split-button ${state.splitTabId ? "active" : ""}" id="toggle-split" aria-label="Toggle split terminal" title="Split terminal">▥</button>
            <span class="encryption"><i></i> end-to-end SSH</span>
          </div>

          <div class="terminal-stage" id="terminal-stage">
            <div class="terminal-glow"></div>
            ${state.terminalSearch ? `<div class="terminal-searchbar">
              ${icons.search}<input id="terminal-search-input" value="${escapeHtml(state.terminalSearchQuery)}" placeholder="Find in terminal">
              <button id="terminal-search-previous" title="Previous result">↑</button><button id="terminal-search-next" title="Next result">↓</button>
              <button id="add-terminal-bookmark" title="Bookmark current position">☆</button>
              ${active?.bookmarks.map((bookmark) => `<button class="terminal-bookmark" data-terminal-bookmark="${bookmark.line}" title="${escapeHtml(bookmark.label)}">${escapeHtml(bookmark.label)}</button>`).join("") ?? ""}
              <button id="close-terminal-search" title="Close">×</button>
            </div>` : ""}
            <div class="terminal-layout ${state.splitTabId ? "split" : ""}">
              ${visibleTabs().map((tab) => `<div class="terminal-pane ${tab.id === state.activeTabId ? "active" : ""}" data-pane-tab="${tab.id}"><div class="terminal-output" data-terminal-slot="${tab.id}"></div></div>`).join("")}
            </div>
            <div class="terminal-copy-toast" id="terminal-copy-toast">Copied</div>
          </div>
        </section>
      </section>
      ${state.modal ? modalMarkup() : ""}
    </main>`;
  bindEvents();
  mountVisibleTerminals();
}

function windowControlsMarkup() {
  if (state.platform !== "windows") return "";
  return `<div class="window-controls" aria-label="Window controls">
    <button data-window="min" aria-label="Minimise" title="Minimise"><span></span></button>
    <button data-window="max" aria-label="Maximise" title="Maximise"><span class="max-icon"></span></button>
    <button data-window="close" class="close" aria-label="Close" title="Close"><span></span></button>
  </div>`;
}

function serverList() {
  const query = state.query.toLowerCase().trim();
  const matches = (server: Server, teamName = "") => !query ||
    `${server.name} ${server.host} ${server.user} ${server.group ?? ""} ${teamName}`.toLowerCase().includes(query);
  const personal = state.config.servers.filter((server) => !server.teamId && matches(server));
  const personalOpen = Boolean(query) || state.personalExpanded;
  const teamsOpen = Boolean(query) || state.teamsExpanded;

  const personalSection = `<section class="server-scope">
    <div class="scope-heading">
      <button type="button" class="scope-toggle" data-toggle-scope="personal"><span class="fold-caret ${personalOpen ? "open" : ""}">${icons.chevron}</span><strong>Personal Servers</strong><small>${personal.length}</small></button>
      <button type="button" class="scope-add" data-add-server-team="" title="Add personal server" aria-label="Add personal server">${icons.plus}</button>
    </div>
    ${personalOpen ? `<div class="scope-content">${serverCardsMarkup(personal) || emptyScopeMarkup(query ? "No matching personal servers" : "No personal servers yet")}</div>` : ""}
  </section>`;

  const teamSections = state.config.teams.map((team) => {
    const servers = state.config.servers.filter((server) => server.teamId === team.id && matches(server, team.name));
    if (query && !servers.length && !team.name.toLowerCase().includes(query)) return "";
    const open = Boolean(query) || !state.collapsedTeamIds.has(team.id);
    return `<section class="team-scope">
      <div class="team-heading">
        <button type="button" class="team-toggle" data-toggle-team="${escapeHtml(team.id)}"><span class="fold-caret ${open ? "open" : ""}">${icons.chevron}</span><span class="team-avatar">${initials(team.name)}</span><strong>${escapeHtml(team.name)}</strong><small>${servers.length}</small></button>
        <button type="button" class="scope-add" data-add-server-team="${escapeHtml(team.id)}" title="Add server to ${escapeHtml(team.name)}" aria-label="Add team server">${icons.plus}</button>
        <button type="button" class="scope-more" data-edit-team="${escapeHtml(team.id)}" title="Rename team" aria-label="Rename team">${icons.more}</button>
      </div>
      ${open ? `<div class="team-content">${serverCardsMarkup(servers) || emptyScopeMarkup(query ? "No matching servers" : "No servers in this team")}</div>` : ""}
    </section>`;
  }).join("");

  const teamsSection = `<section class="server-scope teams-scope">
    <div class="scope-heading">
      <button type="button" class="scope-toggle" data-toggle-scope="teams"><span class="fold-caret ${teamsOpen ? "open" : ""}">${icons.chevron}</span><strong>Team Servers</strong><small>${state.config.teams.length}</small></button>
      <button type="button" class="scope-add" id="create-team" title="Create team" aria-label="Create team">${icons.plus}</button>
    </div>
    ${teamsOpen ? `<div class="scope-content teams-content">${teamSections || `<button type="button" class="empty-team" id="create-first-team"><span>${icons.plus}</span><strong>Create your first team</strong><small>Name a workspace and add shared servers</small></button>`}</div>` : ""}
  </section>`;

  return personalSection + teamsSection;
}

function serverCardsMarkup(servers: Server[]) {
  return [...servers].sort((left, right) => Number(Boolean(right.favorite)) - Number(Boolean(left.favorite)) || left.name.localeCompare(right.name)).map(serverCard).join("");
}

function emptyScopeMarkup(message: string) {
  return `<div class="empty-scope">${escapeHtml(message)}</div>`;
}

function serverCard(server: Server) {
  const active = server.id === state.selectedId;
  const online = state.tabs.some((tab) => tab.serverId === server.id && tab.connection === "connected");
  return `<button class="server-card ${active ? "active" : ""}" data-server="${escapeHtml(server.id)}">
    <span class="server-avatar">${initials(server.name)}</span>
    <span class="server-copy"><strong>${escapeHtml(server.name)}</strong><small>${escapeHtml(server.user)}@${escapeHtml(server.host)}</small></span>
    <span class="server-indicators">
      ${state.readiness[server.id] && !state.readiness[server.id].ready ? `<span class="server-missing" title="${escapeHtml(state.readiness[server.id].message)}" aria-label="${escapeHtml(state.readiness[server.id].message)}">!</span>` : ""}
      <span class="server-state ${online ? "online" : ""}"></span>
    </span>
  </button>`;
}

function modalMarkup() {
  if (state.modal === "account") {
    return `<div class="modal-backdrop account-backdrop"><section class="modal glass-modal account-modal">
      <div class="modal-handle"></div>
      <div class="modal-header"><div><span class="eyebrow">SSHKing Cloud</span><h2>${state.cloud.signedIn ? "Your account" : "Log in"}</h2></div><button type="button" class="modal-close">×</button></div>
      <p class="modal-note">Use one identity across devices and team workspaces. Your SSH passwords and private keys remain on this device.</p>
      ${cloudAccountMarkup()}
      <div class="modal-actions"><button type="button" class="ghost" id="account-settings">Cloud settings</button><button type="button" class="ghost modal-close">Close</button></div>
    </section></div>`;
  }
  if (state.modal === "team") {
    const existing = state.config.teams.find((team) => team.id === state.editingTeamId);
    return `<div class="modal-backdrop"><form class="modal glass-modal team-modal" id="team-form">
      <div class="modal-handle"></div>
      <div class="modal-header"><div><span class="eyebrow">Team workspace</span><h2>${existing ? "Rename team" : "Create a team"}</h2></div><button type="button" class="modal-close">×</button></div>
      <p class="modal-note">Teams group shared server definitions. Cloud membership and synchronization will plug into this workspace in the next phase.</p>
      <input type="hidden" name="id" value="${escapeHtml(existing?.id ?? "")}">
      ${field("Team name", "name", existing?.name ?? "", "text", "Platform, Operations, Client Acme")}
      <div class="modal-actions">
        ${existing ? `<button type="button" class="danger" id="delete-team">${icons.trash} Delete team</button>` : "<span></span>"}
        <div><button type="button" class="ghost modal-close">Cancel</button><button class="primary">${existing ? "Save name" : "Create team"}</button></div>
      </div>
    </form></div>`;
  }
  if (state.modal === "activity") {
    return `<div class="modal-backdrop"><section class="modal glass-modal activity-modal">
      <div class="modal-handle"></div>
      <div class="modal-header"><div><span class="eyebrow">Local history</span><h2>Activity log</h2></div><button type="button" class="modal-close">×</button></div>
      <p class="modal-note">The newest opt-in events are shown first. Logs stay on this device.</p>
      <div class="activity-list">${state.activity.map((line) => `<div class="activity-line">${escapeHtml(line)}</div>`).join("") || `<div class="files-empty">No activity has been logged.</div>`}</div>
      <div class="modal-actions"><span class="modal-note-inline">Disable activity logs in Preferences for private sessions.</span><button type="button" class="ghost modal-close">Close</button></div>
    </section></div>`;
  }
  if (state.modal === "palette") {
    return `<div class="modal-backdrop palette-backdrop"><section class="modal glass-modal palette-modal">
      <label class="palette-search">${icons.search}<input id="palette-input" value="${escapeHtml(state.paletteQuery)}" placeholder="Search actions or type a shell command…" autocomplete="off" spellcheck="false"><kbd>Esc</kbd></label>
      <div class="palette-results">
        ${paletteActions().map((action) => `<button type="button" class="palette-action" data-palette-action="${action.id}" data-palette-search="${escapeHtml(`${action.label} ${action.detail}`.toLowerCase())}">
          <span>${action.icon}</span><span><strong>${escapeHtml(action.label)}</strong><small>${escapeHtml(action.detail)}</small></span>${action.shortcut ? `<kbd>${escapeHtml(action.shortcut)}</kbd>` : ""}
        </button>`).join("")}
      </div>
      <div class="palette-footer"><span>Type any command and press Enter to send it to the active terminal.</span><span>↑↓ Navigate · Enter Run</span></div>
    </section></div>`;
  }
  if (state.modal === "tunnels") {
    const tab = activeTab();
    const tunnels = state.tunnels.filter((tunnel) => tunnel.sessionId === tab?.id);
    return `<div class="modal-backdrop"><section class="modal glass-modal tunnels-modal">
      <div class="modal-handle"></div>
      <div class="modal-header"><div><span class="eyebrow">Port forwarding</span><h2>Local tunnels</h2></div><button type="button" class="modal-close">×</button></div>
      <p class="modal-note">Expose a service reachable by the server on localhost. Use local port 0 to choose a free port automatically.</p>
      <form id="tunnel-form" class="tunnel-form">
        ${field("Local port", "localPort", "0", "number", "0")}
        ${field("Remote host", "remoteHost", "127.0.0.1", "text", "127.0.0.1")}
        ${field("Remote port", "remotePort", "5432", "number", "5432")}
        <button class="primary">Start tunnel</button>
      </form>
      <div class="tunnel-list">${tunnels.map((tunnel) => `<div class="tunnel-row">
        <i></i><span><strong>${escapeHtml(tunnel.local)}</strong><small>→ ${escapeHtml(tunnel.remoteHost)}:${tunnel.remotePort}</small></span>
        <button type="button" data-stop-tunnel="${tunnel.id}">Stop</button>
      </div>`).join("") || `<div class="files-empty tunnel-empty">No active tunnels for this tab.</div>`}</div>
      <div class="modal-actions"><span class="modal-note-inline">Tunnels close automatically when this terminal disconnects.</span><button type="button" class="ghost modal-close">Close</button></div>
    </section></div>`;
  }
  if (state.modal === "files") {
    const rows = state.remoteFiles.map((file) => `<button type="button" class="remote-file-row" data-remote-path="${escapeHtml(file.path)}" data-remote-dir="${file.isDir}">
      <span class="remote-file-icon">${file.isDir ? "⌑" : "·"}</span>
      <span class="remote-file-name">${escapeHtml(file.name)}</span>
      <small>${file.isDir ? "Folder" : formatBytes(file.size)}</small>
      ${file.isDir ? `<span class="remote-file-open">${icons.chevron}</span>` : `<span class="remote-file-download" data-download-path="${escapeHtml(file.path)}">Download</span>`}
    </button>`).join("");
    return `<div class="modal-backdrop"><section class="modal glass-modal files-modal">
      <div class="modal-handle"></div>
      <div class="modal-header"><div><span class="eyebrow">Secure file transfer</span><h2>Remote files</h2></div><button type="button" class="modal-close">×</button></div>
      <div class="files-address">
        <button type="button" id="remote-parent" aria-label="Parent folder">↑</button>
        <span>${escapeHtml(state.browsingPath)}</span>
        <button type="button" id="upload-remote-file">${icons.plus} Upload</button>
      </div>
      <div class="remote-file-list">${state.filesLoading ? `<div class="files-empty">Loading…</div>` : rows || `<div class="files-empty">This folder is empty.</div>`}</div>
      <div class="modal-actions"><span class="modal-note-inline">SFTP uses this tab’s encrypted SSH connection.</span><button type="button" class="ghost modal-close">Close</button></div>
    </section></div>`;
  }
  if (state.modal === "ssh-key") {
    const selected = state.config.servers.find((server) => server.id === state.selectedId);
    const generateChecked = state.sshKeys.length === 0 ? "checked" : "";
    const keyOptions = state.sshKeys.map((key, index) => `<label class="key-choice">
      <input type="radio" name="publicKeyPath" value="${escapeHtml(key.path)}" ${index === 0 ? "checked" : ""}>
      <span class="key-choice-icon">${icons.key}</span>
      <span><strong>${escapeHtml(key.name)}</strong><small>${escapeHtml(key.fingerprint)}</small><em>${escapeHtml(key.path)}</em></span>
      <i></i>
    </label>`).join("");
    return `<div class="modal-backdrop"><form class="modal glass-modal key-modal" id="ssh-key-form">
      <div class="modal-handle"></div>
      <div class="modal-header"><div><span class="eyebrow">Passwordless access</span><h2>Set up SSH key</h2></div><button type="button" class="modal-close">×</button></div>
      <p class="modal-note">Install a public key on <strong>${escapeHtml(selected?.name ?? "this server")}</strong>. This is the cross-platform equivalent of <code>ssh-copy-id</code> and will not add the key twice.</p>
      <div class="key-choice-list">
        ${keyOptions}
        <label class="key-choice generate">
          <input type="radio" name="publicKeyPath" value="__generate__" ${generateChecked}>
          <span class="key-choice-icon">${icons.plus}</span>
          <span><strong>Generate a dedicated Ed25519 key</strong><small>Creates sshking_${escapeHtml(keyName(selected?.name ?? "server"))} in ~/.ssh</small><em>Recommended when you want a key only for this server</em></span>
          <i></i>
        </label>
      </div>
      <label class="form-field key-password"><span>Server password</span><input name="password" type="password" placeholder="${selected?.passwordSaved ? "Saved securely · leave blank to use" : "Required if a key or agent cannot connect"}" autocomplete="current-password"></label>
      ${selected?.passwordSaved ? `<div class="credential-status"><i></i><span>Saved password available${selected.requireBiometric ? ` · protected by ${escapeHtml(state.biometricName)}` : ""}</span></div>` : ""}
      <div class="modal-actions"><span></span><div><button type="button" class="ghost modal-close">Cancel</button><button class="primary">Install key</button></div></div>
    </form></div>`;
  }
  if (state.modal === "zed") {
    const selected = state.config.servers.find((server) => server.id === state.selectedId);
    return `<div class="modal-backdrop"><form class="modal glass-modal zed-modal" id="zed-form">
      <div class="modal-handle"></div>
      <div class="modal-header"><div><span class="eyebrow">Remote editing</span><h2>Open in Zed</h2></div><button type="button" class="modal-close">×</button></div>
      <p class="modal-note">Open a remote file or folder on <strong>${escapeHtml(selected?.name ?? "this server")}</strong>. Zed normally uses your system SSH agent or configured keys. Passing the saved password is optional and less secure.</p>
      <label class="form-field"><span>Remote path</span><input name="remotePath" value="${escapeHtml(currentRemotePath())}" placeholder="~/project or /etc/nginx/nginx.conf" autocomplete="off" spellcheck="false" required></label>
      <div class="zed-destination">${escapeHtml(selected ? `${selected.user}@${selected.host}:${selected.port}` : "")}</div>
      ${selected?.identity ? `<div class="zed-key-status">${icons.key}<span>Using private key <strong>${escapeHtml(selected.identity)}</strong></span></div>` : ""}
      <label class="switch-row zed-window-option">
        <span><strong>Open in a new window</strong><small>Keep the remote project separate from existing Zed workspaces</small></span>
        <input name="newWindow" type="checkbox" checked><i></i>
      </label>
      ${selected?.passwordSaved ? `<label class="switch-row zed-password-option">
        <span><strong>Pass saved password to Zed</strong><small>Less secure: the password may be visible in process arguments or retained by Zed</small></span>
        <input name="passSavedPassword" type="checkbox"><i></i>
      </label>` : ""}
      <div class="modal-actions"><span></span><div><button type="button" class="ghost modal-close">Cancel</button><button class="primary">Open in Zed</button></div></div>
    </form></div>`;
  }
  if (state.modal === "connect") {
    const selected = state.config.servers.find((server) => server.id === state.selectedId);
    const saved = selected?.passwordSaved ?? false;
    return `<div class="modal-backdrop"><form class="modal glass-modal connect-modal" id="connect-form">
      <div class="modal-handle"></div>
      <div class="modal-header"><div><span class="eyebrow">Secure session</span><h2>Connect to ${escapeHtml(selected?.name ?? "server")}</h2></div><button type="button" class="modal-close">×</button></div>
      <p class="modal-note">SSH agent and your configured private key are tried automatically. Saved passwords stay in the operating system’s secure credential store—not in SSHKing’s config.</p>
      <label class="form-field"><span>Session password</span><input name="password" type="password" placeholder="${saved ? `Saved securely · leave blank to use` : "Optional"}" autocomplete="current-password"></label>
      ${saved ? `<div class="credential-status"><i></i><span>Saved in secure storage${selected?.requireBiometric ? ` · protected by ${escapeHtml(state.biometricName)}` : ""}</span></div>` : ""}
      <div class="credential-options">
        <label class="switch-row">
          <span><strong>Save password securely</strong><small>${state.biometricName === "Touch ID" ? "Stored in macOS Keychain" : "Stored in Windows Credential Manager"}</small></span>
          <input name="rememberPassword" type="checkbox" ${saved ? "checked" : ""}><i></i>
        </label>
        <label class="switch-row biometric-row ${state.biometricAvailable ? "" : "unavailable"}">
          <span><strong>Require ${escapeHtml(state.biometricName)}</strong><small>${state.biometricAvailable ? "Verify your identity before every retrieval" : "Not configured or unavailable on this device"}</small></span>
          <input name="requireBiometric" type="checkbox" ${selected?.requireBiometric ? "checked" : ""} ${state.biometricAvailable && saved ? "" : "disabled"}><i></i>
        </label>
      </div>
      <div class="modal-actions"><span></span><div><button type="button" class="ghost modal-close">Cancel</button><button class="primary">Open secure session</button></div></div>
    </form></div>`;
  }
  if (state.modal === "trust-host-key") {
    const selected = state.config.servers.find((server) => server.id === state.selectedId);
    return `<div class="modal-backdrop"><section class="modal glass-modal connect-modal" id="trust-host-key-modal">
      <div class="modal-handle"></div>
      <div class="modal-header"><div><span class="eyebrow">First connection</span><h2>Trust ${escapeHtml(selected?.name ?? "this server")}?</h2></div><button type="button" class="modal-close">×</button></div>
      <p class="modal-note">This server presented a host key that SSHKing has not seen before. Verify the fingerprint with your administrator before trusting it. SSHKing will pin it to this server after connecting.</p>
      <div class="host-key-fingerprint">${escapeHtml(state.pendingHostFingerprint)}</div>
      <div class="modal-actions"><span></span><div><button type="button" class="ghost modal-close">Cancel</button><button type="button" class="primary" id="trust-host-key">Trust and connect</button></div></div>
    </section></div>`;
  }
  if (state.modal === "settings") {
    const p = state.config.preferences;
    return `<div class="modal-backdrop"><form class="modal glass-modal settings-modal" id="settings-form">
      <div class="modal-handle"></div>
      <div class="modal-header"><div><span class="eyebrow">Application</span><h2>Settings</h2></div><button type="button" class="modal-close">×</button></div>
      <div class="settings-content">
        <section class="settings-section appearance-settings">
          <div class="settings-section-title"><strong>Appearance</strong><small>Theme and interface sizing apply immediately</small></div>
          <div class="theme-picker">
            ${themeChoice("glass", "Glass", "Transparent acrylic", p.theme)}
            ${themeChoice("light", "Light", "Clean and bright", p.theme)}
            ${themeChoice("black", "Black", "High contrast dark", p.theme)}
          </div>
          ${rangeControl("Interface scale", "uiScale", p.uiScale, 80, 140, 5, "%")}
        </section>
        <section class="settings-section cloud-settings">
          <div class="settings-section-title"><strong>SSHKing Cloud</strong><small>Sync personal and team servers across devices. SSH credentials always stay local.</small></div>
          <div class="cloud-url-row">
            ${field("Cloud server", "cloudUrl", p.cloudUrl ?? "", "url", "https://cloud.example.com", false)}
            <button type="button" class="history-button compact" id="cloud-check">Check</button>
          </div>
          ${cloudAccountMarkup()}
        </section>
        <div class="settings-columns">
          <section class="settings-section">
            <div class="settings-section-title"><strong>Terminal</strong><small>Text rendering and scrollback</small></div>
            ${rangeControl("Font size", "terminalFontSize", p.terminalFontSize, 10, 28, 1, " px")}
            ${rangeControl("Line spacing", "terminalLineHeight", p.terminalLineHeight, 100, 200, 5, "%")}
            <label class="form-field"><span>Font family</span><select name="terminalFontFamily">
              <option value="system-mono" ${p.terminalFontFamily === "system-mono" ? "selected" : ""}>System monospace</option>
              <option value="cascadia" ${p.terminalFontFamily === "cascadia" ? "selected" : ""}>Cascadia Mono</option>
              <option value="jetbrains" ${p.terminalFontFamily === "jetbrains" ? "selected" : ""}>JetBrains Mono</option>
              <option value="source-code" ${p.terminalFontFamily === "source-code" ? "selected" : ""}>Source Code Pro</option>
            </select></label>
            ${field("Scrollback lines", "scrollback", String(p.scrollback), "number")}
          </section>
          <section class="settings-section">
            <div class="settings-section-title"><strong>Connection defaults</strong><small>Used when adding servers</small></div>
            ${field("Default user", "defaultUser", p.defaultUser)}
            ${field("Default port", "defaultPort", String(p.defaultPort), "number")}
            ${field("Default shell", "defaultShell", p.defaultShell)}
            ${field("Default identity", "defaultIdentity", p.defaultIdentity ?? "", "text", "~/.ssh/id_ed25519", false)}
            <label class="switch-row auto-connect-setting"><span><strong>Auto-connect new tabs</strong><small>Use agent, key, or saved credential immediately</small></span><input name="autoConnectTabs" type="checkbox" ${p.autoConnectTabs ? "checked" : ""}><i></i></label>
            <label class="switch-row auto-connect-setting"><span><strong>Reopen active session</strong><small>Reconnect the active terminal when SSHKing starts</small></span><input name="reopenActiveSession" type="checkbox" ${p.reopenActiveSession ? "checked" : ""}><i></i></label>
          </section>
        </div>
        <section class="settings-section privacy-settings">
          <div class="settings-section-title"><strong>Privacy and history</strong><small>Activity never leaves this device</small></div>
          <label class="switch-row"><span><strong>Activity logs</strong><small>Store connection events and palette commands locally</small></span><input name="logActivity" type="checkbox" ${p.logActivity ? "checked" : ""}><i></i></label>
          <label class="switch-row"><span><strong>Save terminal history</strong><small>Replay terminal commands and output when a tab reopens</small></span><input name="persistTerminalHistory" type="checkbox" ${p.persistTerminalHistory ? "checked" : ""}><i></i></label>
          <div class="history-actions">
            <button type="button" class="history-button" id="open-activity">${icons.search}<span><strong>View activity history</strong><small>Inspect recent connections and commands</small></span>${icons.chevron}</button>
            <button type="button" class="history-button" id="clear-terminal-history">${icons.trash}<span><strong>Clear saved terminal history</strong><small>Remove locally stored terminal transcripts</small></span>${icons.chevron}</button>
          </div>
        </section>
      </div>
      <div class="modal-actions"><button type="button" class="ghost modal-close">Cancel</button><button class="primary">Save preferences</button></div>
    </form></div>`;
  }
  const existing = state.config.servers.find((server) => server.id === state.editingId);
  const p = state.config.preferences;
  return `<div class="modal-backdrop"><form class="modal glass-modal" id="server-form">
    <div class="modal-handle"></div>
    <div class="modal-header"><div><span class="eyebrow">Connection</span><h2>${existing ? "Edit server" : "New server"}</h2></div><button type="button" class="modal-close">×</button></div>
    <input type="hidden" name="id" value="${escapeHtml(existing?.id ?? "")}">
    <div class="form-grid two">
      ${field("Display name", "name", existing?.name ?? "", "text", "Production")}
      ${field("Group (optional)", "group", existing?.group ?? "", "text", "Work, Personal, Clients", false)}
      <label class="form-field full"><span>Workspace</span><select name="teamId">
        <option value="">Personal Servers</option>
        ${state.config.teams.map((team) => `<option value="${escapeHtml(team.id)}" ${(existing?.teamId ?? state.pendingServerTeamId) === team.id ? "selected" : ""}>${escapeHtml(team.name)} · Team Servers</option>`).join("")}
      </select></label>
      ${field("Host", "host", existing?.host ?? "", "text", "server.example.com")}
      ${field("User", "user", existing?.user ?? p.defaultUser)}
      ${field("Port", "port", String(existing?.port ?? p.defaultPort), "number")}
      ${field("Remote shell", "shell", existing?.shell ?? p.defaultShell, "text", "default, zsh, bash, fish")}
      ${field("tmux session prefix", "tmuxSession", existing?.tmuxSession ?? "sshking", "text", "sshking", false)}
      ${field("Private key (optional)", "identity", existing?.identity ?? p.defaultIdentity ?? "", "text", "~/.ssh/id_ed25519", false)}
      <label class="form-field"><span>Jump host (optional)</span><select name="jumpServerId">
        <option value="">Direct connection</option>
        ${state.config.servers.filter((server) => server.id !== existing?.id).map((server) => `<option value="${escapeHtml(server.id)}" ${server.id === existing?.jumpServerId ? "selected" : ""}>${escapeHtml(server.name)} · ${escapeHtml(server.user)}@${escapeHtml(server.host)}</option>`).join("")}
      </select></label>
      <div class="full">${field("Pinned fingerprint (optional)", "fingerprint", existing?.fingerprint ?? "", "text", "SHA256:…", false)}</div>
      <label class="switch-row full">
        <span><strong>Pin this server</strong><small>Keep it at the top of the server list</small></span>
        <input name="favorite" type="checkbox" ${existing?.favorite ? "checked" : ""}><i></i>
      </label>
      <label class="switch-row full">
        <span><strong>Persistent tmux session</strong><small>Keep programs running and reattach after reconnecting</small></span>
        <input name="useTmux" type="checkbox" ${existing ? (existing.useTmux ? "checked" : "") : "checked"}><i></i>
      </label>
    </div>
    <div class="modal-actions">
      ${existing ? `<button type="button" class="danger" id="delete-server">${icons.trash} Delete</button>` : "<span></span>"}
      <div><button type="button" class="ghost modal-close">Cancel</button><button class="primary">Save server</button></div>
    </div>
  </form></div>`;
}

function field(label: string, name: string, value: string, type = "text", placeholder = "", required = true) {
  return `<label class="form-field"><span>${label}</span><input name="${name}" type="${type}" value="${escapeHtml(value)}" placeholder="${escapeHtml(placeholder)}"${required ? " required" : ""}></label>`;
}

function bindEvents() {
  document.querySelector("#account-button")?.addEventListener("click", openAccount);
  document.querySelectorAll<HTMLElement>("[data-window]").forEach((button) => button.onclick = () => {
    const action = button.dataset.window;
    if (action === "min") window.runtime?.WindowMinimise();
    if (action === "max") window.runtime?.WindowToggleMaximise();
    if (action === "close") window.runtime?.Quit();
  });
  document.querySelectorAll<HTMLButtonElement>("[data-server]").forEach((button) => button.onclick = () => {
    state.selectedId = button.dataset.server ?? "";
    const tab = activeTab();
    if (tab && tab.connection === "idle") {
      if (tab.serverId !== state.selectedId) {
        void backend("ClearSessionTranscript", tab.id);
        tab.restoredTranscript = "";
      }
      tab.serverId = state.selectedId;
      if (!tab.manualTitle) tab.title = state.config.servers.find((server) => server.id === tab.serverId)?.name ?? "Terminal";
      resetTerminal(tab);
    }
    render();
  });
  document.querySelectorAll<HTMLElement>("[data-toggle-scope]").forEach((button) => button.addEventListener("click", () => {
    if (button.dataset.toggleScope === "personal") state.personalExpanded = !state.personalExpanded;
    if (button.dataset.toggleScope === "teams") state.teamsExpanded = !state.teamsExpanded;
    persistSidebarState();
    render();
  }));
  document.querySelectorAll<HTMLElement>("[data-toggle-team]").forEach((button) => button.addEventListener("click", () => {
    const teamId = button.dataset.toggleTeam ?? "";
    if (state.collapsedTeamIds.has(teamId)) state.collapsedTeamIds.delete(teamId);
    else state.collapsedTeamIds.add(teamId);
    persistSidebarState();
    render();
  }));
  document.querySelectorAll<HTMLElement>("[data-add-server-team]").forEach((button) => button.addEventListener("click", () => {
    state.editingId = "";
    state.pendingServerTeamId = button.dataset.addServerTeam ?? "";
    openModal("server");
  }));
  document.querySelectorAll<HTMLElement>("[data-edit-team]").forEach((button) => button.addEventListener("click", () => openTeamModal(button.dataset.editTeam ?? "")));
  document.querySelectorAll<HTMLButtonElement>("[data-tab]").forEach((button) => button.onclick = (event) => {
    if ((event.target as HTMLElement).closest("[data-close-tab]")) return;
    selectTab(button.dataset.tab ?? "");
  });
  document.querySelectorAll<HTMLElement>("[data-tab-title]").forEach((title) => title.onclick = (event) => {
    event.stopPropagation();
    const tabId = title.dataset.tabTitle ?? "";
    if (state.activeTabId !== tabId) {
      selectTab(tabId);
      return;
    }
    openTabRename(tabId);
  });
  document.querySelectorAll<HTMLElement>("[data-close-tab]").forEach((button) => button.onclick = (event) => {
    event.stopPropagation();
    void closeTerminalTab(button.dataset.closeTab ?? "");
  });
  document.querySelector("#new-terminal-tab")?.addEventListener("click", () => addTerminalTab());
  document.querySelector("#toggle-split")?.addEventListener("click", toggleSplitPane);
  const search = document.querySelector<HTMLInputElement>("#server-search");
  search?.addEventListener("input", () => { state.query = search.value; render(); });
  document.querySelector("#add-server")?.addEventListener("click", () => {
    state.editingId = "";
    state.pendingServerTeamId = "";
    openModal("server");
  });
  document.querySelector("#create-team")?.addEventListener("click", () => openTeamModal());
  document.querySelector("#create-first-team")?.addEventListener("click", () => openTeamModal());
  document.querySelector("#import-ssh-config")?.addEventListener("click", importSSHConfig);
  document.querySelector("#open-settings")?.addEventListener("click", () => openModal("settings"));
  document.querySelector("#open-activity")?.addEventListener("click", openActivity);
  document.querySelector("#clear-terminal-history")?.addEventListener("click", clearTerminalHistory);
  document.querySelector("#edit-server")?.addEventListener("click", () => {
    state.editingId = state.selectedId;
    openModal("server");
  });
  document.querySelector("#setup-key")?.addEventListener("click", openSSHKeyModal);
  document.querySelector("#open-files")?.addEventListener("click", openRemoteFiles);
  document.querySelector("#open-tunnels")?.addEventListener("click", () => { state.modal = "tunnels"; render(); });
  document.querySelector("#open-zed-direct")?.addEventListener("click", openCurrentFolderInZed);
  document.querySelector("#open-zed-options")?.addEventListener("click", () => {
    state.modal = "zed";
    render();
    setTimeout(() => document.querySelector<HTMLInputElement>('input[name="remotePath"]')?.select(), 20);
  });
  document.querySelectorAll(".modal-close").forEach((button) => button.addEventListener("click", closeModal));
  document.querySelector("#connect-button")?.addEventListener("click", toggleConnection);
  document.querySelector<HTMLFormElement>("#server-form")?.addEventListener("submit", saveServer);
  document.querySelector<HTMLFormElement>("#team-form")?.addEventListener("submit", saveTeam);
  document.querySelector("#delete-team")?.addEventListener("click", deleteTeam);
  document.querySelector<HTMLFormElement>("#settings-form")?.addEventListener("submit", saveSettings);
  document.querySelector("#cloud-check")?.addEventListener("click", () => void refreshCloudState());
  document.querySelectorAll<HTMLElement>("[data-cloud-login]").forEach((button) => button.addEventListener("click", () => void loginCloud(button.dataset.cloudLogin ?? "")));
  document.querySelector("#cloud-logout")?.addEventListener("click", () => void logoutCloud());
  document.querySelector("#account-settings")?.addEventListener("click", () => openModal("settings"));
  const settingsForm = document.querySelector<HTMLFormElement>("#settings-form");
  settingsForm?.addEventListener("input", () => previewSettings(settingsForm));
  settingsForm?.addEventListener("change", () => previewSettings(settingsForm));
  document.querySelector<HTMLFormElement>("#connect-form")?.addEventListener("submit", connectWithCredentials);
  document.querySelector("#trust-host-key")?.addEventListener("click", trustHostKey);
  document.querySelector<HTMLFormElement>("#zed-form")?.addEventListener("submit", openInZed);
  document.querySelector<HTMLFormElement>("#ssh-key-form")?.addEventListener("submit", installSSHKey);
  document.querySelector<HTMLFormElement>("#tunnel-form")?.addEventListener("submit", startLocalTunnel);
  document.querySelectorAll<HTMLElement>("[data-stop-tunnel]").forEach((button) => button.addEventListener("click", () => void stopTunnel(button.dataset.stopTunnel ?? "")));
  const paletteInput = document.querySelector<HTMLInputElement>("#palette-input");
  paletteInput?.addEventListener("input", () => filterPalette(paletteInput.value));
  paletteInput?.addEventListener("keydown", handlePaletteKeydown);
  document.querySelectorAll<HTMLElement>("[data-palette-action]").forEach((button) => button.addEventListener("click", () => void runPaletteAction(button.dataset.paletteAction ?? "")));
  const terminalSearchInput = document.querySelector<HTMLInputElement>("#terminal-search-input");
  terminalSearchInput?.addEventListener("input", () => {
    state.terminalSearchQuery = terminalSearchInput.value;
    activeTab()?.searchAddon?.findNext(terminalSearchInput.value, { incremental: true });
  });
  terminalSearchInput?.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      event.preventDefault();
      (event.shiftKey ? activeTab()?.searchAddon?.findPrevious(state.terminalSearchQuery) : activeTab()?.searchAddon?.findNext(state.terminalSearchQuery));
    }
  });
  document.querySelector("#terminal-search-previous")?.addEventListener("click", () => activeTab()?.searchAddon?.findPrevious(state.terminalSearchQuery));
  document.querySelector("#terminal-search-next")?.addEventListener("click", () => activeTab()?.searchAddon?.findNext(state.terminalSearchQuery));
  document.querySelector("#add-terminal-bookmark")?.addEventListener("click", addTerminalBookmark);
  document.querySelector("#close-terminal-search")?.addEventListener("click", closeTerminalSearch);
  document.querySelectorAll<HTMLElement>("[data-terminal-bookmark]").forEach((button) => button.addEventListener("click", () => activeTab()?.terminal?.scrollToLine(Number(button.dataset.terminalBookmark))));
  document.querySelector("#remote-parent")?.addEventListener("click", () => void browseRemote(parentRemotePath(state.browsingPath)));
  document.querySelector("#upload-remote-file")?.addEventListener("click", uploadRemoteFile);
  document.querySelectorAll<HTMLElement>("[data-remote-path]").forEach((row) => row.addEventListener("dblclick", () => {
    if (row.dataset.remoteDir === "true") void browseRemote(row.dataset.remotePath ?? "~");
  }));
  document.querySelectorAll<HTMLElement>("[data-download-path]").forEach((button) => button.addEventListener("click", (event) => {
    event.stopPropagation();
    void downloadRemoteFile(button.dataset.downloadPath ?? "");
  }));
  const rememberPassword = document.querySelector<HTMLInputElement>('input[name="rememberPassword"]');
  const requireBiometric = document.querySelector<HTMLInputElement>('input[name="requireBiometric"]');
  rememberPassword?.addEventListener("change", () => {
    if (!requireBiometric || !state.biometricAvailable) return;
    requireBiometric.disabled = !rememberPassword.checked;
    if (!rememberPassword.checked) requireBiometric.checked = false;
  });
  document.querySelector("#delete-server")?.addEventListener("click", deleteServer);
}

function themeChoice(value: Preferences["theme"], label: string, description: string, selected: Preferences["theme"]) {
  return `<label class="theme-choice ${value}">
    <input type="radio" name="theme" value="${value}" ${selected === value ? "checked" : ""}>
    <span class="theme-preview"><i></i><b></b></span>
    <span><strong>${label}</strong><small>${description}</small></span>
  </label>`;
}

function cloudAccountMarkup() {
  if (state.cloudLoading) return `<div class="cloud-account"><span class="cloud-status-dot"></span><span><strong>Contacting cloud…</strong><small>Checking provider and account status</small></span></div>`;
  if (state.cloud.error) return `<div class="cloud-account cloud-error"><span><strong>Cloud unavailable</strong><small>${escapeHtml(state.cloud.error)}</small></span></div>`;
  if (state.cloud.signedIn) return `<div class="cloud-account">
    <span class="cloud-avatar">${escapeHtml((state.cloud.user.displayName || state.cloud.user.email || "U").slice(0, 1).toUpperCase())}</span>
    <span><strong>${escapeHtml(state.cloud.user.displayName || state.cloud.user.email)}</strong><small>${escapeHtml(state.cloud.user.email)} · session stored in your OS credential vault</small></span>
    <button type="button" class="ghost cloud-action" id="cloud-logout">Sign out</button>
  </div>`;
  const hasURL = Boolean(currentCloudURL());
  return `<div class="cloud-account cloud-signin"><span><strong>${hasURL ? "Sign in to sync" : "Configure a cloud URL"}</strong><small>${hasURL ? "Providers become available when configured on the server." : "Enter the HTTPS address of your SSHKing cloud server first."}</small></span>
    <div class="cloud-provider-actions">
      <button type="button" class="cloud-provider" data-cloud-login="google" ${!state.cloud.providers.google ? "disabled" : ""}>Google</button>
      <button type="button" class="cloud-provider" data-cloud-login="apple" ${!state.cloud.providers.apple ? "disabled" : ""}>Apple</button>
    </div>
  </div>`;
}

function accountButtonMarkup() {
  if (state.cloudLoading) return `<button class="account-island loading" id="account-button" type="button"><span class="account-spinner"></span><span>Signing in…</span></button>`;
  if (!state.cloud.signedIn) return `<button class="account-island signed-out" id="account-button" type="button"><span class="account-icon">↗</span><span>Log in</span></button>`;
  const displayName = state.cloud.user.displayName || state.cloud.user.email || "Account";
  return `<button class="account-island signed-in" id="account-button" type="button" title="${escapeHtml(state.cloud.user.email)}">
    <span class="account-avatar">${escapeHtml(displayName.slice(0, 1).toUpperCase())}</span>
    <span class="account-label"><small>Logged in as</small><strong>${escapeHtml(displayName)}</strong></span>
  </button>`;
}

function openAccount() {
  state.modal = "account";
  render();
  if (state.config.preferences.cloudUrl) void refreshCloudState();
}

function currentCloudURL() {
  return String(new FormData(document.querySelector<HTMLFormElement>("#settings-form") ?? document.createElement("form")).get("cloudUrl") ?? state.config.preferences.cloudUrl ?? "").trim().replace(/\/$/, "");
}

async function refreshCloudState() {
  const cloudURL = currentCloudURL();
  if (!cloudURL) {
    state.cloud = { cloudUrl: "", signedIn: false, user: { id: "", displayName: "", email: "", avatarUrl: "" }, providers: { google: false, apple: false } };
    return render();
  }
  state.config.preferences.cloudUrl = cloudURL;
  state.cloudLoading = true; state.cloud.error = ""; render();
  try {
    state.cloud = await backend("GetCloudState", cloudURL);
    if (state.cloud.signedIn && !cloudHydrated) await syncCloudWorkspace(true);
  }
  catch (error) { state.cloud.error = error instanceof Error ? error.message : String(error); }
  finally { state.cloudLoading = false; if (!state.modal || state.modal === "settings" || state.modal === "account") render(); }
}

async function loginCloud(provider: string) {
  const cloudURL = currentCloudURL(); if (!cloudURL || !provider) return;
  state.cloudLoading = true; state.cloud.error = ""; render();
  try {
    state.cloud = await backend("LoginCloud", cloudURL, provider);
    state.config.preferences.cloudUrl = cloudURL;
    cloudHydrated = false;
    await syncCloudWorkspace(true);
  }
  catch (error) { state.cloud.error = error instanceof Error ? error.message : String(error); }
  finally { state.cloudLoading = false; if (!state.modal || state.modal === "settings" || state.modal === "account") render(); }
}

async function logoutCloud() {
  const cloudURL = currentCloudURL(); if (!cloudURL) return;
  try { await backend("LogoutCloud", cloudURL); cloudHydrated = false; await refreshCloudState(); }
  catch (error) { state.cloud.error = error instanceof Error ? error.message : String(error); render(); }
}

function cloudTabsPayload(): CloudTab[] {
  return state.tabs.filter((tab) => Boolean(tab.serverId)).map((tab, position) => ({
    id: tab.id, serverId: tab.serverId, title: tab.title, manualTitle: tab.manualTitle,
    restore: tab.restoreSession || tab.connection === "connected" || tab.connection === "connecting",
    lastPath: tab.remotePath, position,
  }));
}

function scheduleCloudSync(options: { tab?: string; server?: string; team?: string } = {}) {
  if (options.tab) pendingCloudTabDeletes.add(options.tab);
  if (options.server) pendingCloudServerDeletes.add(options.server);
  if (options.team) pendingCloudTeamDeletes.add(options.team);
  if (!state.cloud.signedIn) return;
  window.clearTimeout(cloudSyncTimer);
  cloudSyncTimer = window.setTimeout(() => void syncCloudWorkspace(false), 700);
}

async function syncCloudWorkspace(initialPull = false) {
  if (!state.cloud.signedIn || !state.config.preferences.cloudUrl) return;
  if (cloudSyncInFlight) { cloudSyncQueued = true; return; }
  cloudSyncInFlight = true;
  const deletedTabs = Array.from(pendingCloudTabDeletes);
  const deletedServers = Array.from(pendingCloudServerDeletes);
  const deletedTeams = Array.from(pendingCloudTeamDeletes);
  pendingCloudTabDeletes.clear(); pendingCloudServerDeletes.clear(); pendingCloudTeamDeletes.clear();
  const submitted = initialPull ? [] : cloudTabsPayload();
  const submittedIDs = new Set(submitted.map((tab) => tab.id));
  try {
    const result = await backend("SyncCloudWorkspace", state.config.preferences.cloudUrl, submitted, deletedTabs, deletedServers, deletedTeams) as CloudSyncResult;
    state.config = result.config;
    state.readiness = result.readiness ?? {};
    const knownServers = new Set(state.config.servers.map((server) => server.id));
    const cloudIDs = new Set(result.workspace.tabs.map((tab) => tab.id));
    if (initialPull && result.workspace.tabs.length && !state.tabs.some((tab) => tab.connection === "connected" || tab.connection === "connecting")) {
      state.tabs.forEach((tab) => tab.terminal?.dispose());
      state.tabs = [];
    } else {
      for (const tab of [...state.tabs]) {
        if (submittedIDs.has(tab.id) && !cloudIDs.has(tab.id)) {
          tab.terminal?.dispose();
          state.tabs.splice(state.tabs.indexOf(tab), 1);
        }
      }
    }
    for (const remote of result.workspace.tabs) {
      if (!knownServers.has(remote.serverId)) continue;
      let tab = state.tabs.find((item) => item.id === remote.id);
      if (!tab) { tab = newTerminalTab(remote.serverId, remote.id); state.tabs.push(tab); }
      tab.serverId = remote.serverId;
      if (remote.manualTitle || !tab.manualTitle) tab.title = remote.title || state.config.servers.find((server) => server.id === remote.serverId)?.name || "Terminal";
      tab.manualTitle = Boolean(remote.manualTitle);
      tab.remotePath = remote.lastPath || tab.remotePath;
      tab.restoreSession = Boolean(remote.restore) || Boolean(state.config.servers.find((server) => server.id === remote.serverId)?.useTmux);
    }
    if (!state.tabs.length) state.tabs.push(newTerminalTab(state.config.servers[0]?.id ?? ""));
    if (!state.tabs.some((tab) => tab.id === state.activeTabId)) state.activeTabId = state.tabs[0].id;
    const active = activeTab(); if (active) state.selectedId = active.serverId;
    if (!state.tabs.some((tab) => tab.id === state.splitTabId)) state.splitTabId = "";
    cloudHydrated = true;
    persistWorkspace();
    render();
    if (initialPull && !result.workspace.tabs.length) scheduleCloudSync();
  } catch (error) {
    deletedTabs.forEach((id) => pendingCloudTabDeletes.add(id));
    deletedServers.forEach((id) => pendingCloudServerDeletes.add(id));
    deletedTeams.forEach((id) => pendingCloudTeamDeletes.add(id));
    state.cloud.error = `Sync failed: ${error instanceof Error ? error.message : String(error)}`;
  } finally {
    cloudSyncInFlight = false;
    if (cloudSyncQueued) { cloudSyncQueued = false; scheduleCloudSync(); }
  }
}

function rangeControl(label: string, name: string, value: number, min: number, max: number, step: number, suffix: string) {
  return `<label class="range-control">
    <span><strong>${label}</strong><output data-range-output="${name}">${value}${suffix}</output></span>
    <input type="range" name="${name}" value="${value}" min="${min}" max="${max}" step="${step}" data-suffix="${escapeHtml(suffix)}">
  </label>`;
}

function paletteActions() {
  const modifier = state.platform === "darwin" ? "⌘" : "Ctrl";
  return [
    { id: "new-tab", label: "New terminal", detail: "Open another terminal tab", shortcut: `${modifier} T`, icon: icons.plus },
    { id: "split", label: "Toggle split terminal", detail: "View two sessions side by side", shortcut: `${modifier} \\`, icon: "▥" },
    { id: "connect", label: activeTab()?.connection === "connected" ? "Disconnect active server" : "Connect active server", detail: activeTab()?.title ?? "Terminal", shortcut: "", icon: icons.terminal },
    { id: "files", label: "Browse remote files", detail: "Open secure SFTP browser", shortcut: "", icon: icons.server },
    { id: "tunnels", label: "Manage local tunnels", detail: "Forward a local port over SSH", shortcut: "", icon: icons.arrow },
    { id: "zed", label: "Open current folder in Zed", detail: currentRemotePath(), shortcut: "", icon: icons.code },
    { id: "import", label: "Import SSH config", detail: "Add hosts from ~/.ssh/config", shortcut: "", icon: "⇩" },
    { id: "settings", label: "Open preferences", detail: "Terminal, shell, and privacy defaults", shortcut: state.platform === "darwin" ? "⌘ ," : "", icon: icons.settings },
    { id: "cmd:pwd", label: "Run: pwd", detail: "Print the current remote directory", shortcut: "", icon: "›" },
    { id: "cmd:ls -la", label: "Run: ls -la", detail: "List all files with details", shortcut: "", icon: "›" },
    { id: "cmd:git status", label: "Run: git status", detail: "Show repository status", shortcut: "", icon: "›" },
    { id: "cmd:df -h", label: "Run: df -h", detail: "Show remote disk usage", shortcut: "", icon: "›" },
  ];
}

function filterPalette(query: string) {
  state.paletteQuery = query;
  const normalized = query.trim().toLowerCase();
  document.querySelectorAll<HTMLElement>("[data-palette-search]").forEach((row) => {
    row.hidden = Boolean(normalized) && !row.dataset.paletteSearch?.includes(normalized);
  });
}

function handlePaletteKeydown(event: KeyboardEvent) {
  if (event.key !== "Enter") return;
  event.preventDefault();
  const query = (event.currentTarget as HTMLInputElement).value.trim();
  const first = document.querySelector<HTMLElement>("[data-palette-action]:not([hidden])");
  if (!query && first) {
    void runPaletteAction(first.dataset.paletteAction ?? "");
  } else if (query) {
    const exact = paletteActions().find((action) => action.label.toLowerCase() === query.toLowerCase());
    void (exact ? runPaletteAction(exact.id) : runTerminalCommand(query));
  }
}

async function runPaletteAction(action: string) {
  state.modal = "";
  state.paletteQuery = "";
  if (action === "new-tab") return addTerminalTab();
  if (action === "split") return toggleSplitPane();
  if (action === "connect") return toggleConnection();
  if (action === "files") return openRemoteFiles();
  if (action === "tunnels") { state.modal = "tunnels"; return render(); }
  if (action === "zed") return openCurrentFolderInZed();
  if (action === "import") return importSSHConfig();
  if (action === "settings") return openModal("settings");
  if (action.startsWith("cmd:")) return runTerminalCommand(action.slice(4));
  render();
}

async function runTerminalCommand(command: string) {
  const tab = activeTab();
  state.modal = "";
  state.paletteQuery = "";
  if (!tab || tab.connection !== "connected") {
    pushOutput(tab, { kind: "error", text: "Connect this terminal before running a command." });
    render();
    return;
  }
  try {
    await backend("SendCommand", tab.id, command);
    recordObservedCommand(tab, command);
  } catch (error) {
    pushOutput(tab, { kind: "error", text: `Command failed: ${String(error)}` });
  }
  render();
  requestAnimationFrame(() => tab.terminal?.focus());
}

function openModal(type: "server" | "settings") {
  if (type === "server" && !state.editingId) state.editingId = "";
  state.modal = type;
  render();
  setTimeout(() => document.querySelector<HTMLInputElement>(".modal input:not([type=hidden])")?.focus(), 20);
  if (type === "settings" && state.config.preferences.cloudUrl) void refreshCloudState();
}

function openTeamModal(teamId = "") {
  state.editingTeamId = teamId;
  state.modal = "team";
  render();
  setTimeout(() => document.querySelector<HTMLInputElement>('#team-form input[name="name"]')?.focus(), 20);
}

function closeModal() {
  if (state.modal === "settings") applyAppearance(state.config.preferences);
  if (state.modal === "trust-host-key") {
    state.pendingConnection = null;
    state.pendingHostFingerprint = "";
  }
  state.modal = "";
  state.editingId = "";
  state.editingTeamId = "";
  state.pendingServerTeamId = "";
  render();
}

async function saveTeam(event: SubmitEvent) {
  event.preventDefault();
  const form = new FormData(event.currentTarget as HTMLFormElement);
  const team: Team = {
    id: String(form.get("id") ?? ""),
    name: String(form.get("name") ?? "").trim(),
  };
  state.config = await backend("SaveTeam", team);
  const saved = state.config.teams.find((item) => item.id === team.id) ?? state.config.teams.at(-1);
  if (saved) state.collapsedTeamIds.delete(saved.id);
  state.teamsExpanded = true;
  persistSidebarState();
  closeModal();
  scheduleCloudSync();
}

async function deleteTeam() {
  if (!state.editingTeamId) return;
  const deletedId = state.editingTeamId;
  state.config = await backend("DeleteTeam", deletedId);
  state.collapsedTeamIds.delete(deletedId);
  state.personalExpanded = true;
  persistSidebarState();
  closeModal();
  scheduleCloudSync({ team: deletedId });
}

async function saveServer(event: SubmitEvent) {
  event.preventDefault();
  const form = new FormData(event.currentTarget as HTMLFormElement);
  const existing = state.config.servers.find((server) => server.id === String(form.get("id")));
  const server: Server = {
    id: String(form.get("id") ?? ""),
    teamId: String(form.get("teamId") ?? ""),
    name: String(form.get("name") ?? ""),
    group: String(form.get("group") ?? ""),
    host: String(form.get("host") ?? ""),
    user: String(form.get("user") ?? ""),
    port: Number(form.get("port") ?? 22),
    shell: String(form.get("shell") ?? "default"),
    useTmux: form.get("useTmux") === "on",
    tmuxSession: String(form.get("tmuxSession") ?? "sshking"),
    identity: String(form.get("identity") ?? ""),
    jumpServerId: String(form.get("jumpServerId") ?? ""),
    fingerprint: String(form.get("fingerprint") ?? ""),
    favorite: form.get("favorite") === "on",
    passwordSaved: existing?.passwordSaved ?? false,
    requireBiometric: existing?.requireBiometric ?? false,
  };
  state.config = await backend("SaveServer", server);
  if (server.teamId) {
    state.teamsExpanded = true;
    state.collapsedTeamIds.delete(server.teamId);
  } else {
    state.personalExpanded = true;
  }
  persistSidebarState();
  state.selectedId = server.id || state.config.servers.at(-1)?.id || "";
  const tab = activeTab();
  if (tab && tab.connection === "idle") {
    if (tab.serverId !== state.selectedId) {
      void backend("ClearSessionTranscript", tab.id);
      tab.restoredTranscript = "";
    }
    tab.serverId = state.selectedId;
    if (!tab.manualTitle) tab.title = state.config.servers.find((item) => item.id === tab.serverId)?.name ?? "Terminal";
  }
  closeModal();
  state.readiness = await backend("GetServerReadiness");
  scheduleCloudSync();
}

async function saveSettings(event: SubmitEvent) {
  event.preventDefault();
  const form = new FormData(event.currentTarget as HTMLFormElement);
  const preferences: Preferences = {
    cloudUrl: String(form.get("cloudUrl") ?? "").trim().replace(/\/$/, ""),
    defaultUser: String(form.get("defaultUser") ?? ""),
    defaultPort: Number(form.get("defaultPort") ?? 22),
    defaultShell: String(form.get("defaultShell") ?? "default"),
    defaultIdentity: String(form.get("defaultIdentity") ?? ""),
    scrollback: Number(form.get("scrollback") ?? 2000),
    logActivity: form.get("logActivity") === "on",
    persistTerminalHistory: form.get("persistTerminalHistory") === "on",
    theme: String(form.get("theme") ?? "glass") as Preferences["theme"],
    uiScale: Number(form.get("uiScale") ?? 100),
    terminalFontSize: Number(form.get("terminalFontSize") ?? 14),
    terminalFontFamily: String(form.get("terminalFontFamily") ?? "system-mono") as Preferences["terminalFontFamily"],
    terminalLineHeight: Number(form.get("terminalLineHeight") ?? 140),
    autoConnectTabs: form.get("autoConnectTabs") === "on",
    reopenActiveSession: form.get("reopenActiveSession") === "on",
  };
  state.config = await backend("SavePreferences", preferences);
  applyAppearance(state.config.preferences);
  closeModal();
}

function previewSettings(form: HTMLFormElement) {
  const data = new FormData(form);
  const preview: Preferences = {
    ...state.config.preferences,
    theme: String(data.get("theme") ?? state.config.preferences.theme) as Preferences["theme"],
    uiScale: Number(data.get("uiScale") ?? state.config.preferences.uiScale),
    terminalFontSize: Number(data.get("terminalFontSize") ?? state.config.preferences.terminalFontSize),
    terminalFontFamily: String(data.get("terminalFontFamily") ?? state.config.preferences.terminalFontFamily) as Preferences["terminalFontFamily"],
    terminalLineHeight: Number(data.get("terminalLineHeight") ?? state.config.preferences.terminalLineHeight),
  };
  form.querySelectorAll<HTMLInputElement>('input[type="range"]').forEach((input) => {
    const output = form.querySelector<HTMLOutputElement>(`[data-range-output="${input.name}"]`);
    if (output) output.value = `${input.value}${input.dataset.suffix ?? ""}`;
  });
  applyAppearance(preview);
}

function applyAppearance(preferences: Preferences) {
  const shell = document.querySelector<HTMLElement>(".window-shell");
  if (shell) {
    shell.classList.remove("theme-light", "theme-black", "theme-glass");
    shell.classList.add(`theme-${preferences.theme}`);
    shell.style.setProperty("--ui-scale", String(preferences.uiScale / 100));
  }
  for (const tab of state.tabs) {
    if (!tab.terminal) continue;
    tab.terminal.options.fontSize = preferences.terminalFontSize;
    tab.terminal.options.fontFamily = terminalFontFamily(preferences.terminalFontFamily);
    tab.terminal.options.lineHeight = preferences.terminalLineHeight / 100;
    tab.terminal.options.theme = terminalTheme(preferences.theme);
  }
  requestAnimationFrame(fitVisibleTerminals);
}

async function openActivity() {
  try {
    state.activity = await backend("GetActivity", 300);
  } catch (error) {
    state.activity = [`Could not load activity: ${String(error)}`];
  }
  state.modal = "activity";
  render();
}

async function clearTerminalHistory() {
  try {
    await backend("ClearTerminalHistory");
    for (const tab of state.tabs) tab.restoredTranscript = "";
    state.modal = "";
    pushOutput(activeTab(), { kind: "success", text: "Saved terminal history cleared." });
    render();
  } catch (error) {
    pushOutput(activeTab(), { kind: "error", text: `Could not clear terminal history: ${String(error)}` });
  }
}

async function importSSHConfig() {
  const before = state.config.servers.length;
  try {
    state.config = await backend("ImportSSHConfig", "");
    const added = state.config.servers.length - before;
    if (!state.selectedId && state.config.servers[0]) {
      state.selectedId = state.config.servers[0].id;
      const tab = activeTab();
      if (tab) {
        tab.serverId = state.selectedId;
        tab.title = state.config.servers[0].name;
      }
    }
    pushOutput(activeTab(), {
      kind: added ? "success" : "muted",
      text: added ? `Imported ${added} server${added === 1 ? "" : "s"} from ~/.ssh/config` : "No new SSH hosts found to import.",
    });
  } catch (error) {
    pushOutput(activeTab(), { kind: "error", text: `Could not import ~/.ssh/config: ${String(error)}` });
  }
  render();
}

async function deleteServer() {
  if (!state.editingId) return;
  const deletedId = state.editingId;
  state.config = await backend("DeleteServer", deletedId);
  const deletedTabs = state.tabs.filter((item) => item.serverId === deletedId);
  for (const tab of deletedTabs) {
    await backend("ClearSessionTranscript", tab.id).catch(() => undefined);
    window.clearTimeout(tab.reconnectTimer);
    tab.terminal?.dispose();
  }
  state.tabs = state.tabs.filter((item) => item.serverId !== deletedId);
  state.selectedId = state.config.servers[0]?.id ?? "";
  if (!state.tabs.length) state.tabs.push(newTerminalTab(state.selectedId));
  const next = state.tabs.find((tab) => tab.id === state.activeTabId) ?? state.tabs[0];
  state.activeTabId = next.id;
  state.selectedId = next.serverId;
  if (!state.tabs.some((tab) => tab.id === state.splitTabId)) state.splitTabId = "";
  closeModal();
  deletedTabs.forEach((tab) => pendingCloudTabDeletes.add(tab.id));
  scheduleCloudSync({ server: deletedId });
}

async function toggleConnection() {
  const tab = activeTab();
  if (!tab) return;
  if (tab.connection === "connected") {
    tab.restoreSession = false;
    tab.lastRequest = undefined;
    window.clearTimeout(tab.reconnectTimer);
    tab.reconnectTimer = undefined;
    await backend("Disconnect", tab.id);
    tab.connection = "idle";
    state.tunnels = state.tunnels.filter((tunnel) => tunnel.sessionId !== tab.id);
    pushOutput(tab, { kind: "muted", text: "Session disconnected." });
    render();
    scheduleCloudSync();
    return;
  }
  state.modal = "connect";
  render();
}

async function connectWithCredentials(event: SubmitEvent) {
  event.preventDefault();
  const form = new FormData(event.currentTarget as HTMLFormElement);
  await attemptConnection({
    tabId: activeTab()?.id ?? "",
    password: String(form.get("password") ?? ""),
    rememberPassword: form.get("rememberPassword") === "on",
    requireBiometric: form.get("requireBiometric") === "on",
  }, false);
}

async function trustHostKey() {
  const request = state.pendingConnection;
  if (!request) {
    closeModal();
    return;
  }
  state.pendingConnection = null;
  state.pendingHostFingerprint = "";
  await attemptConnection(request, true);
}

async function attemptConnection(request: ConnectionRequest, trustNewHost: boolean) {
  const tab = state.tabs.find((item) => item.id === request.tabId);
  if (!tab) return;
  state.modal = "";
  tab.connection = "connecting";
  pushOutput(tab, { kind: "system", text: `Opening secure session…` });
  render();
  try {
    await backend("Connect", tab.id, tab.serverId, request.password, request.rememberPassword, request.requireBiometric, trustNewHost);
    const refreshed = await backend("GetState");
    if (refreshed?.config) state.config = refreshed.config;
    state.readiness = await backend("GetServerReadiness");
    state.pendingConnection = null;
    state.pendingHostFingerprint = "";
    tab.connection = "connected";
    tab.restoreSession = true;
    const server = state.config.servers.find((item) => item.id === tab.serverId);
    tab.lastRequest = (!request.password || request.rememberPassword || Boolean(server?.identity))
      ? { ...request, password: "" }
      : undefined;
    tab.reconnectAttempts = 0;
    pushOutput(tab, {
      kind: "success",
      text: server?.useTmux
        ? "Connected · persistent tmux session attached for this tab"
        : "Connected · remote shell is ready",
    });
    fitTerminal(tab);
    await backend("ResizeTerminal", tab.id, tab.terminal?.cols ?? 120, tab.terminal?.rows ?? 34);
    if (!window.go) {
      pushOutput(tab, { kind: "output", text: "Last login: Today from 192.168.1.8" });
      pushOutput(tab, { kind: "output", text: "Welcome to Northstar Linux 24.04 LTS" });
    }
  } catch (error) {
    const message = String(error);
    const fingerprint = message.includes("host key verification required")
      ? message.match(/SHA256:[A-Za-z0-9+/=]+/)?.[0]
      : undefined;
    if (fingerprint && !trustNewHost) {
      tab.connection = "idle";
      state.pendingConnection = request;
      state.pendingHostFingerprint = fingerprint;
      state.modal = "trust-host-key";
      render();
      return;
    }
    tab.connection = "error";
    pushOutput(tab, { kind: "error", text: message });
    if (tab.lastRequest && tab.reconnectAttempts > 0) {
      tab.connection = "idle";
      scheduleReconnect(tab);
    }
  }
  render();
  if (tab.connection === "connected") {
    scheduleCloudSync();
    requestAnimationFrame(() => {
      fitTerminal(tab);
      if (tab.id === state.activeTabId) tab.terminal?.focus();
    });
  }
}

async function openInZed(event: SubmitEvent) {
  event.preventDefault();
  const form = new FormData(event.currentTarget as HTMLFormElement);
  const remotePath = String(form.get("remotePath") ?? "~").trim() || "~";
  const newWindow = form.get("newWindow") === "on";
  const passSavedPassword = form.get("passSavedPassword") === "on";
  try {
    await backend("OpenInZed", state.selectedId, remotePath, newWindow, passSavedPassword);
    state.modal = "";
    pushOutput(activeTab(), { kind: "success", text: `Opening ${remotePath} in Zed…` });
  } catch (error) {
    state.modal = "";
    pushOutput(activeTab(), { kind: "error", text: `Could not open Zed: ${String(error)}` });
  }
  render();
}

async function openCurrentFolderInZed() {
  const remotePath = currentRemotePath();
  try {
    await backend("OpenInZed", state.selectedId, remotePath, true, false);
    pushOutput(activeTab(), { kind: "success", text: `Opening ${remotePath} in Zed…` });
  } catch (error) {
    pushOutput(activeTab(), { kind: "error", text: `Could not open Zed: ${String(error)}` });
  }
  render();
}

async function openRemoteFiles() {
  const tab = activeTab();
  if (!tab || tab.connection !== "connected") {
    pushOutput(tab, { kind: "error", text: "Connect this terminal before browsing remote files." });
    state.modal = "";
    render();
    return;
  }
  state.browsingPath = tab.remotePath || "~";
  state.remoteFiles = [];
  state.modal = "files";
  render();
  await browseRemote(state.browsingPath);
}

async function browseRemote(remotePath: string) {
  const tab = activeTab();
  if (!tab) return;
  state.filesLoading = true;
  state.browsingPath = remotePath;
  render();
  try {
    state.remoteFiles = await backend("ListRemoteFiles", tab.id, remotePath);
  } catch (error) {
    state.remoteFiles = [];
    pushOutput(tab, { kind: "error", text: `Could not browse ${remotePath}: ${String(error)}` });
  } finally {
    state.filesLoading = false;
    render();
  }
}

async function downloadRemoteFile(remotePath: string) {
  const tab = activeTab();
  if (!tab || !remotePath) return;
  try {
    const destination = await backend("DownloadRemoteFile", tab.id, remotePath);
    if (destination) pushOutput(tab, { kind: "success", text: `Downloaded ${remotePath} to ${destination}` });
  } catch (error) {
    pushOutput(tab, { kind: "error", text: `Download failed: ${String(error)}` });
  }
  render();
}

async function uploadRemoteFile() {
  const tab = activeTab();
  if (!tab) return;
  try {
    const uploaded = await backend("UploadRemoteFile", tab.id, state.browsingPath);
    if (uploaded) {
      pushOutput(tab, { kind: "success", text: `Uploaded ${uploaded}` });
      await browseRemote(state.browsingPath);
    }
  } catch (error) {
    pushOutput(tab, { kind: "error", text: `Upload failed: ${String(error)}` });
    render();
  }
}

async function startLocalTunnel(event: SubmitEvent) {
  event.preventDefault();
  const tab = activeTab();
  if (!tab) return;
  const form = new FormData(event.currentTarget as HTMLFormElement);
  try {
    const tunnel = await backend(
      "StartLocalTunnel",
      tab.id,
      Number(form.get("localPort") ?? 0),
      String(form.get("remoteHost") ?? "127.0.0.1"),
      Number(form.get("remotePort") ?? 0),
    ) as TunnelInfo;
    state.tunnels.push(tunnel);
    pushOutput(tab, { kind: "success", text: `Tunnel ready · ${tunnel.local} → ${tunnel.remoteHost}:${tunnel.remotePort}` });
  } catch (error) {
    pushOutput(tab, { kind: "error", text: `Could not start tunnel: ${String(error)}` });
  }
  render();
}

async function stopTunnel(id: string) {
  const tunnel = state.tunnels.find((item) => item.id === id);
  try {
    await backend("StopTunnel", id);
    state.tunnels = state.tunnels.filter((item) => item.id !== id);
    if (tunnel) pushOutput(activeTab(), { kind: "muted", text: `Stopped tunnel on ${tunnel.local}` });
  } catch (error) {
    pushOutput(activeTab(), { kind: "error", text: `Could not stop tunnel: ${String(error)}` });
  }
  render();
}

async function openSSHKeyModal() {
  try {
    state.sshKeys = await backend("ListSSHKeys", state.selectedId);
    state.modal = "ssh-key";
    render();
  } catch (error) {
    pushOutput(activeTab(), { kind: "error", text: `Could not list SSH keys: ${String(error)}` });
    render();
  }
}

async function installSSHKey(event: SubmitEvent) {
  event.preventDefault();
  const form = new FormData(event.currentTarget as HTMLFormElement);
  const choice = String(form.get("publicKeyPath") ?? "");
  const password = String(form.get("password") ?? "");
  const generate = choice === "__generate__";
  try {
    state.config = await backend("InstallSSHKey", state.selectedId, generate ? "" : choice, password, generate);
    state.readiness = await backend("GetServerReadiness");
    state.modal = "";
    pushOutput(activeTab(), { kind: "success", text: "SSH key installed · future connections can use the private key" });
    scheduleCloudSync();
  } catch (error) {
    state.modal = "";
    pushOutput(activeTab(), { kind: "error", text: `Could not install SSH key: ${String(error)}` });
  }
  render();
}

function attachRuntimeEvents() {
  window.runtime?.EventsOn("terminal:data", (payload) => {
    const event = payload as { sessionId?: string; data?: string };
    const tab = state.tabs.find((item) => item.id === event.sessionId);
    if (tab) queueTerminalOutput(tab, String(event.data ?? ""));
  });
  window.runtime?.EventsOn("terminal:status", (payload) => {
    const event = payload as { sessionId?: string; state?: ConnectionState | "disconnected"; message?: string };
    const tab = state.tabs.find((item) => item.id === event.sessionId);
    if (!tab) return;
    tab.connection = event.state === "disconnected" ? "idle" : event.state ?? "idle";
    if (event.state === "disconnected") state.tunnels = state.tunnels.filter((tunnel) => tunnel.sessionId !== tab.id);
    if (event.message) pushOutput(tab, { kind: "error", text: event.message });
    if (event.state === "disconnected" && tab.lastRequest) scheduleReconnect(tab);
    render();
    if (event.state === "disconnected") scheduleCloudSync();
  });
}

function queueTerminalOutput(tab: TerminalTab, data: string) {
  if (!data) return;
  tab.pendingTerminalData += data;
  if (tab.terminalWriteScheduled) return;
  tab.terminalWriteScheduled = true;
  requestAnimationFrame(() => flushTerminalOutput(tab));
}

function flushTerminalOutput(tab: TerminalTab) {
  const chunk = tab.pendingTerminalData.slice(0, 65536);
  tab.pendingTerminalData = tab.pendingTerminalData.slice(chunk.length);
  if (chunk) tab.terminal?.write(chunk);
  if (tab.pendingTerminalData) {
    requestAnimationFrame(() => flushTerminalOutput(tab));
  } else {
    tab.terminalWriteScheduled = false;
  }
}

function scheduleReconnect(tab: TerminalTab) {
  if (!tab.restoreSession || tab.reconnectTimer) return;
  const delay = Math.min(30000, 1000 * 2 ** Math.min(tab.reconnectAttempts, 5));
  tab.reconnectAttempts++;
  pushOutput(tab, { kind: "muted", text: `Connection lost · reconnecting in ${delay / 1000}s (attempt ${tab.reconnectAttempts})` });
  tab.reconnectTimer = window.setTimeout(() => {
    tab.reconnectTimer = undefined;
    if (!tab.lastRequest || tab.connection !== "idle") return;
    void attemptConnection(tab.lastRequest, false);
  }, delay);
}

function pushOutput(tab: TerminalTab | undefined, line: OutputLine) {
  if (!tab) return;
  tab.output.push(line);
  const excess = tab.output.length - state.config.preferences.scrollback;
  if (excess > 0) tab.output.splice(0, excess);
  writeOutputLine(tab, line);
}

function writeOutputLine(tab: TerminalTab, line: OutputLine) {
  if (!tab.terminal) return;
  const color = { system: "\x1b[94m", muted: "\x1b[90m", success: "\x1b[92m", error: "\x1b[91m" }[line.kind] ?? "\x1b[0m";
  tab.terminal.writeln(`\r${color}${line.text}\x1b[0m`);
}

function mountVisibleTerminals() {
  for (const tab of visibleTabs()) mountTerminal(tab);
  requestAnimationFrame(() => fitVisibleTerminals());
}

function mountTerminal(tab: TerminalTab) {
  const placeholder = document.querySelector<HTMLDivElement>(`[data-terminal-slot="${tab.id}"]`);
  if (!placeholder) return;

  if (tab.host && tab.terminal) {
    placeholder.replaceWith(tab.host);
  } else {
    tab.host = placeholder;
    tab.terminal = new Terminal({
      cursorBlink: true,
      cursorStyle: "bar",
      fontFamily: terminalFontFamily(state.config.preferences.terminalFontFamily),
      fontSize: state.config.preferences.terminalFontSize,
      fontWeight: "500",
      fontWeightBold: "700",
      lineHeight: state.config.preferences.terminalLineHeight / 100,
      scrollback: state.config.preferences.scrollback,
      allowTransparency: true,
      theme: terminalTheme(state.config.preferences.theme),
    });
    tab.terminal.parser.registerOscHandler(7, (data) => {
      recordRemoteDirectory(tab, data);
      return true;
    });
    tab.terminal.attachCustomKeyEventHandler((event) => {
      const copyShortcut = (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "c";
      const findShortcut = (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "f";
      if (event.type === "keydown" && copyShortcut && tab.terminal?.hasSelection()) {
        void copyTerminalSelection(tab);
        return false;
      }
      if (event.type === "keydown" && findShortcut) {
        openTerminalSearch();
        return false;
      }
      return true;
    });
    tab.fitAddon = new FitAddon();
    tab.searchAddon = new SearchAddon();
    tab.terminal.loadAddon(tab.fitAddon);
    tab.terminal.loadAddon(tab.searchAddon);
    tab.terminal.open(tab.host);
    tab.terminal.onData((data) => queueTerminalInput(tab, data));
    tab.terminal.onResize(({ cols, rows }) => scheduleTerminalResize(tab, cols, rows));
    if (tab.restoredTranscript) {
      tab.terminal.write(tab.restoredTranscript);
      tab.terminal.writeln("\r\n\x1b[90m── Previous session history ──\x1b[0m");
    }
    for (const line of tab.output) writeOutputLine(tab, line);
  }

  const pane = document.querySelector<HTMLElement>(`[data-pane-tab="${tab.id}"]`);
  pane?.addEventListener("mousedown", () => tab.terminal?.focus());
  pane?.addEventListener("contextmenu", (event) => {
    if (!tab.terminal?.hasSelection()) return;
    event.preventDefault();
    void copyTerminalSelection(tab);
  });
}

async function copyTerminalSelection(tab: TerminalTab) {
  const selection = tab.terminal?.getSelection() ?? "";
  if (!selection) return;
  try {
    if (window.runtime?.ClipboardSetText) {
      await window.runtime.ClipboardSetText(selection);
    } else if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(selection);
    } else {
      const clipboardInput = document.createElement("textarea");
      clipboardInput.value = selection;
      clipboardInput.style.position = "fixed";
      clipboardInput.style.opacity = "0";
      document.body.appendChild(clipboardInput);
      clipboardInput.select();
      document.execCommand("copy");
      clipboardInput.remove();
    }
    showCopyToast();
  } catch (error) {
    pushOutput(tab, { kind: "error", text: `Could not copy terminal selection: ${String(error)}` });
  }
}

function showCopyToast() {
  const toast = document.querySelector<HTMLElement>("#terminal-copy-toast");
  if (!toast) return;
  toast.classList.add("visible");
  window.clearTimeout(copyToastTimer);
  copyToastTimer = window.setTimeout(() => toast.classList.remove("visible"), 1100);
}

function openTerminalSearch() {
  state.terminalSearch = true;
  render();
  requestAnimationFrame(() => {
    const input = document.querySelector<HTMLInputElement>("#terminal-search-input");
    input?.focus();
    input?.select();
  });
}

function closeTerminalSearch() {
  state.terminalSearch = false;
  activeTab()?.searchAddon?.clearDecorations();
  render();
  requestAnimationFrame(() => activeTab()?.terminal?.focus());
}

function addTerminalBookmark() {
  const tab = activeTab();
  if (!tab?.terminal) return;
  const line = tab.terminal.buffer.active.viewportY;
  if (!tab.bookmarks.some((bookmark) => bookmark.line === line)) {
    tab.bookmarks.push({ id: `${tab.id}-${line}`, label: `#${tab.bookmarks.length + 1}`, line });
  }
  render();
}

function queueTerminalInput(tab: TerminalTab, data: string) {
  if (tab.connection !== "connected") return;
  observeCommandInput(tab, data);
  tab.inputQueue += data;
  void flushTerminalInput(tab);
}

function observeCommandInput(tab: TerminalTab, data: string) {
  const tokens = data.match(/\x1b\][\s\S]*?(?:\x07|\x1b\\)|\x1bP[\s\S]*?\x1b\\|\x1b\[[0-?]*[ -/]*[@-~]|\x1bO.|[\s\S]/g) ?? [];
  for (const token of tokens) {
    if (token.startsWith("\x1b]") || token.startsWith("\x1bP")) continue;
    if (token === "\r" || token === "\n") {
      submitObservedCommand(tab);
      continue;
    }
    if (token === "\x7f" || token === "\b") {
      if (tab.commandCursor > 0) {
        tab.commandBuffer = tab.commandBuffer.slice(0, tab.commandCursor - 1) + tab.commandBuffer.slice(tab.commandCursor);
        tab.commandCursor--;
      }
      continue;
    }
    if (token === "\x15") {
      tab.commandBuffer = tab.commandBuffer.slice(tab.commandCursor);
      tab.commandCursor = 0;
      continue;
    }
    if (token === "\x17") {
      const before = tab.commandBuffer.slice(0, tab.commandCursor).replace(/\s*\S+\s*$/, "");
      tab.commandBuffer = before + tab.commandBuffer.slice(tab.commandCursor);
      tab.commandCursor = before.length;
      continue;
    }
    if (token === "\x0b") {
      tab.commandBuffer = tab.commandBuffer.slice(0, tab.commandCursor);
      continue;
    }
    if (token === "\x03") {
      resetObservedCommand(tab);
      continue;
    }
    if (token === "\x01") { tab.commandCursor = 0; continue; }
    if (token === "\x05") { tab.commandCursor = tab.commandBuffer.length; continue; }
    if ((token === "\x1b[D" || token === "\x1bOD") && tab.commandCursor > 0) { tab.commandCursor--; continue; }
    if ((token === "\x1b[C" || token === "\x1bOC") && tab.commandCursor < tab.commandBuffer.length) { tab.commandCursor++; continue; }
    if (token === "\x1b[H" || token === "\x1b[1~" || token === "\x1bOH") { tab.commandCursor = 0; continue; }
    if (token === "\x1b[F" || token === "\x1b[4~" || token === "\x1bOF") { tab.commandCursor = tab.commandBuffer.length; continue; }
    if (token === "\x1b[3~") {
      tab.commandBuffer = tab.commandBuffer.slice(0, tab.commandCursor) + tab.commandBuffer.slice(tab.commandCursor + 1);
      continue;
    }
    if (token === "\x1b[A" || token === "\x1b[B" || token === "\x1bOA" || token === "\x1bOB") {
      navigateObservedHistory(tab, token === "\x1b[A" || token === "\x1bOA" ? -1 : 1);
      continue;
    }
    if (token.startsWith("\x1b[") || token.startsWith("\x1bO") || token === "\t" || token < " ") continue;
    tab.commandBuffer = tab.commandBuffer.slice(0, tab.commandCursor) + token + tab.commandBuffer.slice(tab.commandCursor);
    tab.commandCursor += token.length;
  }
}

function submitObservedCommand(tab: TerminalTab) {
  const command = tab.commandBuffer.replace(/[\x00-\x1f\x7f]/g, "").replace(/\s+/g, " ").trim();
  if (command) recordObservedCommand(tab, command);
  resetObservedCommand(tab);
}

function recordObservedCommand(tab: TerminalTab, command: string) {
  const normalized = command.replace(/[\x00-\x1f\x7f]/g, "").replace(/\s+/g, " ").trim();
  if (!normalized) return;
  if (tab.commandHistory.at(-1) !== normalized) tab.commandHistory.push(normalized);
  if (tab.commandHistory.length > 100) tab.commandHistory.shift();
  tab.commandHistoryIndex = tab.commandHistory.length;
  if (!tab.manualTitle) setTabTitle(tab, normalized);
}

function resetObservedCommand(tab: TerminalTab) {
  tab.commandBuffer = "";
  tab.commandCursor = 0;
  tab.commandHistoryIndex = tab.commandHistory.length;
}

function navigateObservedHistory(tab: TerminalTab, direction: number) {
  if (!tab.commandHistory.length) return;
  tab.commandHistoryIndex = Math.max(0, Math.min(tab.commandHistory.length, tab.commandHistoryIndex + direction));
  tab.commandBuffer = tab.commandHistory[tab.commandHistoryIndex] ?? "";
  tab.commandCursor = tab.commandBuffer.length;
}

function setTabTitle(tab: TerminalTab, value: string) {
  tab.title = value.slice(0, 120);
  persistWorkspace();
  const title = document.querySelector<HTMLElement>(`[data-tab-title="${tab.id}"]`);
  if (title && !title.isContentEditable) title.textContent = tab.title;
  scheduleCloudSync();
}

function isTerminalProtocolTitle(value: string) {
  return /^\[(?:[?><=]|\d)[0-9;:?><=]*[A-Za-z~]/.test(value) || /^\](?:10|11|12);/.test(value);
}

function openTabRename(tabId: string) {
  const tab = state.tabs.find((item) => item.id === tabId);
  if (!tab) return;
  if (state.activeTabId !== tabId) selectTab(tabId, false);
  requestAnimationFrame(() => {
    const title = document.querySelector<HTMLElement>(`[data-tab-title="${tabId}"]`);
    if (!title) return;
    const original = tab.title;
    let cancelled = false;
    title.contentEditable = "true";
    title.classList.add("editing");
    title.focus();
    const selection = window.getSelection();
    selection?.selectAllChildren(title);
    title.onkeydown = (event) => {
      event.stopPropagation();
      if (event.key === "Enter") {
        event.preventDefault();
        title.blur();
      } else if (event.key === "Escape") {
        event.preventDefault();
        cancelled = true;
        title.textContent = original;
        title.blur();
      }
    };
    title.onblur = () => {
      title.contentEditable = "false";
      title.classList.remove("editing");
      const value = title.textContent?.replace(/\s+/g, " ").trim() ?? "";
      if (!cancelled && value) {
        tab.manualTitle = true;
        setTabTitle(tab, value);
      } else {
        title.textContent = original;
      }
      requestAnimationFrame(() => tab.terminal?.focus());
    };
  });
}

async function flushTerminalInput(tab: TerminalTab) {
  if (tab.sendingInput) return;
  tab.sendingInput = true;
  try {
    while (tab.inputQueue) {
      const data = tab.inputQueue;
      tab.inputQueue = "";
      await backend("SendInput", tab.id, data);
    }
  } catch (error) {
    pushOutput(tab, { kind: "error", text: `Input failed: ${String(error)}` });
  } finally {
    tab.sendingInput = false;
    if (tab.inputQueue) void flushTerminalInput(tab);
  }
}

function fitTerminal(tab: TerminalTab) {
  if (!tab.host?.isConnected) return;
  try {
    tab.fitAddon?.fit();
  } catch {
    // The fit addon can run before WebView layout has completed.
  }
}

function fitVisibleTerminals() {
  for (const tab of visibleTabs()) fitTerminal(tab);
}

function scheduleTerminalResize(tab: TerminalTab, cols: number, rows: number) {
  window.clearTimeout(tab.resizeTimer);
  tab.resizeTimer = window.setTimeout(() => {
    if (tab.connection === "connected") void backend("ResizeTerminal", tab.id, cols, rows);
  }, 80);
}

function resetTerminal(tab: TerminalTab) {
  tab.inputQueue = "";
  resetObservedCommand(tab);
  tab.output = [
    { kind: "system", text: "SSHKing secure terminal · session ready" },
    { kind: "muted", text: "Press Connect to open this server." },
  ];
  tab.remotePath = "~";
  tab.terminal?.reset();
  tab.terminal?.clear();
  for (const line of tab.output) writeOutputLine(tab, line);
}

function connectionLabel(tab = activeTab()) {
  if (tab?.connection === "connected") return "Secure session";
  if (tab?.connection === "connecting") return "Connecting";
  if (tab?.connection === "error") return "Connection issue";
  return "Ready";
}

function currentRemotePath() {
  return activeTab()?.remotePath || "~";
}

function parentRemotePath(value: string) {
  if (!value || value === "~" || value === "/") return value || "~";
  const clean = value.replace(/\/+$/, "");
  const index = clean.lastIndexOf("/");
  return index <= 0 ? "/" : clean.slice(0, index);
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`;
}

function terminalFontFamily(value: Preferences["terminalFontFamily"]) {
  if (value === "cascadia") return '"Cascadia Mono", "Cascadia Code", Consolas, monospace';
  if (value === "jetbrains") return '"JetBrains Mono", "Cascadia Mono", Consolas, monospace';
  if (value === "source-code") return '"Source Code Pro", "SFMono-Regular", Consolas, monospace';
  return '"SFMono-Regular", "Cascadia Mono", Consolas, monospace';
}

function terminalTheme(theme: Preferences["theme"]) {
  if (theme === "black") {
    return {
      background: "#00000000", foreground: "#e7e9f2", cursor: "#f1f2ff", cursorAccent: "#090a0e",
      selectionBackground: "#7688ff52", black: "#090a0e", red: "#ff6f8a", green: "#77e5bd",
      yellow: "#f1c86b", blue: "#86a8ff", magenta: "#d29cff", cyan: "#76e6ff", white: "#d9dce8",
      brightBlack: "#777f93", brightRed: "#ff91a5", brightGreen: "#9af0d0", brightYellow: "#ffe19c",
      brightBlue: "#adc2ff", brightMagenta: "#e4bdff", brightCyan: "#a6efff", brightWhite: "#ffffff",
    };
  }
  return {
    background: "#00000000", foreground: "#182033", cursor: "#27324d", cursorAccent: "#eef0f7",
    selectionBackground: "#5366c04a", black: "#101522", red: "#a51d3d", green: "#146b50",
    yellow: "#795800", blue: "#244f9e", magenta: "#71308f", cyan: "#006579", white: "#343b4d",
    brightBlack: "#525a6d", brightRed: "#c52e50", brightGreen: "#168061", brightYellow: "#916a00",
    brightBlue: "#315fba", brightMagenta: "#8b3eaa", brightCyan: "#007d94", brightWhite: "#151b2a",
  };
}

function recordRemoteDirectory(tab: TerminalTab, value: string) {
  try {
    const location = new URL(value);
    if (location.protocol !== "file:") return;
    const path = decodeURIComponent(location.pathname);
    if (path && path !== tab.remotePath) { tab.remotePath = path; scheduleCloudSync(); }
  } catch {
    // Ignore malformed or non-standard shell integration messages.
  }
}

function initials(name: string) {
  return name.split(/\s+/).slice(0, 2).map((part) => part[0]).join("").toUpperCase();
}

function keyName(name: string) {
  return name.toLowerCase().trim().replace(/[^a-z0-9]+/g, "_").replace(/^_+|_+$/g, "") || "server";
}

function detectedPlatform(): Platform {
  const platform = navigator.platform.toLowerCase();
  if (platform.includes("mac")) return "darwin";
  if (platform.includes("win")) return "windows";
  return "linux";
}

function normalisePlatform(value: unknown): Platform {
  return value === "darwin" || value === "windows" || value === "linux" ? value : detectedPlatform();
}

function escapeHtml(value: string) {
  return value.replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;" })[char]!);
}

async function initialise() {
  try {
    const initial = await backend("GetState");
    if (initial?.config) state.config = initial.config;
    state.config.teams = state.config.teams ?? [];
    state.platform = normalisePlatform(initial?.platform);
    state.biometricAvailable = Boolean(initial?.biometricAvailable);
    state.biometricName = String(initial?.biometricName || "Device authentication");
    state.selectedId = state.config.servers[0]?.id ?? "";
    state.readiness = await backend("GetServerReadiness");
  } catch {
    // Browser preview keeps the representative demo state.
  }
  state.config.teams = state.config.teams ?? [];
  restoreSidebarState();
  restoreWorkspace();
  await loadTerminalTranscripts();
  const initialTab = activeTab();
  if (!window.go && initialTab?.serverId === "demo-1") initialTab.remotePath = "/srv/northstar";
  attachRuntimeEvents();
  render();
  if (state.config.preferences.cloudUrl) void refreshCloudState();
  window.setInterval(() => { if (state.cloud.signedIn && !state.cloudLoading) void syncCloudWorkspace(false); }, 20000);
  if (initialTab?.restoreSession && state.config.preferences.reopenActiveSession) {
    requestAnimationFrame(() => void autoConnectTab(initialTab, true));
  }
}

async function loadTerminalTranscripts() {
  if (!state.config.preferences.persistTerminalHistory) return;
  await Promise.all(state.tabs.map(async (tab) => {
    try {
      tab.restoredTranscript = String(await backend("GetSessionTranscript", tab.id) ?? "");
    } catch {
      tab.restoredTranscript = "";
    }
  }));
}

window.addEventListener("keydown", (event) => {
  const terminalHasFocus = state.tabs.some((tab) => tab.connection === "connected" && tab.terminal?.textarea === document.activeElement);
  if (!state.modal && (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "f") {
    event.preventDefault();
    openTerminalSearch();
    return;
  }
  if (!state.modal && (event.metaKey || event.ctrlKey) && event.shiftKey && event.key.toLowerCase() === "p") {
    event.preventDefault();
    state.paletteQuery = "";
    state.modal = "palette";
    render();
    requestAnimationFrame(() => document.querySelector<HTMLInputElement>("#palette-input")?.focus());
    return;
  }
  if (state.platform === "darwin" && event.metaKey && (event.code === "Comma" || event.key === ",")) {
    event.preventDefault();
    openModal("settings");
    return;
  }
  if (!terminalHasFocus && (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    document.querySelector<HTMLInputElement>("#server-search")?.focus();
  }
  if (!state.modal && (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "t") {
    event.preventDefault();
    addTerminalTab();
    return;
  }
  if (!state.modal && (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "w") {
    event.preventDefault();
    void closeTerminalTab(state.activeTabId);
    return;
  }
  if (!state.modal && event.ctrlKey && event.key === "Tab") {
    event.preventDefault();
    const index = state.tabs.findIndex((tab) => tab.id === state.activeTabId);
    const direction = event.shiftKey ? -1 : 1;
    const next = state.tabs[(index + direction + state.tabs.length) % state.tabs.length];
    if (next) selectTab(next.id);
    return;
  }
  if (!state.modal && (event.metaKey || event.ctrlKey) && event.key === "\\") {
    event.preventDefault();
    toggleSplitPane();
    return;
  }
  if (event.key === "Escape" && state.modal) closeModal();
  else if (event.key === "Escape" && state.terminalSearch) closeTerminalSearch();
});

window.addEventListener("resize", () => requestAnimationFrame(fitVisibleTerminals));

void initialise();
