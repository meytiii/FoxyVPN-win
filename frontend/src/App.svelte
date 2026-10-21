<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Connect,
    Disconnect,
    GetStatus,
    GetCountries,
    SetSelectedCountry,
    SetTlsSplitMode,
    SetSystemProxyEnabled,
    FxaLogin,
    FxaSubmit2FACode,
    FxaSignOut
  } from '../wailsjs/go/main/App.js';
  import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime.js';
  import type { core, remotesettings } from '../wailsjs/go/models';

  // Live VPN state
  let status: core.VpnStatus = {
    state: 'DISCONNECTED',
    selected_country: 'US',
    selected_city: 'nyc',
    connected_node: '',
    ping_ms: 0,
    upload_speed_bytes: 0,
    download_speed_bytes: 0,
    total_uploaded_bytes: 0,
    total_downloaded_bytes: 0,
    quota_limit_bytes: 53687091200,
    quota_remaining_bytes: 53687091200,
    quota_reset_epoch: 0,
    is_signed_in: false,
    tls_split_mode: 1,
    system_proxy_enabled: true,
    socks_port: 1080,
    http_port: 8080
  } as core.VpnStatus;

  // Countries
  let countries: remotesettings.VpnCountry[] = [];
  let countrySearch = '';

  // Modals
  let showLocationModal = false;
  let showAccountModal = false;
  let showSettingsModal = false;

  // Auth Form
  let authEmail = '';
  let authPassword = '';
  let auth2FACode = '';
  let authNeeds2FA = false;
  let authLoading = false;
  let authError = '';

  // Timer for connected duration
  let connectedSeconds = 0;
  let timerInterval: any = null;

  onMount(async () => {
    try {
      const initStatus = await GetStatus();
      if (initStatus) status = initStatus;
    } catch (e) {
      console.error('Failed to get initial status:', e);
    }

    try {
      countries = await GetCountries();
    } catch (e) {
      console.error('Failed to fetch countries:', e);
    }

    EventsOn('vpn:status', (newStatus: core.VpnStatus) => {
      status = newStatus;
      if (status.state === 'CONNECTED' && !timerInterval) {
        timerInterval = setInterval(() => {
          connectedSeconds++;
        }, 1000);
      } else if (status.state !== 'CONNECTED') {
        if (timerInterval) {
          clearInterval(timerInterval);
          timerInterval = null;
        }
        connectedSeconds = 0;
      }
    });
  });

  onDestroy(() => {
    EventsOff('vpn:status');
    if (timerInterval) clearInterval(timerInterval);
  });

  async function handleToggleConnect() {
    if (status.state === 'CONNECTED') {
      try {
        await Disconnect();
      } catch (e) {
        console.error('Disconnect error:', e);
      }
    } else if (status.state === 'DISCONNECTED' || status.state === 'ERROR') {
      if (!status.is_signed_in) {
        showAccountModal = true;
        authError = 'Please sign in to Firefox Accounts first to obtain free VPN pass.';
        return;
      }
      try {
        await Connect();
      } catch (e) {
        console.error('Connect error:', e);
      }
    }
  }

  async function handleSelectCountry(country: remotesettings.VpnCountry) {
    const city = country.cities && country.cities.length > 0 ? country.cities[0].code : '';
    await SetSelectedCountry(country.code, city);
    status.selected_country = country.code;
    status.selected_city = city;
    showLocationModal = false;
    if (status.state === 'CONNECTED') {
      await Disconnect();
      await Connect();
    }
  }

  async function handleFxaSubmit() {
    authError = '';
    authLoading = true;
    try {
      if (authNeeds2FA) {
        await FxaSubmit2FACode(auth2FACode);
        authNeeds2FA = false;
        auth2FACode = '';
        showAccountModal = false;
      } else {
        const needs2FA = await FxaLogin(authEmail, authPassword);
        if (needs2FA) {
          authNeeds2FA = true;
        } else {
          showAccountModal = false;
          authPassword = '';
        }
      }
    } catch (e: any) {
      authError = e?.message || String(e);
    } finally {
      authLoading = false;
    }
  }

  async function handleSignOut() {
    try {
      await FxaSignOut();
      status.is_signed_in = false;
      showAccountModal = false;
    } catch (e) {
      console.error('Sign out error:', e);
    }
  }

  async function toggleSystemProxy() {
    const nextVal = !status.system_proxy_enabled;
    await SetSystemProxyEnabled(nextVal);
    status.system_proxy_enabled = nextVal;
  }

  async function changeTlsSplit(mode: number) {
    await SetTlsSplitMode(mode);
    status.tls_split_mode = mode;
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

  $: quotaPercent = status.quota_limit_bytes > 0
    ? Math.max(0, Math.min(100, (status.quota_remaining_bytes / status.quota_limit_bytes) * 100))
    : 100;

  $: filteredCountries = countries.filter(c => 
    c.name.toLowerCase().includes(countrySearch.toLowerCase()) || 
    c.code.toLowerCase().includes(countrySearch.toLowerCase())
  );

  function getCountryFlag(code: string): string {
    if (!code || code === 'AUTO') return '🌐';
    const codePoints = code
      .toUpperCase()
      .split('')
      .map(char => 127397 + char.charCodeAt(0));
    return String.fromCodePoint(...codePoints);
  }
</script>

<div class="app-layout">
  <!-- Top Navigation Header -->
  <header class="app-header">
    <div class="brand">
      <div class="fox-icon-wrapper">
        <svg class="fox-icon" viewBox="0 0 32 32" fill="none">
          <path d="M16 2L20 8L28 7L24 15L28 24L19 23L16 30L13 23L4 24L8 15L4 7L12 8L16 2Z" fill="url(#fox-grad)" />
          <path d="M16 8L18 12L23 11L20 16L23 21L18 20L16 25L14 20L9 21L12 16L9 11L14 12L16 8Z" fill="#121520" />
          <defs>
            <linearGradient id="fox-grad" x1="4" y1="2" x2="28" y2="30" gradientUnits="userSpaceOnUse">
              <stop stop-color="#FF7A00" />
              <stop offset="0.5" stop-color="#FF3D00" />
              <stop offset="1" stop-color="#D5006D" />
            </linearGradient>
          </defs>
        </svg>
      </div>
      <div class="brand-info">
        <div class="brand-title">
          <span>Foxy</span><span class="gradient-text">VPN</span>
        </div>
        <div class="brand-badge">Mozilla Fastly H2</div>
      </div>
    </div>

    <div class="header-actions">
      <button class="icon-pill" on:click={() => showAccountModal = true} title="Firefox Account">
        <span class="status-dot" class:active={status.is_signed_in}></span>
        <span class="pill-label">{status.is_signed_in ? 'FxA Active' : 'Sign In'}</span>
      </button>

      <button class="icon-btn" on:click={() => showSettingsModal = true} title="Settings">
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="3" />
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
        </svg>
      </button>
    </div>
  </header>

  <!-- Hero Power Reactor -->
  <main class="reactor-container">
    <div class="orbital-glow" class:connected={status.state === 'CONNECTED'} class:connecting={status.state === 'CONNECTING'}></div>
    
    <button 
      class="power-orb" 
      class:connected={status.state === 'CONNECTED'} 
      class:connecting={status.state === 'CONNECTING'}
      class:error={status.state === 'ERROR'}
      on:click={handleToggleConnect}
      disabled={status.state === 'CONNECTING'}
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
        <div class="status-badge connected">PROTECTED & SECURED</div>
        <div class="duration-counter">{formatDuration(connectedSeconds)}</div>
      {:else if status.state === 'CONNECTING'}
        <div class="status-badge connecting">ESTABLISHING TUNNEL...</div>
        <div class="status-hint">Solving challenges & negotiating H2 multiplexing</div>
      {:else if status.state === 'ERROR'}
        <div class="status-badge error">CONNECTION FAILED</div>
        <div class="status-hint">{status.error_message || 'Tap to retry connection'}</div>
      {:else}
        <div class="status-badge disconnected">NOT CONNECTED</div>
        <div class="status-hint">Tap orb to initiate HTTP/2 tunnel</div>
      {/if}
    </div>
  </main>

  <!-- Location Picker Pill -->
  <section class="location-section">
    <button type="button" class="location-card" on:click={() => showLocationModal = true}>
      <div class="location-flag">{getCountryFlag(status.selected_country)}</div>
      <div class="location-details">
        <div class="location-title">
          {status.selected_country} - {status.selected_city.toUpperCase()}
        </div>
        <div class="location-subtitle">Fastly Edge Proxy</div>
      </div>
      <div class="location-meta">
        <span class="ping-chip" class:live={status.ping_ms > 0}>
          ⚡ {status.ping_ms > 0 ? `${status.ping_ms} ms` : 'Standby'}
        </span>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="9 18 15 12 9 6" />
        </svg>
      </div>
    </button>
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

    <!-- Free Quota Progress -->
    <div class="bento-card full-span">
      <div class="quota-header">
        <div class="quota-title">
          <span class="quota-badge">50 GB FREE</span>
          <span>Monthly Bandwidth</span>
        </div>
        <div class="quota-figures font-mono">
          {formatBytes(status.quota_remaining_bytes)} left
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
        <div class="toggle-name">Windows System Proxy</div>
        <div class="toggle-desc">WinINet route all browsers & applications</div>
      </div>
      <div class="switch" class:active={status.system_proxy_enabled}>
        <div class="switch-handle"></div>
      </div>
    </button>
  </footer>

  <!-- LOCATION SELECTION MODAL -->
  {#if showLocationModal}
    <div 
      class="modal-backdrop" 
      role="dialog" 
      aria-modal="true" 
      tabindex="-1"
      on:click={() => showLocationModal = false}
      on:keydown={(e) => { if (e.key === 'Escape') showLocationModal = false; }}
    >
      <div 
        class="modal-card" 
        role="document"
        tabindex="-1"
        on:click|stopPropagation 
        on:keydown|stopPropagation
      >
        <div class="modal-header">
          <h3>Select Exit Location</h3>
          <button class="close-btn" on:click={() => showLocationModal = false}>✕</button>
        </div>

        <div class="search-box">
          <input 
            type="text" 
            placeholder="Search countries or cities..." 
            bind:value={countrySearch}
          />
        </div>

        <div class="country-list">
          {#each filteredCountries as c}
            <button 
              type="button"
              class="country-item" 
              class:selected={c.code === status.selected_country}
              on:click={() => handleSelectCountry(c)}
            >
              <div class="item-flag">{getCountryFlag(c.code)}</div>
              <div class="item-info">
                <div class="item-name">{c.name}</div>
                <div class="item-code">{c.cities?.length || 1} edge location(s)</div>
              </div>
              {#if c.code === status.selected_country}
                <div class="selected-indicator">✓</div>
              {/if}
            </button>
          {/each}
        </div>
      </div>
    </div>
  {/if}

  <!-- FIREFOX ACCOUNT MODAL -->
  {#if showAccountModal}
    <div 
      class="modal-backdrop" 
      role="dialog" 
      aria-modal="true" 
      tabindex="-1"
      on:click={() => showAccountModal = false}
      on:keydown={(e) => { if (e.key === 'Escape') showAccountModal = false; }}
    >
      <div 
        class="modal-card" 
        role="document"
        tabindex="-1"
        on:click|stopPropagation 
        on:keydown|stopPropagation
      >
        <div class="modal-header">
          <h3>Firefox Accounts</h3>
          <button class="close-btn" on:click={() => showAccountModal = false}>✕</button>
        </div>

        {#if status.is_signed_in}
          <div class="account-active-view">
            <div class="account-badge-box">
              <div class="account-avatar">🦊</div>
              <div class="account-active-title">Authenticated</div>
              <div class="account-active-sub">Free 50GB monthly proxy pass active</div>
            </div>
            <button class="danger-btn" on:click={handleSignOut}>Sign Out</button>
          </div>
        {:else}
          <form on:submit|preventDefault={handleFxaSubmit} class="auth-form">
            <p class="auth-intro">
              Log in with your Firefox Account to activate your monthly 50 GB free proxy quota.
            </p>

            {#if authError}
              <div class="alert-error">{authError}</div>
            {/if}

            {#if !authNeeds2FA}
              <div class="form-group">
                <label for="fxa-email">Firefox Account Email</label>
                <input 
                  id="fxa-email"
                  type="email" 
                  bind:value={authEmail} 
                  required 
                  placeholder="name@example.com"
                  disabled={authLoading}
                />
              </div>

              <div class="form-group">
                <label for="fxa-password">Password</label>
                <input 
                  id="fxa-password"
                  type="password" 
                  bind:value={authPassword} 
                  required 
                  placeholder="••••••••••••"
                  disabled={authLoading}
                />
              </div>

              <button class="primary-btn" type="submit" disabled={authLoading}>
                {authLoading ? 'Solving Fastly Challenge...' : 'Sign In'}
              </button>
            {:else}
              <div class="form-group">
                <label for="fxa-2fa">Two-Factor / Email Code</label>
                <input 
                  id="fxa-2fa"
                  type="text" 
                  bind:value={auth2FACode} 
                  required 
                  placeholder="6-digit code"
                  class="font-mono text-center text-lg"
                  disabled={authLoading}
                  autofocus
                />
              </div>

              <button class="primary-btn" type="submit" disabled={authLoading}>
                {authLoading ? 'Verifying Code...' : 'Confirm 2FA Code'}
              </button>
            {/if}
          </form>
        {/if}
      </div>
    </div>
  {/if}

  <!-- SETTINGS MODAL -->
  {#if showSettingsModal}
    <div 
      class="modal-backdrop" 
      role="dialog" 
      aria-modal="true" 
      tabindex="-1"
      on:click={() => showSettingsModal = false}
      on:keydown={(e) => { if (e.key === 'Escape') showSettingsModal = false; }}
    >
      <div 
        class="modal-card" 
        role="document"
        tabindex="-1"
        on:click|stopPropagation 
        on:keydown|stopPropagation
      >
        <div class="modal-header">
          <h3>Tunnel Settings</h3>
          <button class="close-btn" on:click={() => showSettingsModal = false}>✕</button>
        </div>

        <div class="settings-content">
          <div class="setting-item">
            <div class="setting-title">DPI TLS ClientHello Splitting</div>
            <div class="setting-desc">Circumvents deep packet inspection blocking</div>
            <div class="pill-group">
              <button 
                class="pill-opt" 
                class:active={status.tls_split_mode === 0}
                on:click={() => changeTlsSplit(0)}
              >Off</button>
              <button 
                class="pill-opt" 
                class:active={status.tls_split_mode === 1}
                on:click={() => changeTlsSplit(1)}
              >1st Byte</button>
              <button 
                class="pill-opt" 
                class:active={status.tls_split_mode === 2}
                on:click={() => changeTlsSplit(2)}
              >SNI Record</button>
            </div>
          </div>

          <div class="setting-item">
            <div class="setting-title">Local Proxy Ports</div>
            <div class="port-specs font-mono">
              <div>SOCKS5 Proxy: <span>127.0.0.1:{status.socks_port}</span></div>
              <div>HTTP/S Proxy: <span>127.0.0.1:{status.http_port}</span></div>
            </div>
          </div>

          <div class="setting-item">
            <div class="setting-title">Protocol</div>
            <div class="setting-desc font-mono">HTTP/2 CONNECT over TLS 1.3 / Fastly</div>
          </div>
        </div>

        <button class="primary-btn" on:click={() => showSettingsModal = false}>Done</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .app-layout {
    display: flex;
    flex-direction: column;
    height: 100vh;
    padding: 16px 20px;
    gap: 16px;
    background: transparent;
  }

  /* Header */
  .app-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .fox-icon-wrapper {
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(255, 87, 34, 0.1);
    border: 1px solid var(--border-active);
    border-radius: 10px;
  }

  .fox-icon {
    width: 24px;
    height: 24px;
  }

  .brand-title {
    font-size: 16px;
    font-weight: 800;
    letter-spacing: -0.02em;
  }

  .gradient-text {
    background: var(--fox-gradient);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .brand-badge {
    font-size: 10px;
    color: var(--text-dim);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .icon-pill {
    display: flex;
    align-items: center;
    gap: 6px;
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    padding: 6px 12px;
    border-radius: 20px;
    color: var(--text-white);
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }

  .icon-pill:hover {
    border-color: rgba(255, 255, 255, 0.2);
    background: var(--bg-card-hover);
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #64748b;
  }

  .status-dot.active {
    background: var(--emerald-active);
    box-shadow: 0 0 8px var(--emerald-active);
  }

  .icon-btn {
    width: 34px;
    height: 34px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    color: var(--text-muted);
    cursor: pointer;
    transition: all 0.2s;
  }

  .icon-btn:hover {
    color: var(--text-white);
    border-color: rgba(255, 255, 255, 0.2);
  }

  /* Power Reactor Hero */
  .reactor-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 20px 0;
    position: relative;
  }

  .orbital-glow {
    position: absolute;
    width: 220px;
    height: 220px;
    border-radius: 50%;
    background: radial-gradient(circle, rgba(255, 87, 34, 0.2) 0%, transparent 70%);
    pointer-events: none;
    transition: all 0.5s ease;
  }

  .orbital-glow.connected {
    background: radial-gradient(circle, rgba(16, 185, 129, 0.25) 0%, transparent 70%);
  }

  .orbital-glow.connecting {
    background: radial-gradient(circle, rgba(255, 145, 0, 0.3) 0%, transparent 70%);
    animation: pulse 1.5s infinite alternate;
  }

  @keyframes pulse {
    0% { transform: scale(0.9); opacity: 0.5; }
    100% { transform: scale(1.15); opacity: 0.9; }
  }

  .power-orb {
    width: 140px;
    height: 140px;
    border-radius: 50%;
    background: #10131d;
    border: 2px solid var(--border-subtle);
    cursor: pointer;
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
    transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .power-orb:hover {
    transform: scale(1.03);
    border-color: rgba(255, 255, 255, 0.25);
  }

  .power-orb:active {
    transform: scale(0.97);
  }

  .power-orb.connected {
    border-color: var(--border-success);
    box-shadow: var(--emerald-glow);
  }

  .power-orb.connecting {
    border-color: var(--border-active);
    box-shadow: var(--fox-glow);
  }

  .power-orb.error {
    border-color: var(--crimson-err);
    box-shadow: 0 0 30px rgba(239, 68, 68, 0.35);
  }

  .power-ring-inner {
    width: 90px;
    height: 90px;
    border-radius: 50%;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.05);
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.3s;
  }

  .power-orb.connected .power-ring-inner {
    background: var(--emerald-gradient);
  }

  .power-orb.connecting .power-ring-inner {
    background: var(--fox-gradient);
    animation: spin 3s linear infinite;
  }

  @keyframes spin {
    100% { transform: rotate(360deg); }
  }

  .power-icon {
    width: 36px;
    height: 36px;
    color: var(--text-dim);
    transition: color 0.3s;
  }

  .power-orb.connected .power-icon {
    color: #ffffff;
  }

  .power-orb.connecting .power-icon {
    color: #ffffff;
  }

  .reactor-status {
    margin-top: 18px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .status-badge {
    font-size: 13px;
    font-weight: 800;
    letter-spacing: 0.06em;
  }

  .status-badge.connected {
    color: var(--emerald-active);
  }

  .status-badge.connecting {
    color: var(--fox-accent);
  }

  .status-badge.disconnected {
    color: var(--text-dim);
  }

  .status-badge.error {
    color: var(--crimson-err);
  }

  .duration-counter {
    font-family: var(--font-mono);
    font-size: 14px;
    font-weight: 600;
    color: var(--text-white);
  }

  .status-hint {
    font-size: 11px;
    color: var(--text-dim);
    text-align: center;
    max-width: 260px;
  }

  /* Location Card */
  .location-card {
    display: flex;
    width: 100%;
    text-align: left;
    font-family: inherit;
    align-items: center;
    padding: 12px 14px;
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    border-radius: 14px;
    cursor: pointer;
    transition: all 0.2s;
  }

  .location-card:hover {
    background: var(--bg-card-hover);
    border-color: rgba(255, 255, 255, 0.15);
  }

  .location-flag {
    font-size: 24px;
    margin-right: 12px;
  }

  .location-details {
    flex: 1;
  }

  .location-title {
    font-size: 14px;
    font-weight: 700;
    color: var(--text-white);
  }

  .location-subtitle {
    font-size: 11px;
    color: var(--text-dim);
  }

  .location-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--text-dim);
  }

  .ping-chip {
    font-family: var(--font-mono);
    font-size: 11px;
    padding: 2px 8px;
    background: rgba(255, 255, 255, 0.05);
    border-radius: 6px;
  }

  .ping-chip.live {
    color: var(--emerald-active);
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
    padding: 12px 14px;
  }

  .bento-card.full-span {
    grid-column: span 2;
  }

  .bento-header {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 6px;
  }

  .bento-icon {
    font-weight: bold;
    font-size: 12px;
  }

  .bento-icon.download {
    color: #38bdf8;
  }

  .bento-icon.upload {
    color: #f43f5e;
  }

  .bento-label {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.05em;
    color: var(--text-dim);
  }

  .bento-value {
    font-size: 15px;
    font-weight: 700;
    color: var(--text-white);
  }

  .bento-sub {
    font-size: 11px;
    color: var(--text-dim);
    margin-top: 2px;
  }

  /* Quota Bar */
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
    font-size: 12px;
    font-weight: 600;
  }

  .quota-badge {
    font-size: 9px;
    padding: 2px 6px;
    background: rgba(255, 87, 34, 0.15);
    color: var(--fox-primary);
    border-radius: 4px;
    font-weight: 800;
  }

  .quota-figures {
    font-size: 11px;
    color: var(--text-muted);
  }

  .progress-bar-track {
    width: 100%;
    height: 6px;
    background: rgba(255, 255, 255, 0.08);
    border-radius: 3px;
    overflow: hidden;
  }

  .progress-bar-fill {
    height: 100%;
    background: var(--fox-gradient);
    border-radius: 3px;
    transition: width 0.3s;
  }

  /* Quick Controls */
  .quick-controls {
    margin-top: auto;
  }

  .toggle-row {
    display: flex;
    width: 100%;
    text-align: left;
    font-family: inherit;
    color: inherit;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    border-radius: 12px;
    cursor: pointer;
  }

  .toggle-name {
    font-size: 13px;
    font-weight: 600;
  }

  .toggle-desc {
    font-size: 11px;
    color: var(--text-dim);
  }

  .switch {
    width: 40px;
    height: 22px;
    border-radius: 11px;
    background: rgba(255, 255, 255, 0.1);
    position: relative;
    transition: background 0.2s;
  }

  .switch.active {
    background: var(--emerald-active);
  }

  .switch-handle {
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #ffffff;
    position: absolute;
    top: 2px;
    left: 2px;
    transition: transform 0.2s;
  }

  .switch.active .switch-handle {
    transform: translateX(18px);
  }

  /* Modals */
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(0, 0, 0, 0.7);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: flex-end;
    justify-content: center;
    z-index: 100;
  }

  .modal-card {
    width: 100%;
    max-height: 85vh;
    background: #11141f;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 20px 20px 0 0;
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    animation: slideUp 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  }

  @keyframes slideUp {
    from { transform: translateY(100%); }
    to { transform: translateY(0); }
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .modal-header h3 {
    font-size: 16px;
    font-weight: 700;
  }

  .close-btn {
    background: transparent;
    border: none;
    color: var(--text-dim);
    font-size: 16px;
    cursor: pointer;
    padding: 4px;
  }

  .search-box input {
    width: 100%;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    padding: 10px 12px;
    color: var(--text-white);
    outline: none;
  }

  .search-box input:focus {
    border-color: var(--fox-primary);
  }

  .country-list {
    overflow-y: auto;
    max-height: 50vh;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .country-item {
    display: flex;
    width: 100%;
    text-align: left;
    font-family: inherit;
    color: inherit;
    background: transparent;
    border: 1px solid transparent;
    align-items: center;
    padding: 10px 12px;
    border-radius: 10px;
    cursor: pointer;
    transition: background 0.15s;
  }

  .country-item:hover {
    background: rgba(255, 255, 255, 0.05);
  }

  .country-item.selected {
    background: rgba(255, 87, 34, 0.12);
    border: 1px solid var(--border-active);
  }

  .item-flag {
    font-size: 20px;
    margin-right: 12px;
  }

  .item-info {
    flex: 1;
  }

  .item-name {
    font-size: 13px;
    font-weight: 600;
  }

  .item-code {
    font-size: 11px;
    color: var(--text-dim);
  }

  .selected-indicator {
    color: var(--fox-primary);
    font-weight: bold;
  }

  /* Forms & Buttons */
  .auth-form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .auth-intro {
    font-size: 12px;
    color: var(--text-muted);
    line-height: 1.5;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .form-group label {
    font-size: 11px;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .form-group input {
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    padding: 10px 12px;
    color: #ffffff;
    font-size: 14px;
    outline: none;
  }

  .form-group input:focus {
    border-color: var(--fox-primary);
  }

  .primary-btn {
    background: var(--fox-gradient);
    color: #ffffff;
    border: none;
    border-radius: 10px;
    padding: 12px;
    font-size: 14px;
    font-weight: 700;
    cursor: pointer;
    transition: opacity 0.2s;
  }

  .primary-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .danger-btn {
    background: rgba(239, 68, 68, 0.15);
    border: 1px solid rgba(239, 68, 68, 0.4);
    color: var(--crimson-err);
    border-radius: 10px;
    padding: 12px;
    font-size: 14px;
    font-weight: 700;
    cursor: pointer;
  }

  .alert-error {
    background: rgba(239, 68, 68, 0.15);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #fca5a5;
    font-size: 12px;
    padding: 10px;
    border-radius: 8px;
  }

  .account-active-view {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .account-badge-box {
    text-align: center;
    padding: 20px;
    background: rgba(255, 255, 255, 0.03);
    border-radius: 14px;
  }

  .account-avatar {
    font-size: 40px;
    margin-bottom: 8px;
  }

  .account-active-title {
    font-size: 16px;
    font-weight: 700;
  }

  .account-active-sub {
    font-size: 12px;
    color: var(--text-dim);
    margin-top: 4px;
  }

  /* Settings */
  .settings-content {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .setting-item {
    display: flex;
    flex-direction: column;
    gap: 6px;
    background: rgba(255, 255, 255, 0.03);
    padding: 12px;
    border-radius: 10px;
  }

  .setting-title {
    font-size: 13px;
    font-weight: 700;
  }

  .setting-desc {
    font-size: 11px;
    color: var(--text-dim);
  }

  .pill-group {
    display: flex;
    gap: 6px;
    margin-top: 6px;
  }

  .pill-opt {
    flex: 1;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid var(--border-subtle);
    color: var(--text-muted);
    padding: 6px;
    border-radius: 8px;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }

  .pill-opt.active {
    background: var(--fox-primary);
    color: #ffffff;
    border-color: var(--fox-primary);
  }

  .port-specs {
    font-size: 12px;
    color: var(--text-muted);
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 4px;
  }

  .port-specs span {
    color: var(--text-white);
  }

  .font-mono {
    font-family: var(--font-mono);
  }
</style>
