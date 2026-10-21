<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Connect,
    Disconnect,
    GetStatus,
    GetCountries,
    SetSelectedCountry,
    SetSystemProxyEnabled,
    PingAllServers
  } from '../wailsjs/go/main/App.js';
  import { EventsOn, EventsOff, BrowserOpenURL } from '../wailsjs/runtime/runtime.js';
  import type { core, remotesettings } from '../wailsjs/go/models';
  import foxyLogo from './assets/images/foxy-logo.png';
  import { getCountryFlagUrl } from './utils/flags';

  import LocationDrawer from './components/LocationDrawer.svelte';
  import SettingsModal from './components/SettingsModal.svelte';
  import AccountModal from './components/AccountModal.svelte';
  import LogsModal from './components/LogsModal.svelte';

  // Live VPN status state
  let status: core.VpnStatus = {
    state: 'DISCONNECTED',
    selected_country: 'REC',
    selected_city: 'REC',
    connected_node: '',
    ping_ms: 0,
    upload_speed_bytes: 0,
    download_speed_bytes: 0,
    total_uploaded_bytes: 0,
    total_downloaded_bytes: 0,
    quota_limit_bytes: 53687091200,
    quota_remaining_bytes: 53687091200,
    quota_reset_epoch: 0,
    is_unlimited: false,
    is_signed_in: false,
    user_email: '',
    tls_split_mode: 1,
    system_proxy_enabled: true,
    socks_port: 1080,
    http_port: 8080,
    socks_bind_address: '127.0.0.1',
    real_ip: '',
    real_country: '',
    exit_ip: '',
    exit_country: '',
    is_protected: false,
    exit_check_enabled: true,
    dns_provider: 'SYSTEM_DEFAULT',
    custom_doh_url: '',
    bootstrap_dns: 'AUTOMATIC',
    custom_bootstrap_ip: '',
    custom_edge_address: '',
    upstream_proxy_enabled: false,
    upstream_proxy_type: 'SOCKS5',
    upstream_proxy_host: '',
    upstream_proxy_port: 1080,
    minimize_to_tray: true,
    close_to_tray: true,
    launch_on_startup: false,
    bypass_lan: true,
    custom_bypass_list: '<local>;192.168.*;10.*;172.16.*'
  } as core.VpnStatus;

  // Countries catalog
  let countries: remotesettings.VpnCountry[] = [];

  // Modal display toggles
  let showLocationModal = false;
  let showAccountModal = false;
  let showSettingsModal = false;
  let showLogsModal = false;

  // Connection timer
  let connectedSeconds = 0;
  let timerInterval: any = null;
  let pollInterval: any = null;
  let isPinging = false;

  onMount(async () => {
    await refreshStatus();
    try {
      countries = await GetCountries();
    } catch (e) {
      console.error('Failed to fetch countries catalog:', e);
    }

    try {
      EventsOn('vpn:status', (newStatus: core.VpnStatus) => {
        if (!newStatus) return;
        const prevState = status.state;
        status = newStatus;

        if (status.state === 'CONNECTED') {
          if (!timerInterval) {
            timerInterval = setInterval(() => {
              connectedSeconds++;
            }, 1000);
          }
        } else {
          if (timerInterval) {
            clearInterval(timerInterval);
            timerInterval = null;
          }
          if (status.state === 'DISCONNECTED') {
            connectedSeconds = 0;
          }
        }
      });
    } catch (e) {
      console.warn('EventsOn binding warning:', e);
    }

    // Backup polling ticker every 1.5s
    pollInterval = setInterval(async () => {
      try {
        const s = await GetStatus();
        if (s) {
          status = s;
          if (status.state === 'CONNECTED' && !timerInterval) {
            timerInterval = setInterval(() => {
              connectedSeconds++;
            }, 1000);
          } else if (status.state !== 'CONNECTED' && timerInterval) {
            clearInterval(timerInterval);
            timerInterval = null;
            connectedSeconds = 0;
          }
        }
      } catch (e) {}
    }, 1500);
  });

  onDestroy(() => {
    try {
      EventsOff('vpn:status');
    } catch (e) {}
    if (timerInterval) clearInterval(timerInterval);
    if (pollInterval) clearInterval(pollInterval);
  });

  async function refreshStatus() {
    try {
      const s = await GetStatus();
      if (s) status = s;
    } catch (e) {
      console.error('Failed to refresh status:', e);
    }
  }

  async function handleToggleConnect() {
    if (status.state === 'CONNECTED') {
      try {
        status.state = 'DISCONNECTING';
        await Disconnect();
      } catch (e) {
        console.error('Disconnect error:', e);
      } finally {
        await refreshStatus();
      }
    } else if (status.state === 'CONNECTING') {
      await handleCancelConnect();
    } else {
      if (!status.is_signed_in) {
        showAccountModal = true;
        return;
      }
      try {
        status.state = 'CONNECTING';
        await Connect();
      } catch (e) {
        console.error('Connect error:', e);
        status.state = 'ERROR';
      }
    }
  }

  async function handleCancelConnect() {
    try {
      status.state = 'DISCONNECTING';
      await Disconnect();
    } catch (e) {
      console.warn('Cancel connect error:', e);
    } finally {
      status.state = 'DISCONNECTED';
      await refreshStatus();
    }
  }

  async function handleSelectLocation(country: string, city: string) {
    try {
      await SetSelectedCountry(country, city);
      status.selected_country = country;
      status.selected_city = city;
      showLocationModal = false;
      if (status.state === 'CONNECTED') {
        await Disconnect();
        await Connect();
      }
    } catch (e) {
      console.error('Failed selecting location:', e);
    }
  }

  function handleOpenGithub() {
    try {
      BrowserOpenURL('https://github.com/meytiii/FoxyVPN-win');
    } catch (e) {
      console.error('Failed to open GitHub repo:', e);
    }
  }

  async function toggleSystemProxy() {
    const nextVal = !status.system_proxy_enabled;
    try {
      await SetSystemProxyEnabled(nextVal);
      status.system_proxy_enabled = nextVal;
    } catch (e) {
      console.error('Failed toggling system proxy:', e);
    }
  }

  async function triggerPingCheck(event: MouseEvent) {
    event.stopPropagation();
    if (isPinging) return;
    isPinging = true;
    try {
      await PingAllServers();
      await refreshStatus();
    } catch (e) {
      console.warn('Ping check failed:', e);
    } finally {
      isPinging = false;
    }
  }

  function formatBytes(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function formatSpeed(bytesPerSec: number): string {
    return formatBytes(bytesPerSec) + '/s';
  }

  function formatDuration(sec: number): string {
    const m = Math.floor(sec / 60);
    const s = sec % 60;
    const h = Math.floor(m / 60);
    const mm = m % 60;
    return `${h.toString().padStart(2, '0')}:${mm.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  }

  $: quotaPercent = status.is_unlimited
    ? 100
    : status.quota_limit_bytes > 0
      ? Math.max(0, Math.min(100, (status.quota_remaining_bytes / status.quota_limit_bytes) * 100))
      : 100;

  function getCountryFlag(code: string): string {
    if (!code || code === 'REC' || code.length !== 2) return '🌐';
    const codePoints = code
      .toUpperCase()
      .split('')
      .map(char => 127397 + char.charCodeAt(0));
    return String.fromCodePoint(...codePoints);
  }

  $: selectedCountryName = (() => {
    if (!status.selected_country || status.selected_country === 'REC') {
      return 'Recommended (Auto)';
    }
    const found = countries.find(c => c.code === status.selected_country);
    return found ? found.name : status.selected_country;
  })();

  $: selectedCityName = (() => {
    if (!status.selected_country || status.selected_country === 'REC' || status.selected_city === 'REC') {
      return 'Fastest Anycast Edge';
    }
    const found = countries.find(c => c.code === status.selected_country);
    if (found && found.cities) {
      const city = found.cities.find(ct => ct.code === status.selected_city);
      if (city) return city.name;
    }
    return status.selected_city.toUpperCase();
  })();
</script>

<div class="app-layout">
  <!-- Top Navigation Header -->
  <header class="app-header">
    <div class="brand">
      <div class="fox-icon-wrapper">
        <img class="fox-icon" src={foxyLogo} alt="FoxyVPN Shield Logo" />
      </div>
      <div class="brand-info">
        <div class="brand-title">
          <span>Foxy</span><span class="gradient-text">VPN</span>
        </div>
        <div class="brand-badge">Mozilla Fastly H2</div>
      </div>
    </div>

    <div class="header-actions">
      <!-- Logs Terminal Modal Trigger -->
      <button 
        type="button" 
        class="icon-btn" 
        on:click={() => showLogsModal = true} 
        title="Live Traffic Logs"
      >
        <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="4 17 10 11 4 5" />
          <line x1="12" y1="19" x2="20" y2="19" />
        </svg>
      </button>

      <!-- Settings Modal Trigger -->
      <button 
        type="button" 
        class="icon-btn" 
        on:click={() => showSettingsModal = true} 
        title="Tunnel Settings"
      >
        <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="3" />
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
        </svg>
      </button>

      <!-- Firefox Account Pill Trigger -->
      <button 
        type="button" 
        class="icon-pill" 
        on:click={() => showAccountModal = true} 
        title="Firefox Account"
      >
        <span class="status-dot" class:active={status.is_signed_in}></span>
        <span class="pill-label">{status.is_signed_in ? (status.user_email ? status.user_email.split('@')[0] : 'FxA Active') : 'Sign In'}</span>
      </button>
    </div>
  </header>

  <!-- Security State & IP Leak Protection Badge -->
  <section class="security-card" class:protected={status.is_protected} class:unprotected={!status.is_protected}>
    <div class="security-left">
      <div class="shield-indicator" class:secured={status.is_protected}>
        {#if status.is_protected}
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4zm-2 16l-4-4 1.41-1.41L10 14.17l6.59-6.59L18 9l-8 8z" />
          </svg>
        {:else}
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
            <line x1="12" y1="9" x2="12" y2="13" />
            <line x1="12" y1="17" x2="12.01" y2="17" />
          </svg>
        {/if}
      </div>
      <div class="security-info">
        <div class="security-headline">
          {status.is_protected ? 'TRAFFIC PROTECTED' : 'UNENCRYPTED CONNECTION'}
        </div>
        <div class="security-ip font-mono">
          {#if status.is_protected}
            Exit IP: <strong>{status.exit_ip || 'Fastly Edge'}</strong>
            {#if status.exit_country}
              <span class="ip-country-badge">{status.exit_country}</span>
            {/if}
          {:else}
            Real IP: <strong>{status.real_ip || 'Exposed'}</strong>
            {#if status.real_country}
              <span class="ip-country-badge alert">{status.real_country}</span>
            {/if}
          {/if}
        </div>
      </div>
    </div>
    {#if status.connected_node}
      <div class="edge-node-badge" title="Active Fastly Edge Node">
        {status.connected_node}
      </div>
    {/if}
  </section>

  <!-- Hero Power Reactor Orb -->
  <main class="reactor-container">
    <div 
      class="orbital-glow" 
      class:connected={status.state === 'CONNECTED'} 
      class:connecting={status.state === 'CONNECTING'}
    ></div>
    
    <button 
      type="button"
      class="power-orb" 
      aria-label="Toggle VPN Connection"
      class:connected={status.state === 'CONNECTED'} 
      class:connecting={status.state === 'CONNECTING' || status.state === 'DISCONNECTING'}
      class:error={status.state === 'ERROR'}
      on:click={handleToggleConnect}
    >
      <div class="power-ring-outer"></div>
      <div class="power-ring-inner">
        <svg class="power-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <path d="M18.36 6.64a9 9 0 1 1-12.73 0" stroke-linecap="round" />
          <line x1="12" y1="2" x2="12" y2="12" stroke-linecap="round" />
        </svg>
      </div>
    </button>

    <div class="reactor-status">
      {#if status.state === 'CONNECTED'}
        <div class="status-badge connected">CONNECTED & TUNNELED</div>
        <div class="duration-counter font-mono">{formatDuration(connectedSeconds)}</div>
      {:else if status.state === 'CONNECTING'}
        <div class="status-badge connecting">ESTABLISHING TUNNEL...</div>
        <div class="status-hint">Solving challenge & negotiating HTTP/2 TLS 1.3</div>
        <button type="button" class="btn-cancel-connect" on:click={handleCancelConnect}>Cancel Connection</button>
      {:else if status.state === 'DISCONNECTING'}
        <div class="status-badge connecting">DISCONNECTING...</div>
        <div class="status-hint">Restoring system proxy & terminating H2 tunnel</div>
      {:else if status.state === 'ERROR'}
        <div class="status-badge error">CONNECTION FAILED</div>
        <div class="status-hint">{status.error_message || 'Tap orb to retry connection'}</div>
      {:else}
        <div class="status-badge disconnected">NOT CONNECTED</div>
        <div class="status-hint">Tap orb to initiate HTTP/2 tunnel</div>
      {/if}
    </div>
  </main>

  <!-- Location Picker Card -->
  <section class="location-section">
    <div 
      class="location-card" 
      role="button" 
      tabindex="0"
      on:click={() => showLocationModal = true}
      on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') showLocationModal = true; }}
    >
      <div class="location-flag">
        {#if !status.selected_country || status.selected_country === 'REC'}
          <span class="flag-globe">🌐</span>
        {:else}
          <div class="flag-container-hero">
            <img 
              class="country-flag-img-hero" 
              src={getCountryFlagUrl(status.selected_country)} 
              alt="{status.selected_country} flag"
              on:error={(e) => {
                const target = e.currentTarget;
                if (target instanceof HTMLElement) target.style.display = 'none';
                const fallback = target.nextElementSibling;
                if (fallback instanceof HTMLElement) fallback.style.display = 'inline-flex';
              }}
            />
            <span class="flag-pill flag-fallback" style="display: none;">{status.selected_country}</span>
          </div>
        {/if}
      </div>
      <div class="location-details">
        <div class="location-title">
          {selectedCountryName}
        </div>
        <div class="location-subtitle">
          {selectedCityName}
        </div>
      </div>
      <div class="location-meta">
        <button 
          type="button" 
          class="ping-chip" 
          class:live={status.ping_ms > 0} 
          class:spinning={isPinging}
          on:click={triggerPingCheck}
          title="Click to test server latencies"
        >
          {#if isPinging}
            <span class="spinner-inline"></span> Pinging...
          {:else if status.selected_country === 'REC'}
            ⚡ Auto-Best
          {:else if status.ping_ms > 0}
            ⚡ {status.ping_ms} ms
          {:else}
            ⚡ Ping
          {/if}
        </button>
        <svg class="chevron-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="9 18 15 12 9 6" />
        </svg>
      </div>
    </div>
  </section>

  <!-- Telemetry Bento Grid -->
  <section class="bento-grid">
    <!-- Live Speeds -->
    <div class="bento-card">
      <div class="bento-header">
        <span class="bento-icon download">↓</span>
        <span class="bento-label">DOWNLOAD</span>
      </div>
      <div class="bento-value font-mono">{formatSpeed(status.download_speed_bytes)}</div>
      <div class="bento-sub font-mono">Total {formatBytes(status.total_downloaded_bytes)}</div>
    </div>

    <div class="bento-card">
      <div class="bento-header">
        <span class="bento-icon upload">↑</span>
        <span class="bento-label">UPLOAD</span>
      </div>
      <div class="bento-value font-mono">{formatSpeed(status.upload_speed_bytes)}</div>
      <div class="bento-sub font-mono">Total {formatBytes(status.total_uploaded_bytes)}</div>
    </div>

    <!-- Bandwidth Quota Progress -->
    <div class="bento-card full-span">
      <div class="quota-header">
        <div class="quota-title">
          <span class="quota-badge">{status.is_unlimited ? 'UNLIMITED' : '50 GB FREE'}</span>
          <span>Monthly Bandwidth</span>
        </div>
        <div class="quota-figures font-mono">
          {#if status.is_unlimited}
            Unlimited Pass Active
          {:else}
            {formatBytes(status.quota_remaining_bytes)} remaining
          {/if}
        </div>
      </div>
      <div class="progress-bar-track">
        <div class="progress-bar-fill" style="width: {quotaPercent}%"></div>
      </div>
    </div>
  </section>

  <!-- Quick Settings Controls -->
  <footer class="quick-controls">
    <button type="button" class="toggle-row" on:click={toggleSystemProxy}>
      <div class="toggle-info">
        <div class="toggle-name">Windows System Proxy (WinINet)</div>
        <div class="toggle-desc">Automatically route Edge, Chrome & Windows apps</div>
      </div>
      <div class="switch" class:active={status.system_proxy_enabled}>
        <div class="switch-handle"></div>
      </div>
    </button>

    <div class="ports-strip">
      <button type="button" class="github-star-pill font-mono" on:click={handleOpenGithub} title="Star FoxyVPN on GitHub">
        ★ Star on GitHub
      </button>
      <span class="port-label">Proxy:</span>
      <span class="port-pill font-mono">SOCKS5 :{status.socks_port}</span>
      <span class="port-pill font-mono">HTTP :{status.http_port}</span>
    </div>
  </footer>

  <!-- Modular Modals -->
  <LocationDrawer 
    show={showLocationModal} 
    {countries} 
    selectedCountryCode={status.selected_country} 
    selectedCityCode={status.selected_city} 
    onClose={() => showLocationModal = false} 
    onSelect={handleSelectLocation} 
  />

  <SettingsModal 
    show={showSettingsModal} 
    onClose={() => showSettingsModal = false} 
  />

  <AccountModal 
    show={showAccountModal} 
    isSignedIn={status.is_signed_in} 
    userEmail={status.user_email || ''} 
    isUnlimited={status.is_unlimited} 
    quotaRemainingBytes={status.quota_remaining_bytes} 
    quotaLimitBytes={status.quota_limit_bytes} 
    onClose={() => showAccountModal = false} 
    onAuthSuccess={refreshStatus} 
    onSignOutSuccess={refreshStatus} 
  />

  <LogsModal 
    show={showLogsModal} 
    onClose={() => showLogsModal = false} 
  />
</div>

<style>
  .app-layout {
    display: flex;
    flex-direction: column;
    height: 100vh;
    padding: 16px 20px;
    gap: 14px;
    background: transparent;
    overflow-y: auto;
    overflow-x: hidden;
  }

  /* App Header */
  .app-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 2px;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .fox-icon-wrapper {
    width: 36px;
    height: 36px;
    border-radius: 10px;
    background: rgba(255, 87, 34, 0.12);
    border: 1px solid rgba(255, 87, 34, 0.25);
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 0 15px rgba(255, 87, 34, 0.2);
  }

  .fox-icon {
    width: 26px;
    height: 26px;
    object-fit: contain;
  }

  .brand-title {
    font-size: 1.15rem;
    font-weight: 800;
    letter-spacing: -0.02em;
    color: var(--text-white);
  }

  .gradient-text {
    background: var(--fox-gradient);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .brand-badge {
    font-size: 0.65rem;
    font-weight: 700;
    color: var(--text-dim);
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .icon-btn {
    width: 34px;
    height: 34px;
    border-radius: 9px;
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    color: var(--text-muted);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .icon-btn:hover {
    background: var(--bg-card-hover);
    color: var(--text-white);
    border-color: rgba(255, 255, 255, 0.18);
  }

  .icon-pill {
    height: 34px;
    padding: 0 12px;
    border-radius: 9px;
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    color: var(--text-muted);
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-size: 0.78rem;
    font-weight: 600;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .icon-pill:hover {
    background: var(--bg-card-hover);
    color: var(--text-white);
    border-color: rgba(255, 255, 255, 0.18);
  }

  .status-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: #64748b;
    box-shadow: 0 0 6px rgba(100, 116, 139, 0.5);
  }

  .status-dot.active {
    background: var(--emerald-active);
    box-shadow: 0 0 8px rgba(16, 185, 129, 0.7);
  }

  /* Security Status Card */
  .security-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-radius: 10px;
    background: rgba(18, 22, 34, 0.65);
    border: 1px solid var(--border-subtle);
    backdrop-filter: blur(12px);
    transition: all 0.3s ease;
  }

  .security-card.protected {
    background: rgba(16, 185, 129, 0.08);
    border-color: rgba(16, 185, 129, 0.25);
  }

  .security-card.unprotected {
    background: rgba(239, 68, 68, 0.06);
    border-color: rgba(239, 68, 68, 0.2);
  }

  .security-left {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .shield-indicator {
    width: 28px;
    height: 28px;
    border-radius: 8px;
    background: rgba(239, 68, 68, 0.15);
    color: #ef4444;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .shield-indicator.secured {
    background: rgba(16, 185, 129, 0.15);
    color: #10b981;
  }

  .security-headline {
    font-size: 0.68rem;
    font-weight: 800;
    letter-spacing: 0.06em;
    color: var(--text-white);
  }

  .security-ip {
    font-size: 0.72rem;
    color: var(--text-muted);
  }

  .ip-country-badge {
    padding: 1px 5px;
    border-radius: 4px;
    background: rgba(16, 185, 129, 0.15);
    color: #10b981;
    font-weight: 700;
    margin-left: 4px;
  }

  .ip-country-badge.alert {
    background: rgba(239, 68, 68, 0.15);
    color: #ef4444;
  }

  .edge-node-badge {
    font-size: 0.65rem;
    padding: 3px 7px;
    border-radius: 6px;
    background: rgba(255, 255, 255, 0.06);
    color: var(--text-dim);
    font-family: var(--font-mono);
  }

  /* Hero Power Reactor */
  .reactor-container {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 24px 0 16px;
  }

  .orbital-glow {
    position: absolute;
    width: 170px;
    height: 170px;
    border-radius: 50%;
    background: radial-gradient(circle, rgba(255, 87, 34, 0.25) 0%, transparent 70%);
    filter: blur(25px);
    pointer-events: none;
    transition: all 0.5s ease;
  }

  .orbital-glow.connected {
    background: radial-gradient(circle, rgba(16, 185, 129, 0.35) 0%, transparent 70%);
  }

  .orbital-glow.connecting {
    background: radial-gradient(circle, rgba(255, 153, 0, 0.35) 0%, transparent 70%);
    animation: glow-pulse 1.6s infinite alternate;
  }

  @keyframes glow-pulse {
    0% { transform: scale(0.9); opacity: 0.6; }
    100% { transform: scale(1.15); opacity: 1; }
  }

  .power-orb {
    position: relative;
    width: 120px;
    height: 120px;
    border-radius: 50%;
    border: none;
    background: #111522;
    cursor: pointer;
    outline: none;
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.6), inset 0 2px 4px rgba(255, 255, 255, 0.1);
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .power-orb:hover:not(:disabled) {
    transform: scale(1.03);
  }

  .power-orb:active:not(:disabled) {
    transform: scale(0.97);
  }

  .power-ring-outer {
    position: absolute;
    inset: -6px;
    border-radius: 50%;
    border: 2px dashed rgba(255, 87, 34, 0.3);
    transition: all 0.4s ease;
  }

  .power-ring-inner {
    width: 90px;
    height: 90px;
    border-radius: 50%;
    background: linear-gradient(135deg, #1b2030 0%, #0d101a 100%);
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid rgba(255, 255, 255, 0.08);
    transition: all 0.4s ease;
  }

  .power-icon {
    width: 38px;
    height: 38px;
    color: #ff5722;
    transition: all 0.4s ease;
  }

  /* Orb connected state */
  .power-orb.connected .power-ring-outer {
    border-color: rgba(16, 185, 129, 0.5);
    border-style: solid;
    box-shadow: 0 0 20px rgba(16, 185, 129, 0.35);
  }

  .power-orb.connected .power-ring-inner {
    background: var(--emerald-gradient);
    border-color: rgba(255, 255, 255, 0.3);
  }

  .power-orb.connected .power-icon {
    color: #ffffff;
    filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.3));
  }

  /* Orb connecting state */
  .power-orb.connecting .power-ring-outer {
    border-color: rgba(255, 153, 0, 0.7);
    animation: rotate-ring 1.8s linear infinite;
  }

  .power-orb.connecting .power-icon {
    color: #ff9800;
  }

  @keyframes rotate-ring {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
  }

  /* Orb error state */
  .power-orb.error .power-ring-outer {
    border-color: rgba(239, 68, 68, 0.6);
  }

  .power-orb.error .power-icon {
    color: #ef4444;
  }

  .reactor-status {
    margin-top: 14px;
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .status-badge {
    font-size: 0.76rem;
    font-weight: 800;
    letter-spacing: 0.08em;
    padding: 3px 12px;
    border-radius: 999px;
  }

  .status-badge.connected {
    background: rgba(16, 185, 129, 0.15);
    color: #10b981;
    border: 1px solid rgba(16, 185, 129, 0.3);
  }

  .status-badge.connecting {
    background: rgba(255, 152, 0, 0.15);
    color: #ff9800;
    border: 1px solid rgba(255, 152, 0, 0.3);
  }

  .status-badge.disconnected {
    background: rgba(255, 255, 255, 0.06);
    color: var(--text-muted);
    border: 1px solid var(--border-subtle);
  }

  .status-badge.error {
    background: rgba(239, 68, 68, 0.15);
    color: #ef4444;
    border: 1px solid rgba(239, 68, 68, 0.3);
  }

  .duration-counter {
    font-size: 1.15rem;
    font-weight: 700;
    color: var(--text-white);
    letter-spacing: 0.02em;
  }

  .status-hint {
    font-size: 0.74rem;
    color: var(--text-dim);
  }

  /* Location Card */
  .location-section {
    width: 100%;
  }

  .location-card {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    border-radius: 12px;
    cursor: pointer;
    text-align: left;
    transition: all 0.2s ease;
  }

  .location-card:hover {
    background: var(--bg-card-hover);
    border-color: rgba(255, 255, 255, 0.18);
  }

  .location-flag {
    font-size: 1.45rem;
    line-height: 1;
  }

  .location-details {
    flex: 1;
    min-width: 0;
  }

  .location-title {
    font-size: 0.88rem;
    font-weight: 700;
    color: var(--text-white);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .location-subtitle {
    font-size: 0.72rem;
    color: var(--text-muted);
  }

  .location-meta {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .ping-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 0.72rem;
    font-weight: 600;
    padding: 3px 8px;
    border-radius: 6px;
    background: rgba(255, 255, 255, 0.06);
    color: var(--text-muted);
    border: 1px solid var(--border-subtle);
    cursor: pointer;
    font-family: var(--font-mono);
    transition: all 0.2s ease;
  }

  .ping-chip:hover {
    background: rgba(255, 255, 255, 0.1);
    color: var(--text-white);
  }

  .ping-chip.live {
    background: rgba(16, 185, 129, 0.12);
    color: #10b981;
    border-color: rgba(16, 185, 129, 0.3);
  }

  .spinner-inline {
    width: 10px;
    height: 10px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: #ff5722;
    border-radius: 50%;
    animation: rotate-ring 0.8s linear infinite;
  }

  .chevron-icon {
    color: var(--text-dim);
  }

  /* Bento Grid */
  .bento-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }

  .bento-card {
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    border-radius: 12px;
    padding: 10px 12px;
  }

  .bento-card.full-span {
    grid-column: span 2;
  }

  .bento-header {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 4px;
  }

  .bento-icon {
    font-size: 0.82rem;
    font-weight: 800;
  }

  .bento-icon.download { color: #38bdf8; }
  .bento-icon.upload { color: #f43f5e; }

  .bento-label {
    font-size: 0.65rem;
    font-weight: 700;
    color: var(--text-dim);
    letter-spacing: 0.05em;
  }

  .bento-value {
    font-size: 1.05rem;
    font-weight: 700;
    color: var(--text-white);
  }

  .bento-sub {
    font-size: 0.68rem;
    color: var(--text-muted);
    margin-top: 2px;
  }

  /* Quota Card */
  .quota-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .quota-title {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.74rem;
    font-weight: 600;
    color: var(--text-muted);
  }

  .quota-badge {
    font-size: 0.65rem;
    font-weight: 800;
    padding: 2px 6px;
    border-radius: 4px;
    background: rgba(255, 87, 34, 0.15);
    color: #ff5722;
  }

  .quota-figures {
    font-size: 0.72rem;
    color: var(--text-white);
    font-weight: 600;
  }

  .progress-bar-track {
    width: 100%;
    height: 6px;
    border-radius: 3px;
    background: rgba(255, 255, 255, 0.06);
    overflow: hidden;
  }

  .progress-bar-fill {
    height: 100%;
    border-radius: 3px;
    background: var(--fox-gradient);
    transition: width 0.4s ease;
  }

  /* Quick Controls */
  .quick-controls {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-top: auto;
    padding-top: 4px;
  }

  .toggle-row {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    border-radius: 12px;
    cursor: pointer;
    text-align: left;
    transition: all 0.2s ease;
  }

  .toggle-row:hover {
    background: var(--bg-card-hover);
  }

  .toggle-name {
    font-size: 0.82rem;
    font-weight: 700;
    color: var(--text-white);
  }

  .toggle-desc {
    font-size: 0.68rem;
    color: var(--text-muted);
  }

  .switch {
    width: 38px;
    height: 22px;
    border-radius: 11px;
    background: rgba(255, 255, 255, 0.12);
    position: relative;
    transition: background 0.25s ease;
    flex-shrink: 0;
  }

  .switch.active {
    background: #10b981;
  }

  .switch-handle {
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: #ffffff;
    position: absolute;
    top: 3px;
    left: 3px;
    transition: transform 0.25s ease;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
  }

  .switch.active .switch-handle {
    transform: translateX(16px);
  }

  .ports-strip {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    font-size: 0.68rem;
    color: var(--text-dim);
  }

  .port-pill {
    padding: 2px 7px;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid var(--border-subtle);
    color: var(--text-muted);
  }

  .github-star-pill {
    background: rgba(255, 193, 7, 0.1);
    border: 1px solid rgba(255, 193, 7, 0.25);
    color: #ffd166;
    font-size: 0.68rem;
    font-weight: 700;
    padding: 2px 8px;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.2s;
  }

  .github-star-pill:hover {
    background: rgba(255, 193, 7, 0.2);
    border-color: rgba(255, 193, 7, 0.45);
    color: #ffffff;
  }

  .flag-container-hero {
    width: 32px;
    height: 24px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .country-flag-img-hero {
    width: 30px;
    height: 21px;
    object-fit: cover;
    border-radius: 4px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.4), inset 0 0 0 1px rgba(255, 255, 255, 0.15);
    background: #141724;
    flex-shrink: 0;
  }

  .flag-globe {
    font-size: 1.35rem;
    line-height: 1;
  }

  .flag-pill {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 0.72rem;
    font-weight: 800;
    font-family: var(--font-mono);
    padding: 3px 6px;
    border-radius: 6px;
    background: rgba(255, 87, 34, 0.15);
    border: 1px solid rgba(255, 87, 34, 0.35);
    color: #ff5722;
    letter-spacing: 0.05em;
  }

  .btn-cancel-connect {
    margin-top: 6px;
    background: rgba(239, 68, 68, 0.15);
    border: 1px solid rgba(239, 68, 68, 0.35);
    color: #ef4444;
    padding: 3px 12px;
    border-radius: 999px;
    font-size: 0.72rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }

  .btn-cancel-connect:hover {
    background: rgba(239, 68, 68, 0.28);
    color: white;
  }

  /* Responsive compact scaling for minimum window height (640px) and High-DPI displays */
  @media (max-height: 720px) {
    .app-layout {
      padding: 10px 16px;
      gap: 10px;
    }
    .reactor-container {
      padding: 12px 0 8px;
    }
    .power-orb {
      width: 96px;
      height: 96px;
    }
    .power-ring-inner {
      width: 72px;
      height: 72px;
    }
    .power-icon {
      width: 30px;
      height: 30px;
    }
    .orbital-glow {
      width: 130px;
      height: 130px;
    }
    .bento-card {
      padding: 8px 10px;
    }
    .location-card {
      padding: 8px 12px;
    }
    .toggle-row {
      padding: 8px 12px;
    }
  }
</style>
