import './style.css';
import './app.css';

// Wails backend bridge - only available in production build
// In dev mode, we mock these for UI iteration
let wailsReady = false;
let backend = {};

// ----------------------------------------------------------------
// Load Wails bindings
// ----------------------------------------------------------------
async function loadBackend() {
  try {
    const mod = await import('../wailsjs/go/main/App.js');
    const runtime = await import('../wailsjs/runtime/runtime.js');
    backend = { ...mod, runtime };
    wailsReady = true;
  } catch (e) {
    console.warn('Wails backend not available (dev mode). Using mock data.');
    backend = createMockBackend();
  }
}

function createMockBackend() {
  return {
    GetSystemInfo: async () => ({
      distroName: 'Arch Linux',
      distroId: 'arch',
      nativeManager: 'pacman',
      hasFlatpak: true,
      hasSnap: false,
      detectedManagers: ['pacman', 'flatpak'],
      arch: 'amd64',
      kernel: '6.10.0-arch1',
    }),
    GetInstalledApps: async () => [
      { name: 'neovim', displayName: 'Neovim', version: 'v0.10.0', source: 'tar.gz', sizeMb: 14.3, installedAt: new Date().toISOString() },
      { name: 'vlc', displayName: 'VLC', version: '3.0.21', source: 'flatpak', sizeMb: 68.4, installedAt: new Date().toISOString() },
    ],
    SearchPackages: async (query) => {
      await sleep(500);
      return [
        { name: query, version: '1.0.0', description: `Mock result for "${query}"`, backend: 'pacman', installed: false },
        { name: query + '-extra', version: '2.0.0', description: `Extra package for "${query}"`, backend: 'flatpak', installed: false },
      ];
    },
    InstallFromURL: async (url) => {
      await sleep(3000);
      const name = url.split('/').pop().split('.')[0] || 'app';
      return { success: true, appName: name, version: 'latest', executable: `~/.local/bin/${name}`, desktopFile: `~/.local/share/applications/${name}.desktop`, installedDir: `~/.local/share/lpm/apps/${name}`, message: 'App installed!' };
    },
    InstallFromFile: async (file) => {
      await sleep(2000);
      const name = file.split('/').pop().split('.')[0] || 'app';
      return { success: true, appName: name, version: 'latest', executable: `~/.local/bin/${name}`, desktopFile: `~/.local/share/applications/${name}.desktop`, installedDir: `~/.local/share/lpm/apps/${name}`, message: 'App installed!' };
    },
    SelectArchiveFile: async () => '/home/user/downloads/example-v1.0.tar.gz',
    UninstallApp: async () => {},
    InstallSystemPackage: async (pkg, backend) => `Successfully installed ${pkg} via ${backend}`,
    LaunchApp: async () => {},
    OpenAppDirectory: async () => {},
    CleanCache: async () => ({ success: true, message: 'Cache cleaned' }),
    EventsOn: (event, cb) => { /* noop in mock */ },
    EventsOff: (event) => {},
  };
}

const sleep = ms => new Promise(r => setTimeout(r, ms));

// ----------------------------------------------------------------
// App State
// ----------------------------------------------------------------
const state = {
  activePanel: 'sideload',
  systemInfo: null,
  installedApps: [],
  searchResults: [],
  searchQuery: '',
  isSearching: false,
  logs: [],
  consoleExpanded: false,
  installing: false,
  installProgress: { step: '', message: '', percent: 0, speed: '' },
  lastInstallResult: null,
};

// ----------------------------------------------------------------
// Toast notifications
// ----------------------------------------------------------------
function showToast(msg, type = 'info', duration = 4000) {
  const container = document.getElementById('toast-container');
  const toast = document.createElement('div');
  toast.className = `toast ${type}`;
  const icons = { success: '✓', error: '✕', info: 'ℹ' };
  toast.innerHTML = `<span>${icons[type] || '•'}</span><span>${msg}</span>`;
  container.appendChild(toast);
  setTimeout(() => {
    toast.style.animation = 'none';
    toast.style.opacity = '0';
    toast.style.transform = 'translateX(60px)';
    toast.style.transition = 'all 0.25s';
    setTimeout(() => toast.remove(), 250);
  }, duration);
}

// ----------------------------------------------------------------
// Console Log
// ----------------------------------------------------------------
function addLog(message, type = 'info') {
  const now = new Date();
  const time = now.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
  state.logs.push({ message, type, time });
  renderConsole();
}

function renderConsole() {
  const body = document.getElementById('console-body');
  if (!body) return;
  const wasAtBottom = body.scrollTop + body.clientHeight >= body.scrollHeight - 20;
  const html = state.logs.slice(-200).map(log =>
    `<div class="log-line ${log.type}"><span class="log-time">${log.time}</span><span class="log-msg">${escHtml(log.message)}</span></div>`
  ).join('');
  body.innerHTML = html;
  if (wasAtBottom) body.scrollTop = body.scrollHeight;

  // Update console dot
  const dot = document.getElementById('console-dot');
  if (dot) {
    dot.className = state.logs.length > 0 ? 'console-dot' : 'console-dot inactive';
  }
}

// ----------------------------------------------------------------
// Navigation
// ----------------------------------------------------------------
function navigate(panel) {
  state.activePanel = panel;

  // Update nav items
  document.querySelectorAll('.nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.panel === panel);
  });

  // Show correct panel
  document.querySelectorAll('.panel').forEach(el => {
    el.classList.toggle('active', el.id === `panel-${panel}`);
  });

  // Update topbar title
  const titles = {
    sideload: '📥 Sideload & Install',
    explore:  '🔍 Explore Packages',
    installed: '📦 Installed Applications',
    cleaner: '🧹 System & Cache',
  };
  document.getElementById('topbar-title').textContent = titles[panel] || 'Omnipack';

  // Load data for panel
  if (panel === 'installed') loadInstalledApps();
}

// ----------------------------------------------------------------
// Sideload Panel
// ----------------------------------------------------------------
function setupSideloadPanel() {
  const dropzone = document.getElementById('dropzone');
  const urlInput = document.getElementById('url-input');
  const nameInput = document.getElementById('name-input');
  const installBtn = document.getElementById('install-url-btn');
  const fileBtn = document.getElementById('choose-file-btn');

  // Drag & Drop
  dropzone.addEventListener('dragover', e => {
    e.preventDefault();
    dropzone.classList.add('drag-over');
  });
  dropzone.addEventListener('dragleave', () => dropzone.classList.remove('drag-over'));
  dropzone.addEventListener('drop', async e => {
    e.preventDefault();
    dropzone.classList.remove('drag-over');
    const files = Array.from(e.dataTransfer?.files || []);
    if (files.length === 0) return;
    const file = files[0];
    await doInstallFile(file.path || file.name, '');
  });
  dropzone.addEventListener('click', async () => {
    const file = await backend.SelectArchiveFile();
    if (file) await doInstallFile(file, '');
  });

  // File chooser button
  fileBtn.addEventListener('click', async () => {
    const file = await backend.SelectArchiveFile();
    if (file) await doInstallFile(file, '');
  });

  // URL install
  installBtn.addEventListener('click', async () => {
    const url = urlInput.value.trim();
    if (!url) { showToast('Please enter a URL', 'error'); return; }
    await doInstallURL(url, nameInput.value.trim());
  });

  urlInput.addEventListener('keydown', e => {
    if (e.key === 'Enter') installBtn.click();
  });
}

async function doInstallURL(url, name) {
  if (state.installing) return;
  state.installing = true;
  resetInstallUI();
  showProgress(true);

  addLog(`Starting URL install: ${url}`, 'info');

  try {
    // Listen for progress events if Wails is available
    if (wailsReady && backend.runtime?.EventsOn) {
      backend.runtime.EventsOn('install:progress', (data) => {
        updateInstallProgress(data);
      });
    } else {
      // Simulate progress for mock
      simulateProgress();
    }

    const result = await backend.InstallFromURL(url, name);

    if (wailsReady && backend.runtime?.EventsOff) {
      backend.runtime.EventsOff('install:progress');
    }

    handleInstallResult(result);
  } catch (err) {
    handleInstallError(err.message || String(err));
  } finally {
    state.installing = false;
  }
}

async function doInstallFile(filePath, name) {
  if (state.installing) return;
  state.installing = true;
  resetInstallUI();
  showProgress(true);

  addLog(`Starting file install: ${filePath}`, 'info');

  try {
    if (wailsReady && backend.runtime?.EventsOn) {
      backend.runtime.EventsOn('install:progress', (data) => {
        updateInstallProgress(data);
      });
    } else {
      simulateProgress();
    }

    const result = await backend.InstallFromFile(filePath, name);

    if (wailsReady && backend.runtime?.EventsOff) {
      backend.runtime.EventsOff('install:progress');
    }

    handleInstallResult(result);
  } catch (err) {
    handleInstallError(err.message || String(err));
  } finally {
    state.installing = false;
  }
}

function simulateProgress() {
  const steps = [
    { step: 'downloading', message: 'Connecting and downloading...', percent: 15, speed: '4.3 MB/s' },
    { step: 'downloading', message: 'Downloading archive...', percent: 55, speed: '7.1 MB/s' },
    { step: 'extracting', message: 'Safely extracting archive contents...', percent: 70, speed: '' },
    { step: 'scanning', message: 'Scanning for Linux ELF executables...', percent: 82, speed: '' },
    { step: 'integrating', message: 'Creating PATH symlink and desktop launcher...', percent: 95, speed: '' },
  ];
  steps.forEach((s, i) => {
    setTimeout(() => updateInstallProgress(s), i * 600);
  });
}

function updateInstallProgress(data) {
  state.installProgress = data;

  const fill = document.getElementById('progress-fill');
  const label = document.getElementById('progress-label');
  const pct = document.getElementById('progress-pct');
  const speed = document.getElementById('progress-speed');

  if (fill) fill.style.width = `${data.percent}%`;
  if (label) label.textContent = data.message || '';
  if (pct) pct.textContent = `${Math.round(data.percent)}%`;
  if (speed) speed.textContent = data.speed || '';

  addLog(`[${data.step}] ${data.message}`, 'info');

  // Update step indicators
  const stepNames = ['downloading', 'extracting', 'scanning', 'integrating', 'complete'];
  stepNames.forEach(step => {
    const el = document.getElementById(`step-${step}`);
    if (!el) return;
    el.className = 'progress-step';
    const stepIndex = stepNames.indexOf(step);
    const currentIndex = stepNames.indexOf(data.step);
    if (data.step === 'failed') {
      el.className = 'progress-step error';
    } else if (stepIndex < currentIndex) {
      el.className = 'progress-step done';
    } else if (stepIndex === currentIndex) {
      el.className = 'progress-step current';
    }
  });
}

function handleInstallResult(result) {
  if (result.success) {
    updateInstallProgress({ step: 'complete', message: 'Installation complete!', percent: 100, speed: '' });

    // Show success
    const banner = document.getElementById('success-banner');
    if (banner) {
      banner.classList.add('visible');
      document.getElementById('sb-name').textContent = result.appName || '—';
      document.getElementById('sb-executable').textContent = result.executable || '—';
      document.getElementById('sb-desktop').textContent = result.desktopFile || '—';
      document.getElementById('sb-dir').textContent = result.installedDir || '—';

      const launchBtn = document.getElementById('sb-launch-btn');
      if (launchBtn) {
        launchBtn.onclick = () => backend.LaunchApp(result.appName);
      }
      const openBtn = document.getElementById('sb-open-dir-btn');
      if (openBtn) {
        openBtn.onclick = () => backend.OpenAppDirectory(result.appName);
      }
    }

    state.lastInstallResult = result;
    showToast(`✓ ${result.appName} installed successfully!`, 'success');
    addLog(`Installed: ${result.appName} → ${result.executable}`, 'success');
  } else {
    handleInstallError(result.message || 'Installation failed');
  }
}

function handleInstallError(msg) {
  updateInstallProgress({ step: 'failed', message: `Error: ${msg}`, percent: 0, speed: '' });
  document.getElementById('step-downloading')?.classList.add('error');
  showToast(`Installation failed: ${msg}`, 'error', 6000);
  addLog(`Error: ${msg}`, 'error');
}

function resetInstallUI() {
  const banner = document.getElementById('success-banner');
  if (banner) banner.classList.remove('visible');
  updateInstallProgress({ step: '', message: 'Preparing...', percent: 0, speed: '' });
  ['downloading', 'extracting', 'scanning', 'integrating', 'complete'].forEach(step => {
    const el = document.getElementById(`step-${step}`);
    if (el) el.className = 'progress-step';
  });
}

function showProgress(show) {
  const section = document.getElementById('progress-section');
  if (section) section.classList.toggle('visible', show);
}

// ----------------------------------------------------------------
// Explore / Search Panel
// ----------------------------------------------------------------
function setupSearchPanel() {
  const searchInput = document.getElementById('search-input');
  const searchBtn = document.getElementById('search-btn');

  let debounce = null;
  searchInput.addEventListener('input', () => {
    clearTimeout(debounce);
    debounce = setTimeout(() => doSearch(searchInput.value.trim()), 400);
  });
  searchInput.addEventListener('keydown', e => {
    if (e.key === 'Enter') doSearch(searchInput.value.trim());
  });
  searchBtn.addEventListener('click', () => doSearch(searchInput.value.trim()));
}

async function doSearch(query) {
  if (!query) {
    renderSearchResults([]);
    return;
  }
  state.isSearching = true;
  state.searchQuery = query;

  const resultsEl = document.getElementById('search-results');
  resultsEl.innerHTML = `<div class="empty-state"><div class="spinner"></div><p class="empty-state-sub" style="margin-top:12px;">Searching ${state.systemInfo?.nativeManager || 'repositories'} and Flatpak...</p></div>`;

  try {
    const results = await backend.SearchPackages(query);
    state.searchResults = results || [];
    renderSearchResults(state.searchResults);
    addLog(`Search "${query}" returned ${state.searchResults.length} results`, 'info');
  } catch (err) {
    resultsEl.innerHTML = `<div class="empty-state"><div class="empty-state-icon">⚠️</div><div class="empty-state-title">Search failed</div><p class="empty-state-sub">${escHtml(err.message || String(err))}</p></div>`;
    addLog(`Search error: ${err.message || err}`, 'error');
  } finally {
    state.isSearching = false;
  }
}

function renderSearchResults(results) {
  const el = document.getElementById('search-results');
  if (!results || results.length === 0) {
    el.innerHTML = `<div class="empty-state"><div class="empty-state-icon">🔍</div><div class="empty-state-title">No packages found</div><p class="empty-state-sub">Try a different package name or check your repository connection.</p></div>`;
    return;
  }

  const backendIcons = { pacman: '🏹', apt: '🟠', dnf: '🎩', flatpak: '📦', snap: '🔵' };

  el.innerHTML = `<div class="package-grid">${results.map(pkg => `
    <div class="package-card">
      <div class="pkg-icon ${escHtml(pkg.backend)}">${backendIcons[pkg.backend] || '📦'}</div>
      <div class="pkg-info">
        <div class="pkg-name">
          ${escHtml(pkg.name)}
          ${pkg.installed ? '<span class="installed-badge">Installed</span>' : ''}
        </div>
        <div class="pkg-desc">${escHtml(pkg.description || '—')}</div>
      </div>
      <div class="pkg-meta">
        <span class="pkg-version">${escHtml(pkg.version || '')}</span>
        <span class="backend-badge ${escHtml(pkg.backend)}">${escHtml(pkg.backend)}</span>
        ${!pkg.installed ? `<button class="btn btn-primary btn-sm" onclick="installSystemPkg('${escAttr(pkg.name)}', '${escAttr(pkg.backend)}')">Install</button>` : ''}
      </div>
    </div>
  `).join('')}</div>`;
}

window.installSystemPkg = async function(pkgName, pkgBackend) {
  showToast(`Installing ${pkgName} via ${pkgBackend}…`, 'info');
  addLog(`Installing ${pkgName} via ${pkgBackend}...`, 'info');
  try {
    const out = await backend.InstallSystemPackage(pkgName, pkgBackend);
    showToast(`${pkgName} installed!`, 'success');
    addLog(out, 'success');
  } catch (err) {
    showToast(`Failed: ${err.message || err}`, 'error', 6000);
    addLog(`Install error: ${err.message || err}`, 'error');
  }
};

// ----------------------------------------------------------------
// Installed Apps Panel
// ----------------------------------------------------------------
async function loadInstalledApps() {
  const listEl = document.getElementById('installed-list');
  const countEl = document.getElementById('installed-count');

  listEl.innerHTML = `<div class="empty-state"><div class="spinner"></div><p class="empty-state-sub" style="margin-top:12px;">Loading installed applications...</p></div>`;

  try {
    const apps = await backend.GetInstalledApps();
    state.installedApps = apps || [];
    if (countEl) countEl.textContent = state.installedApps.length;
    renderInstalledApps(state.installedApps);
  } catch (err) {
    listEl.innerHTML = `<div class="empty-state"><div class="empty-state-icon">⚠️</div><div class="empty-state-title">Could not load apps</div><p class="empty-state-sub">${escHtml(err.message || String(err))}</p></div>`;
  }
}

function renderInstalledApps(apps) {
  const el = document.getElementById('installed-list');
  const filterEl = document.getElementById('installed-filter');
  const filter = filterEl ? filterEl.value : 'all';

  const filtered = apps.filter(app => {
    if (filter === 'all') return true;
    if (filter === 'direct') return !['flatpak', 'pacman', 'apt', 'dnf', 'snap'].includes(app.source);
    return app.source === filter;
  });

  if (filtered.length === 0) {
    el.innerHTML = `<div class="empty-state"><div class="empty-state-icon">📦</div><div class="empty-state-title">No applications found</div><p class="empty-state-sub">Install an application using Sideload or Explore.</p></div>`;
    return;
  }

  const sourceIcons = {
    'flatpak': '📦', 'pacman': '🏹', 'apt': '🟠', 'dnf': '🎩', 'snap': '🔵',
    'tar.gz': '🗜️', 'tar.xz': '🗜️', 'zip': '📁', 'appimage': '💿', 'binary': '⚡', 'direct': '⚡'
  };

  el.innerHTML = `<div class="app-list">${filtered.map(app => {
    const icon = sourceIcons[app.source] || '📦';
    const sizeMb = app.sizeMb ? `${app.sizeMb.toFixed(1)} MB` : '';
    const installDate = app.installedAt ? new Date(app.installedAt).toLocaleDateString() : '';
    const isDirectInstall = !['flatpak', 'pacman', 'apt', 'dnf', 'snap'].includes(app.source);

    return `
    <div class="app-card" id="app-card-${escAttr(app.name)}">
      <div class="app-card-icon">${icon}</div>
      <div class="app-card-info">
        <div class="app-card-name">
          ${escHtml(app.displayName || app.name)}
          <span class="backend-badge ${escHtml(app.source)}">${escHtml(app.source)}</span>
        </div>
        <div class="app-card-meta">
          ${app.version ? `<span class="app-card-meta-item font-mono text-sm">${escHtml(app.version)}</span>` : ''}
          ${sizeMb ? `<span class="app-card-meta-item text-sm">💾 ${sizeMb}</span>` : ''}
          ${installDate ? `<span class="app-card-meta-item text-sm">📅 ${installDate}</span>` : ''}
        </div>
      </div>
      <div class="app-card-actions">
        ${isDirectInstall ? `
          <button class="btn btn-success btn-sm" title="Launch" onclick="launchApp('${escAttr(app.name)}')">▶ Launch</button>
          <button class="btn btn-secondary btn-sm btn-icon" title="Open folder" onclick="openAppDir('${escAttr(app.name)}')">📂</button>
        ` : ''}
        <button class="btn btn-danger btn-sm" onclick="uninstallApp('${escAttr(app.name)}', '${escAttr(app.source)}')">🗑 Remove</button>
      </div>
    </div>`;
  }).join('')}</div>`;
}

window.launchApp = async function(name) {
  try {
    await backend.LaunchApp(name);
    showToast(`Launching ${name}…`, 'info', 2000);
  } catch (err) {
    showToast(`Failed to launch: ${err.message || err}`, 'error');
  }
};

window.openAppDir = async function(name) {
  try {
    await backend.OpenAppDirectory(name);
  } catch (err) {
    showToast(`Failed: ${err.message || err}`, 'error');
  }
};

window.uninstallApp = async function(name, source) {
  if (!confirm(`Cleanly uninstall "${name}"? This will remove the application, PATH symlinks, and desktop launcher.`)) return;
  try {
    await backend.UninstallApp(name, source);
    showToast(`${name} removed.`, 'success');
    addLog(`Uninstalled: ${name}`, 'info');
    state.installedApps = state.installedApps.filter(a => a.name !== name);
    renderInstalledApps(state.installedApps);
    document.getElementById('installed-count').textContent = state.installedApps.length;
  } catch (err) {
    showToast(`Uninstall failed: ${err.message || err}`, 'error');
    addLog(`Uninstall error: ${err.message || err}`, 'error');
  }
};

// ----------------------------------------------------------------
// Cleaner Panel
// ----------------------------------------------------------------
function setupCleanerPanel() {
  const cleanCacheBtn = document.getElementById('clean-cache-btn');
  if (cleanCacheBtn) {
    cleanCacheBtn.addEventListener('click', async () => {
      try {
        const result = await backend.CleanCache();
        showToast(result.message || 'Cache cleaned!', 'success');
        addLog('Cache cleared.', 'success');
      } catch (err) {
        showToast(`Failed: ${err.message || err}`, 'error');
      }
    });
  }
}

// ----------------------------------------------------------------
// System Info
// ----------------------------------------------------------------
async function loadSystemInfo() {
  try {
    state.systemInfo = await backend.GetSystemInfo();
    renderSidebarSysInfo(state.systemInfo);
    updateStatCards(state.systemInfo);
    addLog(`Detected: ${state.systemInfo.distroName} / ${state.systemInfo.nativeManager} / kernel ${state.systemInfo.kernel}`, 'info');
  } catch (err) {
    addLog(`System info error: ${err.message || err}`, 'error');
  }
}

function renderSidebarSysInfo(info) {
  const nameEl = document.getElementById('sys-distro');
  const mgrEl  = document.getElementById('sys-mgr');
  const tagsEl = document.getElementById('sys-tags');

  if (nameEl) nameEl.textContent = info.distroName || 'Linux';
  if (mgrEl) mgrEl.textContent = info.nativeManager ? `native: ${info.nativeManager}` : 'No native manager';
  if (tagsEl) {
    const tags = [];
    if (info.nativeManager) tags.push(`<span class="sys-tag native">${escHtml(info.nativeManager)}</span>`);
    if (info.hasFlatpak)    tags.push(`<span class="sys-tag flatpak">Flatpak</span>`);
    if (info.hasSnap)       tags.push(`<span class="sys-tag snap">Snap</span>`);
    tagsEl.innerHTML = tags.join('');
  }
}

function updateStatCards(info) {
  const managers = info.detectedManagers ? info.detectedManagers.length : 0;
  const mgrEl = document.getElementById('stat-managers');
  if (mgrEl) mgrEl.textContent = managers;
}

// ----------------------------------------------------------------
// Utility helpers
// ----------------------------------------------------------------
function escHtml(str) {
  if (!str && str !== 0) return '';
  return String(str).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
function escAttr(str) {
  if (!str) return '';
  return String(str).replace(/'/g, "\\'").replace(/"/g, '&quot;');
}

// ----------------------------------------------------------------
// Build the full HTML shell
// ----------------------------------------------------------------
function buildShell() {
  document.getElementById('app').innerHTML = `
    <div class="layout">

      <!-- SIDEBAR -->
      <aside class="sidebar">
        <div class="sidebar-logo">
          <div class="sidebar-logo-inner">
            <div class="sidebar-logo-icon">📦</div>
            <div class="sidebar-logo-text">
              <span class="sidebar-logo-name">Omnipack</span>
              <span class="sidebar-logo-sub">App Manager</span>
            </div>
          </div>
        </div>

        <nav class="sidebar-nav">
          <span class="nav-section-label">Install</span>
          <div class="nav-item active" data-panel="sideload" id="nav-sideload">
            <span class="nav-item-icon">📥</span>
            <span>Sideload</span>
          </div>
          <div class="nav-item" data-panel="explore" id="nav-explore">
            <span class="nav-item-icon">🔍</span>
            <span>Explore</span>
          </div>

          <span class="nav-section-label">Manage</span>
          <div class="nav-item" data-panel="installed" id="nav-installed">
            <span class="nav-item-icon">📦</span>
            <span>Installed</span>
            <span class="nav-item-badge" id="installed-count">—</span>
          </div>
          <div class="nav-item" data-panel="cleaner" id="nav-cleaner">
            <span class="nav-item-icon">🧹</span>
            <span>System</span>
          </div>
        </nav>

        <div class="sidebar-footer">
          <div class="sys-info-card">
            <div class="sys-info-distro">
              🐧 <span id="sys-distro">Detecting…</span>
            </div>
            <div class="sys-info-mgr" id="sys-mgr">Loading…</div>
            <div class="sys-info-tags" id="sys-tags"></div>
          </div>
        </div>
      </aside>

      <!-- MAIN -->
      <div class="main-content">
        <!-- Topbar -->
        <div class="topbar">
          <div class="topbar-title" id="topbar-title">📥 Sideload &amp; Install</div>
        </div>

        <!-- Panels -->
        <div class="content-panels">

          <!-- ── SIDELOAD PANEL ── -->
          <div class="panel active" id="panel-sideload">

            <!-- Stats row -->
            <div class="stats-row">
              <div class="stat-card">
                <div class="stat-label">Active Managers</div>
                <div class="stat-value" id="stat-managers">—</div>
                <div class="stat-sub">Detected on this system</div>
              </div>
              <div class="stat-card">
                <div class="stat-label">Formats Supported</div>
                <div class="stat-value">6</div>
                <div class="stat-sub">tar.gz, xz, zip, AppImage…</div>
              </div>
              <div class="stat-card">
                <div class="stat-label">Install Method</div>
                <div class="stat-value" style="font-size:15px;padding-top:4px;">Direct</div>
                <div class="stat-sub">~/.local/share/lpm</div>
              </div>
            </div>

            <!-- Dropzone -->
            <div id="dropzone" class="dropzone" role="button" tabindex="0" aria-label="Drop archive or AppImage here">
              <span class="dropzone-icon">⬇️</span>
              <div class="dropzone-title">Drop archive or AppImage here</div>
              <div class="dropzone-sub">or click to browse files</div>
              <div class="dropzone-tags">
                <span class="format-tag tgz">.tar.gz</span>
                <span class="format-tag tgz">.tar.xz</span>
                <span class="format-tag zip">.zip</span>
                <span class="format-tag img">.AppImage</span>
                <span class="format-tag deb">.deb</span>
                <span class="format-tag rpm">.rpm</span>
                <span class="format-tag bin">binary</span>
              </div>
            </div>

            <!-- Divider -->
            <div class="divider">Or install from URL</div>

            <!-- URL Install Card -->
            <div class="card">
              <div class="card-header">
                <span class="card-header-icon">🔗</span>
                <h2>Install from Link</h2>
              </div>
              <div class="card-body flex-col gap-3">
                <div class="form-group">
                  <label class="form-label" for="url-input">Download URL</label>
                  <div class="input-group">
                    <input id="url-input" class="input" type="url"
                      placeholder="https://github.com/user/app/releases/download/v1.0/app-linux-amd64.tar.gz"
                      autocomplete="off" spellcheck="false"/>
                  </div>
                </div>
                <div class="form-group">
                  <label class="form-label" for="name-input">Custom App Name <span class="text-muted">(optional)</span></label>
                  <input id="name-input" class="input" type="text"
                    placeholder="Auto-detected from filename" autocomplete="off"/>
                </div>
                <div class="flex gap-2">
                  <button id="install-url-btn" class="btn btn-primary btn-lg flex-1">
                    ⚡ Fetch &amp; Install
                  </button>
                  <button id="choose-file-btn" class="btn btn-secondary btn-lg">
                    📂 Browse File
                  </button>
                </div>
              </div>
            </div>

            <!-- Progress -->
            <div id="progress-section" class="card progress-section">
              <div class="card-header">
                <span class="card-header-icon">⚙️</span>
                <h2>Installation Progress</h2>
              </div>
              <div class="card-body flex-col gap-3">
                <div class="progress-header">
                  <span id="progress-label" class="progress-label">Preparing…</span>
                  <span id="progress-pct" class="progress-pct">0%</span>
                </div>
                <div class="progress-bar-wrap">
                  <div id="progress-fill" class="progress-bar-fill" style="width:0%"></div>
                </div>
                <span id="progress-speed" class="progress-speed"></span>
                <div class="progress-steps">
                  <div id="step-downloading" class="progress-step"><div class="step-dot"></div>Downloading archive</div>
                  <div id="step-extracting"  class="progress-step"><div class="step-dot"></div>Safe extraction (Zip-Slip protected)</div>
                  <div id="step-scanning"    class="progress-step"><div class="step-dot"></div>Detecting ELF executable</div>
                  <div id="step-integrating" class="progress-step"><div class="step-dot"></div>Creating ~/.local/bin symlink &amp; .desktop launcher</div>
                  <div id="step-complete"    class="progress-step"><div class="step-dot"></div>Ready</div>
                </div>
              </div>
            </div>

            <!-- Success Banner -->
            <div id="success-banner" class="success-banner">
              <div class="success-banner-title">✅ Installation Successful</div>
              <div class="success-banner-details">
                <div class="success-detail"><span class="success-detail-label">App Name</span><span id="sb-name"></span></div>
                <div class="success-detail"><span class="success-detail-label">Executable</span><span id="sb-executable"></span></div>
                <div class="success-detail"><span class="success-detail-label">Launcher</span><span id="sb-desktop"></span></div>
                <div class="success-detail"><span class="success-detail-label">Directory</span><span id="sb-dir"></span></div>
              </div>
              <div class="success-banner-actions">
                <button id="sb-launch-btn" class="btn btn-success">▶ Launch App</button>
                <button id="sb-open-dir-btn" class="btn btn-secondary">📂 Open Folder</button>
              </div>
            </div>

          </div> <!-- /panel-sideload -->


          <!-- ── EXPLORE PANEL ── -->
          <div class="panel" id="panel-explore">
            <div class="panel-header">
              <div class="panel-header-left">
                <h1>Explore Packages</h1>
                <p>Search across your native package manager and Flatpak simultaneously</p>
              </div>
            </div>

            <div class="search-box">
              <span class="search-icon">🔍</span>
              <input id="search-input" type="text" placeholder="Search packages… e.g. neovim, vlc, firefox"/>
              <button id="search-btn" class="btn btn-primary btn-sm">Search</button>
            </div>

            <div id="search-results">
              <div class="empty-state">
                <div class="empty-state-icon">🔍</div>
                <div class="empty-state-title">Search for packages</div>
                <p class="empty-state-sub">Type a package name to search across your system repositories and Flatpak.</p>
              </div>
            </div>
          </div> <!-- /panel-explore -->


          <!-- ── INSTALLED PANEL ── -->
          <div class="panel" id="panel-installed">
            <div class="panel-header">
              <div class="panel-header-left">
                <h1>Installed Applications</h1>
                <p>All applications managed by Omnipack, Flatpak, and system repositories</p>
              </div>
              <div class="flex gap-2 items-center">
                <select id="installed-filter" class="input" style="width:auto" onchange="filterInstalled(this.value)">
                  <option value="all">All sources</option>
                  <option value="direct">Sideloaded</option>
                  <option value="flatpak">Flatpak</option>
                  <option value="pacman">Pacman</option>
                  <option value="apt">APT</option>
                  <option value="dnf">DNF</option>
                </select>
                <button class="btn btn-secondary btn-sm" onclick="loadInstalledApps()">⟳ Refresh</button>
              </div>
            </div>

            <div id="installed-list">
              <div class="empty-state">
                <div class="empty-state-icon">📦</div>
                <div class="empty-state-title">No apps loaded</div>
                <p class="empty-state-sub">Switch to this tab to load installed applications.</p>
              </div>
            </div>
          </div> <!-- /panel-installed -->


          <!-- ── CLEANER PANEL ── -->
          <div class="panel" id="panel-cleaner">
            <div class="panel-header">
              <div class="panel-header-left">
                <h1>System &amp; Cache</h1>
                <p>Clean download cache and manage Omnipack data</p>
              </div>
            </div>

            <div class="stats-row">
              <div class="stat-card">
                <div class="stat-label">Install Root</div>
                <div class="stat-value" style="font-size:12px;padding-top:6px;font-family:var(--font-mono)">~/.local/share/lpm</div>
                <div class="stat-sub">App installations directory</div>
              </div>
              <div class="stat-card">
                <div class="stat-label">Cache Dir</div>
                <div class="stat-value" style="font-size:12px;padding-top:6px;font-family:var(--font-mono)">~/.cache/lpm</div>
                <div class="stat-sub">Temporary downloads</div>
              </div>
              <div class="stat-card">
                <div class="stat-label">Manifest</div>
                <div class="stat-value" style="font-size:12px;padding-top:6px;font-family:var(--font-mono)">manifest.json</div>
                <div class="stat-sub">App install records</div>
              </div>
            </div>

            <div class="card">
              <div class="card-header">
                <span class="card-header-icon">🗑️</span>
                <h2>Cache Management</h2>
              </div>
              <div class="card-body flex-col gap-3">
                <p style="font-size:13px;color:var(--text-secondary)">
                  Downloaded archives are temporarily stored in <code style="font-family:var(--font-mono);color:var(--indigo-400)">~/.cache/lpm/downloads</code> during installation and automatically removed. Click below to clear any remaining cache files.
                </p>
                <div>
                  <button id="clean-cache-btn" class="btn btn-secondary">🧹 Clear Download Cache</button>
                </div>
              </div>
            </div>

            <div class="card">
              <div class="card-header">
                <span class="card-header-icon">📋</span>
                <h2>How Sideloading Works</h2>
              </div>
              <div class="card-body flex-col gap-3">
                <div style="display:flex;flex-direction:column;gap:10px;">
                  ${[
                    ['📥', 'Download', 'Archive streamed directly from URL to ~/.cache/lpm/downloads/'],
                    ['🗜️', 'Extract',  'Safe extraction with Zip-Slip path traversal protection'],
                    ['🔍', 'Scan',     'debug/elf inspects archive for Linux ELF executables and scripts'],
                    ['🔗', 'Symlink',  'Primary binary symlinked to ~/.local/bin/<name>'],
                    ['🖥️', 'Launcher', '.desktop entry written to ~/.local/share/applications/'],
                    ['📋', 'Manifest', 'All files tracked in manifest.json for 100% clean uninstall'],
                  ].map(([icon, label, desc]) => `
                    <div style="display:flex;align-items:flex-start;gap:12px;">
                      <span style="font-size:18px;flex-shrink:0">${icon}</span>
                      <div>
                        <div style="font-size:12px;font-weight:600;color:var(--text-primary)">${label}</div>
                        <div style="font-size:12px;color:var(--text-muted);font-family:var(--font-mono)">${desc}</div>
                      </div>
                    </div>
                  `).join('')}
                </div>
              </div>
            </div>
          </div> <!-- /panel-cleaner -->

        </div> <!-- /content-panels -->
      </div> <!-- /main-content -->

    </div> <!-- /layout -->

    <!-- Toast Container -->
    <div id="toast-container" class="toast-container"></div>

    <!-- Console Drawer -->
    <div class="console-drawer" id="console-drawer">
      <div class="console-header" id="console-header">
        <div id="console-dot" class="console-dot inactive"></div>
        <span class="console-title">Console Log</span>
        <span class="console-chevron" id="console-chevron">▲</span>
      </div>
      <div class="console-body" id="console-body"></div>
    </div>
  `;
}

// ----------------------------------------------------------------
// Wire up all event listeners after DOM is built
// ----------------------------------------------------------------
function wireEvents() {
  // Sidebar nav
  document.querySelectorAll('.nav-item[data-panel]').forEach(item => {
    item.addEventListener('click', () => navigate(item.dataset.panel));
  });

  // Console drawer toggle
  document.getElementById('console-header').addEventListener('click', () => {
    state.consoleExpanded = !state.consoleExpanded;
    document.getElementById('console-drawer').classList.toggle('expanded', state.consoleExpanded);
  });

  setupSideloadPanel();
  setupSearchPanel();
  setupCleanerPanel();
}

// Expose for inline event handlers
window.filterInstalled = function(val) {
  renderInstalledApps(state.installedApps);
};
window.loadInstalledApps = loadInstalledApps;

// Listen for backend log events if Wails is available
function listenForLogs() {
  if (wailsReady && backend.runtime?.EventsOn) {
    backend.runtime.EventsOn('log:message', msg => {
      addLog(msg, 'info');
    });
  }
}

// ----------------------------------------------------------------
// Bootstrap
// ----------------------------------------------------------------
async function main() {
  buildShell();
  wireEvents();
  await loadBackend();
  listenForLogs();
  await loadSystemInfo();
  addLog('Omnipack ready.', 'success');
}

main();
