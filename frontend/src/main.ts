import "./style.css";
import "@xterm/xterm/css/xterm.css";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";

type Server = {
  id: string;
  name: string;
  host: string;
  port: number;
  user: string;
  shell: string;
  identity?: string;
  fingerprint?: string;
  favorite?: boolean;
  passwordSaved?: boolean;
  requireBiometric?: boolean;
};

type Preferences = {
  defaultUser: string;
  defaultPort: number;
  defaultShell: string;
  defaultIdentity?: string;
  logActivity: boolean;
  scrollback: number;
};

type Config = { servers: Server[]; preferences: Preferences };
type PublicKeyInfo = {
  path: string;
  privatePath?: string;
  name: string;
  fingerprint: string;
};
type ConnectionState = "idle" | "connecting" | "connected" | "error";
type ConnectionRequest = {
  password: string;
  rememberPassword: boolean;
  requireBiometric: boolean;
};

declare global {
  interface Window {
    go?: { main?: { App?: Record<string, (...args: unknown[]) => Promise<unknown>> } };
    runtime?: {
      WindowMinimise(): void;
      WindowToggleMaximise(): void;
      Quit(): void;
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
  servers: [
    { id: "demo-1", name: "Production", host: "api.northstar.dev", port: 22, user: "deploy", shell: "zsh", favorite: true, passwordSaved: true, requireBiometric: true },
    { id: "demo-2", name: "Staging", host: "stage.northstar.dev", port: 22, user: "ubuntu", shell: "bash", favorite: true },
    { id: "demo-3", name: "Home lab", host: "192.168.1.42", port: 22, user: "operator", shell: "fish" },
  ],
  preferences: {
    defaultUser: "admin",
    defaultPort: 22,
    defaultShell: "default",
    defaultIdentity: "~/.ssh/id_ed25519",
    logActivity: true,
    scrollback: 2000,
  },
};

const state = {
  config: demoConfig,
  selectedId: demoConfig.servers[0]?.id ?? "",
  query: "",
  connection: "idle" as ConnectionState,
  output: [
    { kind: "system", text: "SSHKing secure terminal · session ready" },
    { kind: "muted", text: "Select a server and connect when you’re ready." },
  ],
  modal: "" as "" | "server" | "settings" | "connect" | "zed" | "ssh-key" | "trust-host-key",
  editingId: "",
  sshKeys: [] as PublicKeyInfo[],
  biometricAvailable: false,
  biometricName: "Device authentication",
  pendingConnection: null as ConnectionRequest | null,
  pendingHostFingerprint: "",
};

const app = document.querySelector<HTMLDivElement>("#app")!;
let terminal: Terminal | undefined;
let fitAddon: FitAddon | undefined;
let terminalHost: HTMLDivElement | undefined;
let inputQueue = "";
let sendingInput = false;
let resizeTimer: number | undefined;

function backend(name: string, ...args: unknown[]): Promise<any> {
  const fn = window.go?.main?.App?.[name];
  if (fn) return fn(...args);
  return mockBackend(name, args);
}

async function mockBackend(name: string, args: unknown[]) {
  if (name === "GetState") return {
    config: demoConfig,
    platform: navigator.platform.toLowerCase().includes("mac") ? "darwin" : "windows",
    biometricAvailable: true,
    biometricName: navigator.platform.toLowerCase().includes("mac") ? "Touch ID" : "Windows Hello",
  };
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
  if (name === "SavePreferences") {
    state.config.preferences = args[0] as Preferences;
    return state.config;
  }
  if (name === "Connect") {
    await new Promise((resolve) => setTimeout(resolve, 650));
    const selected = state.config.servers.find((server) => server.id === args[0]);
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
}

function render() {
  const selected = state.config.servers.find((server) => server.id === state.selectedId);
  if (terminalHost?.isConnected) terminalHost.remove();
  app.innerHTML = `
    <main class="window-shell">
      <div class="ambient ambient-a"></div>
      <div class="ambient ambient-b"></div>
      <header class="titlebar">
        <div class="brand">
          <span class="brand-mark">${icons.terminal}</span>
          <span>SSHKing</span>
        </div>
        <div class="session-island">
          <span class="status-dot ${state.connection}"></span>
          <span>${connectionLabel()}</span>
        </div>
        <div class="window-controls">
          <button data-window="min" aria-label="Minimise"><span></span></button>
          <button data-window="max" aria-label="Maximise"><span class="max-icon"></span></button>
          <button data-window="close" class="close" aria-label="Close"><span></span></button>
        </div>
      </header>

      <section class="workspace">
        <aside class="sidebar glass-panel">
          <div class="sidebar-heading">
            <div>
              <span class="eyebrow">Your space</span>
              <h1>Servers</h1>
            </div>
            <button class="orb-button" id="add-server" aria-label="Add server">${icons.plus}</button>
          </div>
          <label class="searchbox">
            ${icons.search}
            <input id="server-search" value="${escapeHtml(state.query)}" placeholder="Search servers" autocomplete="off" />
            <kbd>⌘ K</kbd>
          </label>
          <nav class="server-list">${serverList()}</nav>
          <div class="sidebar-footer">
            <button class="footer-button" id="open-settings">
              <span class="footer-icon">${icons.settings}</span>
              <span><strong>Preferences</strong><small>${state.config.preferences.logActivity ? "Activity logging on" : "Private mode"}</small></span>
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
              <button class="zed-button" id="open-zed" ${selected ? "" : "disabled"} aria-label="Open in Zed">${icons.code}<span>Zed</span></button>
              <button class="icon-button" id="edit-server" ${selected ? "" : "disabled"} aria-label="Edit server">${icons.more}</button>
              <button class="connect-button ${state.connection === "connected" ? "connected" : ""}" id="connect-button" ${selected ? "" : "disabled"}>
                <span>${state.connection === "connected" ? "Disconnect" : state.connection === "connecting" ? "Connecting…" : "Connect"}</span>
                ${icons.arrow}
              </button>
            </div>
          </div>

          <div class="tabbar">
            <button class="terminal-tab active"><span class="status-dot ${state.connection}"></span>${escapeHtml(selected?.name ?? "Terminal")}</button>
            <button class="new-tab" aria-label="New terminal">${icons.plus}</button>
            <span class="encryption"><i></i> end-to-end SSH</span>
          </div>

          <div class="terminal-stage" id="terminal-stage">
            <div class="terminal-glow"></div>
            <div class="terminal-output" id="terminal-output"></div>
          </div>
        </section>
      </section>
      ${state.modal ? modalMarkup() : ""}
    </main>`;
  bindEvents();
  mountTerminal();
}

function serverList() {
  const query = state.query.toLowerCase().trim();
  const servers = state.config.servers.filter((server) =>
    `${server.name} ${server.host} ${server.user}`.toLowerCase().includes(query),
  );
  if (!servers.length) {
    return `<div class="empty-servers"><span>${icons.server}</span><strong>No servers found</strong><small>Try another search or add one.</small></div>`;
  }
  const favorite = servers.filter((server) => server.favorite);
  const others = servers.filter((server) => !server.favorite);
  return [groupMarkup("Pinned", favorite), groupMarkup("All servers", others)].join("");
}

function groupMarkup(label: string, servers: Server[]) {
  if (!servers.length) return "";
  return `<div class="server-group"><div class="group-label">${label}<span>${servers.length}</span></div>${servers.map(serverCard).join("")}</div>`;
}

function serverCard(server: Server) {
  const active = server.id === state.selectedId;
  return `<button class="server-card ${active ? "active" : ""}" data-server="${escapeHtml(server.id)}">
    <span class="server-avatar">${initials(server.name)}</span>
    <span class="server-copy"><strong>${escapeHtml(server.name)}</strong><small>${escapeHtml(server.user)}@${escapeHtml(server.host)}</small></span>
    <span class="server-state ${active && state.connection === "connected" ? "online" : ""}"></span>
  </button>`;
}

function modalMarkup() {
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
      <label class="form-field"><span>Remote path</span><input name="remotePath" value="~" placeholder="~/project or /etc/nginx/nginx.conf" autocomplete="off" spellcheck="false" required></label>
      <div class="zed-destination">${escapeHtml(selected ? `${selected.user}@${selected.host}:${selected.port}` : "")}</div>
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
    return `<div class="modal-backdrop"><form class="modal glass-modal" id="settings-form">
      <div class="modal-handle"></div>
      <div class="modal-header"><div><span class="eyebrow">Workspace</span><h2>Preferences</h2></div><button type="button" class="modal-close">×</button></div>
      <div class="form-grid">
        ${field("Default user", "defaultUser", p.defaultUser)}
        ${field("Default port", "defaultPort", String(p.defaultPort), "number")}
        ${field("Default shell", "defaultShell", p.defaultShell)}
        ${field("Default identity", "defaultIdentity", p.defaultIdentity ?? "")}
        ${field("Scrollback lines", "scrollback", String(p.scrollback), "number")}
        <label class="switch-row"><span><strong>Activity logs</strong><small>Stored locally and streamed directly to disk</small></span><input name="logActivity" type="checkbox" ${p.logActivity ? "checked" : ""}><i></i></label>
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
      ${field("Host", "host", existing?.host ?? "", "text", "server.example.com")}
      ${field("User", "user", existing?.user ?? p.defaultUser)}
      ${field("Port", "port", String(existing?.port ?? p.defaultPort), "number")}
      ${field("Remote shell", "shell", existing?.shell ?? p.defaultShell, "text", "default, zsh, bash, fish")}
      ${field("Private key (optional)", "identity", existing?.identity ?? p.defaultIdentity ?? "", "text", "~/.ssh/id_ed25519", false)}
      <div class="full">${field("Pinned fingerprint (optional)", "fingerprint", existing?.fingerprint ?? "", "text", "SHA256:…", false)}</div>
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
  document.querySelectorAll<HTMLElement>("[data-window]").forEach((button) => button.onclick = () => {
    const action = button.dataset.window;
    if (action === "min") window.runtime?.WindowMinimise();
    if (action === "max") window.runtime?.WindowToggleMaximise();
    if (action === "close") window.runtime?.Quit();
  });
  document.querySelectorAll<HTMLButtonElement>("[data-server]").forEach((button) => button.onclick = () => {
    state.selectedId = button.dataset.server ?? "";
    state.output = [{ kind: "system", text: "SSHKing secure terminal · session ready" }, { kind: "muted", text: "Press Connect to open this server." }];
    resetTerminal();
    render();
  });
  const search = document.querySelector<HTMLInputElement>("#server-search");
  search?.addEventListener("input", () => { state.query = search.value; render(); });
  document.querySelector("#add-server")?.addEventListener("click", () => openModal("server"));
  document.querySelector("#open-settings")?.addEventListener("click", () => openModal("settings"));
  document.querySelector("#edit-server")?.addEventListener("click", () => {
    state.editingId = state.selectedId;
    openModal("server");
  });
  document.querySelector("#setup-key")?.addEventListener("click", openSSHKeyModal);
  document.querySelector("#open-zed")?.addEventListener("click", () => {
    state.modal = "zed";
    render();
    setTimeout(() => document.querySelector<HTMLInputElement>('input[name="remotePath"]')?.select(), 20);
  });
  document.querySelectorAll(".modal-close").forEach((button) => button.addEventListener("click", closeModal));
  document.querySelector("#connect-button")?.addEventListener("click", toggleConnection);
  document.querySelector<HTMLFormElement>("#server-form")?.addEventListener("submit", saveServer);
  document.querySelector<HTMLFormElement>("#settings-form")?.addEventListener("submit", saveSettings);
  document.querySelector<HTMLFormElement>("#connect-form")?.addEventListener("submit", connectWithCredentials);
  document.querySelector("#trust-host-key")?.addEventListener("click", trustHostKey);
  document.querySelector<HTMLFormElement>("#zed-form")?.addEventListener("submit", openInZed);
  document.querySelector<HTMLFormElement>("#ssh-key-form")?.addEventListener("submit", installSSHKey);
  const rememberPassword = document.querySelector<HTMLInputElement>('input[name="rememberPassword"]');
  const requireBiometric = document.querySelector<HTMLInputElement>('input[name="requireBiometric"]');
  rememberPassword?.addEventListener("change", () => {
    if (!requireBiometric || !state.biometricAvailable) return;
    requireBiometric.disabled = !rememberPassword.checked;
    if (!rememberPassword.checked) requireBiometric.checked = false;
  });
  document.querySelector("#delete-server")?.addEventListener("click", deleteServer);
}

function openModal(type: "server" | "settings") {
  if (type === "server" && !state.editingId) state.editingId = "";
  state.modal = type;
  render();
  setTimeout(() => document.querySelector<HTMLInputElement>(".modal input:not([type=hidden])")?.focus(), 20);
}

function closeModal() {
  if (state.modal === "trust-host-key") {
    state.pendingConnection = null;
    state.pendingHostFingerprint = "";
  }
  state.modal = "";
  state.editingId = "";
  render();
}

async function saveServer(event: SubmitEvent) {
  event.preventDefault();
  const form = new FormData(event.currentTarget as HTMLFormElement);
  const existing = state.config.servers.find((server) => server.id === String(form.get("id")));
  const server: Server = {
    id: String(form.get("id") ?? ""),
    name: String(form.get("name") ?? ""),
    host: String(form.get("host") ?? ""),
    user: String(form.get("user") ?? ""),
    port: Number(form.get("port") ?? 22),
    shell: String(form.get("shell") ?? "default"),
    identity: String(form.get("identity") ?? ""),
    fingerprint: String(form.get("fingerprint") ?? ""),
    favorite: existing?.favorite ?? false,
    passwordSaved: existing?.passwordSaved ?? false,
    requireBiometric: existing?.requireBiometric ?? false,
  };
  state.config = await backend("SaveServer", server);
  state.selectedId = server.id || state.config.servers.at(-1)?.id || "";
  closeModal();
}

async function saveSettings(event: SubmitEvent) {
  event.preventDefault();
  const form = new FormData(event.currentTarget as HTMLFormElement);
  const preferences: Preferences = {
    defaultUser: String(form.get("defaultUser") ?? ""),
    defaultPort: Number(form.get("defaultPort") ?? 22),
    defaultShell: String(form.get("defaultShell") ?? "default"),
    defaultIdentity: String(form.get("defaultIdentity") ?? ""),
    scrollback: Number(form.get("scrollback") ?? 2000),
    logActivity: form.get("logActivity") === "on",
  };
  state.config = await backend("SavePreferences", preferences);
  closeModal();
}

async function deleteServer() {
  if (!state.editingId) return;
  state.config = await backend("DeleteServer", state.editingId);
  state.selectedId = state.config.servers[0]?.id ?? "";
  closeModal();
}

async function toggleConnection() {
  if (state.connection === "connected") {
    await backend("Disconnect");
    state.connection = "idle";
    pushOutput({ kind: "muted", text: "Session disconnected." });
    render();
    return;
  }
  state.modal = "connect";
  render();
}

async function connectWithCredentials(event: SubmitEvent) {
  event.preventDefault();
  const form = new FormData(event.currentTarget as HTMLFormElement);
  await attemptConnection({
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
  state.modal = "";
  state.connection = "connecting";
  pushOutput({ kind: "system", text: `Opening secure session…` });
  render();
  try {
    await backend("Connect", state.selectedId, request.password, request.rememberPassword, request.requireBiometric, trustNewHost);
    const refreshed = await backend("GetState");
    if (refreshed?.config) state.config = refreshed.config;
    state.pendingConnection = null;
    state.pendingHostFingerprint = "";
    state.connection = "connected";
    pushOutput({ kind: "success", text: "Connected · remote shell is ready" });
    fitTerminal();
    await backend("ResizeTerminal", terminal?.cols ?? 120, terminal?.rows ?? 34);
    if (!window.go) {
      pushOutput({ kind: "output", text: "Last login: Today from 192.168.1.8" });
      pushOutput({ kind: "output", text: "Welcome to Northstar Linux 24.04 LTS" });
    }
  } catch (error) {
    const message = String(error);
    const fingerprint = message.includes("host key verification required")
      ? message.match(/SHA256:[A-Za-z0-9+/=]+/)?.[0]
      : undefined;
    if (fingerprint && !trustNewHost) {
      state.connection = "idle";
      state.pendingConnection = request;
      state.pendingHostFingerprint = fingerprint;
      state.modal = "trust-host-key";
      render();
      return;
    }
    state.connection = "error";
    pushOutput({ kind: "error", text: message });
  }
  render();
  if (state.connection === "connected") {
    requestAnimationFrame(() => {
      fitTerminal();
      terminal?.focus();
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
    pushOutput({ kind: "success", text: `Opening ${remotePath} in Zed…` });
  } catch (error) {
    state.modal = "";
    pushOutput({ kind: "error", text: `Could not open Zed: ${String(error)}` });
  }
  render();
}

async function openSSHKeyModal() {
  try {
    state.sshKeys = await backend("ListSSHKeys", state.selectedId);
    state.modal = "ssh-key";
    render();
  } catch (error) {
    pushOutput({ kind: "error", text: `Could not list SSH keys: ${String(error)}` });
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
    state.modal = "";
    pushOutput({ kind: "success", text: "SSH key installed · future connections can use the private key" });
  } catch (error) {
    state.modal = "";
    pushOutput({ kind: "error", text: `Could not install SSH key: ${String(error)}` });
  }
  render();
}

function attachRuntimeEvents() {
  window.runtime?.EventsOn("terminal:data", (data) => {
    terminal?.write(String(data));
  });
  window.runtime?.EventsOn("terminal:status", (payload) => {
    const event = payload as { state?: ConnectionState | "disconnected"; message?: string };
    state.connection = event.state === "disconnected" ? "idle" : event.state ?? "idle";
    if (event.message) pushOutput({ kind: "error", text: event.message });
    render();
  });
}

function pushOutput(line: { kind: string; text: string }) {
  state.output.push(line);
  const excess = state.output.length - state.config.preferences.scrollback;
  if (excess > 0) state.output.splice(0, excess);
  if (!terminal) return;
  const color = {
    system: "\x1b[38;2;38;53;91m",
    muted: "\x1b[38;2;75;83;103m",
    success: "\x1b[38;2;20;107;80m",
    error: "\x1b[38;2;165;29;61m",
  }[line.kind] ?? "\x1b[0m";
  terminal.writeln(`\r${color}${line.text}\x1b[0m`);
}

function mountTerminal() {
  const placeholder = document.querySelector<HTMLDivElement>("#terminal-output");
  if (!placeholder) return;

  if (terminalHost && terminal) {
    placeholder.replaceWith(terminalHost);
  } else {
    terminalHost = placeholder;
    terminal = new Terminal({
      cursorBlink: true,
      cursorStyle: "bar",
      fontFamily: '"Cascadia Mono", "SFMono-Regular", Consolas, monospace',
      fontSize: 14,
      fontWeight: "500",
      fontWeightBold: "700",
      lineHeight: 1.4,
      scrollback: state.config.preferences.scrollback,
      allowTransparency: true,
      theme: {
        background: "#00000000",
        foreground: "#182033",
        cursor: "#27324d",
        cursorAccent: "#eef0f7",
        selectionBackground: "#5366c04a",
        black: "#101522",
        red: "#a51d3d",
        green: "#146b50",
        yellow: "#795800",
        blue: "#244f9e",
        magenta: "#71308f",
        cyan: "#006579",
        white: "#343b4d",
        brightBlack: "#525a6d",
        brightRed: "#c52e50",
        brightGreen: "#168061",
        brightYellow: "#916a00",
        brightBlue: "#315fba",
        brightMagenta: "#8b3eaa",
        brightCyan: "#007d94",
        brightWhite: "#151b2a",
      },
    });
    fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(terminalHost);
    terminal.onData(queueTerminalInput);
    terminal.onResize(({ cols, rows }) => scheduleTerminalResize(cols, rows));
    for (const line of state.output) pushOutputWithoutState(line);
  }

  document.querySelector("#terminal-stage")?.addEventListener("mousedown", () => terminal?.focus());
  requestAnimationFrame(fitTerminal);
}

function pushOutputWithoutState(line: { kind: string; text: string }) {
  if (!terminal) return;
  const color = line.kind === "system" ? "\x1b[38;2;38;53;91m" : "\x1b[38;2;75;83;103m";
  terminal.writeln(`${color}${line.text}\x1b[0m`);
}

function queueTerminalInput(data: string) {
  if (state.connection !== "connected") return;
  inputQueue += data;
  void flushTerminalInput();
}

async function flushTerminalInput() {
  if (sendingInput) return;
  sendingInput = true;
  try {
    while (inputQueue) {
      const data = inputQueue;
      inputQueue = "";
      await backend("SendInput", data);
    }
  } catch (error) {
    pushOutput({ kind: "error", text: `Input failed: ${String(error)}` });
  } finally {
    sendingInput = false;
    if (inputQueue) void flushTerminalInput();
  }
}

function fitTerminal() {
  if (!terminalHost?.isConnected) return;
  try {
    fitAddon?.fit();
  } catch {
    // The fit addon can run before WebView layout has completed.
  }
}

function scheduleTerminalResize(cols: number, rows: number) {
  window.clearTimeout(resizeTimer);
  resizeTimer = window.setTimeout(() => {
    if (state.connection === "connected") void backend("ResizeTerminal", cols, rows);
  }, 80);
}

function resetTerminal() {
  inputQueue = "";
  terminal?.reset();
  terminal?.clear();
  terminal?.writeln("\x1b[38;2;38;53;91mSSHKing secure terminal · session ready\x1b[0m");
  terminal?.writeln("\x1b[38;2;75;83;103mPress Connect to open this server.\x1b[0m");
}

function connectionLabel() {
  if (state.connection === "connected") return "Secure session";
  if (state.connection === "connecting") return "Connecting";
  if (state.connection === "error") return "Connection issue";
  return "Ready";
}

function initials(name: string) {
  return name.split(/\s+/).slice(0, 2).map((part) => part[0]).join("").toUpperCase();
}

function keyName(name: string) {
  return name.toLowerCase().trim().replace(/[^a-z0-9]+/g, "_").replace(/^_+|_+$/g, "") || "server";
}

function escapeHtml(value: string) {
  return value.replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;" })[char]!);
}

async function initialise() {
  try {
    const initial = await backend("GetState");
    if (initial?.config) state.config = initial.config;
    state.biometricAvailable = Boolean(initial?.biometricAvailable);
    state.biometricName = String(initial?.biometricName || "Device authentication");
    state.selectedId = state.config.servers[0]?.id ?? "";
  } catch {
    // Browser preview keeps the representative demo state.
  }
  attachRuntimeEvents();
  render();
}

window.addEventListener("keydown", (event) => {
  const terminalHasFocus = state.connection === "connected" && terminal?.textarea === document.activeElement;
  if (!terminalHasFocus && (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    document.querySelector<HTMLInputElement>("#server-search")?.focus();
  }
  if (event.key === "Escape" && state.modal) closeModal();
});

window.addEventListener("resize", () => requestAnimationFrame(fitTerminal));

void initialise();
