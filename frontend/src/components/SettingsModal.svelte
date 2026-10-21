<script lang="ts">
  import { onMount } from 'svelte';
  import { GetSettings, UpdateSettings, QuitApplication } from '../../wailsjs/go/main/App.js';
  import type { settings } from '../../wailsjs/go/models';

  export let show = false;
  export let onClose: () => void;

  let s: settings.AppSettings = {
    exit_check_enabled: true,
    dns_provider: 'SYSTEM_DEFAULT',
    custom_doh_url: '',
    bootstrap_dns_provider: 'AUTOMATIC',
    custom_bootstrap_ip: '',
    socks_port: 1080,
    socks_bind_address: '127.0.0.1',
    http_proxy_enabled: true,
    http_proxy_port: 8080,
    proxy_only_mode: false,
    custom_edge_address: '',
    tls_split_mode: 1,
    upstream_proxy_enabled: false,
    upstream_proxy_type: 'SOCKS5',
    upstream_proxy_host: '',
    upstream_proxy_port: 1080,
    upstream_proxy_username: '',
    upstream_proxy_password: '',
    minimize_to_tray: true,
    close_to_tray: true,
    launch_on_startup: false,
    bypass_lan: true,
    custom_bypass_list: '<local>;192.168.*;10.*;172.16.*'
  } as settings.AppSettings;

  let activeTab: 'connection' | 'dns' | 'proxy' | 'advanced' | 'system' = 'connection';
  let savedMessage = '';
  let saving = false;

  async function loadSettings() {
    try {
      const loaded = await GetSettings();
      if (loaded) s = loaded;
    } catch (e) {
      console.error('Failed to load settings:', e);
    }
  }

  $: if (show) {
    loadSettings();
  }

  async function handleSave() {
    saving = true;
    try {
      await UpdateSettings(s);
      savedMessage = 'Settings saved successfully';
      setTimeout(() => (savedMessage = ''), 2500);
    } catch (e) {
      console.error('Failed to save settings:', e);
    } finally {
      saving = false;
    }
  }

  async function handleQuit() {
    try {
      await QuitApplication();
    } catch (e) {
      console.error('Failed to quit:', e);
    }
  }
</script>

{#if show}
  <div 
    class="modal-overlay" 
    role="dialog" 
    aria-modal="true" 
    tabindex="-1"
    on:click={(e) => { if (e.target === e.currentTarget) onClose(); }}
    on:keydown={(e) => { if (e.key === 'Escape') onClose(); }}
  >
    <div class="modal-box" role="document">
      <div class="modal-header">
        <h3>Settings & Preferences</h3>
        <button class="close-btn" on:click={onClose}>✕</button>
      </div>

      <div class="tab-nav">
        <button class:active={activeTab === 'connection'} on:click={() => (activeTab = 'connection')}>Connection</button>
        <button class:active={activeTab === 'dns'} on:click={() => (activeTab = 'dns')}>DNS</button>
        <button class:active={activeTab === 'proxy'} on:click={() => (activeTab = 'proxy')}>Local Proxy</button>
        <button class:active={activeTab === 'advanced'} on:click={() => (activeTab = 'advanced')}>Anti-Censorship</button>
        <button class:active={activeTab === 'system'} on:click={() => (activeTab = 'system')}>Desktop</button>
      </div>

      {#if savedMessage}
        <div class="save-toast">{savedMessage}</div>
      {/if}

      <div class="tab-body">
        {#if activeTab === 'connection'}
          <label class="setting-item">
            <div class="setting-text">
              <span class="setting-label">Verify Exit Location</span>
              <span class="setting-desc">Check that tunnel exits in the selected country via Cloudflare trace</span>
            </div>
            <input type="checkbox" bind:checked={s.exit_check_enabled} />
          </label>

          <label class="setting-item">
            <div class="setting-text">
              <span class="setting-label">Bypass Intranet & LAN</span>
              <span class="setting-desc">Keep local network devices accessible (192.168.*, 10.*, &lt;local&gt;)</span>
            </div>
            <input type="checkbox" bind:checked={s.bypass_lan} />
          </label>

          <label class="setting-field">
            <span class="setting-title">Windows Proxy Bypass List</span>
            <input type="text" bind:value={s.custom_bypass_list} placeholder="<local>;192.168.*;10.*" />
            <span class="hint">Semicolon-separated list of domains or IP prefixes</span>
          </label>
        {:else if activeTab === 'dns'}
          <label class="setting-field">
            <span class="setting-title">Tunnel DNS Provider (DoH)</span>
            <select bind:value={s.dns_provider}>
              <option value="SYSTEM_DEFAULT">Default (Resolved at VPN Edge)</option>
              <option value="CLOUDFLARE">Cloudflare (1.1.1.1)</option>
              <option value="GOOGLE">Google (8.8.8.8)</option>
              <option value="QUAD9">Quad9 (9.9.9.9 - Malware blocking)</option>
              <option value="ADGUARD">AdGuard (Ad & tracker blocking)</option>
              <option value="CUSTOM">Custom DoH URL</option>
            </select>
          </label>

          {#if s.dns_provider === 'CUSTOM'}
            <label class="setting-field">
              <span class="setting-title">Custom DoH Endpoint URL</span>
              <input type="text" bind:value={s.custom_doh_url} placeholder="https://example.com/dns-query" />
            </label>
          {/if}

          <label class="setting-field">
            <span class="setting-title">Pre-Tunnel Resolver (Bootstrap DNS)</span>
            <select bind:value={s.bootstrap_dns_provider}>
              <option value="AUTOMATIC">Automatic (Races 9.9.9.9, 1.1.1.1, 8.8.8.8)</option>
              <option value="CLOUDFLARE">Cloudflare (1.1.1.1, 1.0.0.1)</option>
              <option value="GOOGLE">Google (8.8.8.8, 8.8.4.4)</option>
              <option value="QUAD9">Quad9 (9.9.9.9, 149.112.112.112)</option>
              <option value="SYSTEM">System Windows DNS</option>
              <option value="CUSTOM">Custom Resolver IP</option>
            </select>
            <span class="hint">Resolves Fastly edge nodes before tunnel connects to bypass DNS poisoning</span>
          </label>

          {#if s.bootstrap_dns_provider === 'CUSTOM'}
            <label class="setting-field">
              <span class="setting-title">Custom Resolver IP</span>
              <input type="text" bind:value={s.custom_bootstrap_ip} placeholder="e.g. 76.76.2.2" />
            </label>
          {/if}
        {:else if activeTab === 'proxy'}
          <label class="setting-item">
            <div class="setting-text">
              <span class="setting-label">Proxy-Only Mode</span>
              <span class="setting-desc">Run SOCKS5 & HTTP proxy servers without modifying Windows system proxy</span>
            </div>
            <input type="checkbox" bind:checked={s.proxy_only_mode} />
          </label>

          <label class="setting-field">
            <span class="setting-title">Local Bind Address</span>
            <select bind:value={s.socks_bind_address}>
              <option value="127.0.0.1">Loopback only (127.0.0.1 - Recommended)</option>
              <option value="0.0.0.0">All interfaces (0.0.0.0 - Share on Local LAN)</option>
            </select>
          </label>

          <div class="row-fields">
            <label class="setting-field half">
              <span class="setting-title">SOCKS5 Port</span>
              <input type="number" min="1" max="65535" bind:value={s.socks_port} />
            </label>
            <label class="setting-field half">
              <span class="setting-title">HTTP Proxy Port</span>
              <input type="number" min="1" max="65535" bind:value={s.http_proxy_port} />
            </label>
          </div>

          <label class="setting-item">
            <div class="setting-text">
              <span class="setting-label">Enable HTTP Proxy</span>
              <span class="setting-desc">Runs local HTTP CONNECT proxy alongside SOCKS5</span>
            </div>
            <input type="checkbox" bind:checked={s.http_proxy_enabled} />
          </label>
        {:else if activeTab === 'advanced'}
          <label class="setting-field">
            <span class="setting-title">TLS ClientHello Fragmentation (Bypass DPI)</span>
            <select bind:value={s.tls_split_mode}>
              <option value={0}>Disabled (Standard TLS handshake)</option>
              <option value={1}>First Byte (Splits record header at 0x16)</option>
              <option value={2}>SNI Hostname (Fragments SNI across TCP packets)</option>
            </select>
            <span class="hint">Disrupts Deep Packet Inspection (DPI) on censored networks</span>
          </label>

          <label class="setting-field">
            <span class="setting-title">Custom Edge Node Address</span>
            <input type="text" bind:value={s.custom_edge_address} placeholder="Custom host or IP (leave empty for default)" />
            <span class="hint">Overrides standard Fastly MASQUE edge resolution</span>
          </label>

          <label class="setting-item">
            <div class="setting-text">
              <span class="setting-label">Chain Through Upstream Proxy</span>
              <span class="setting-desc">Route VPN connection through an external proxy first</span>
            </div>
            <input type="checkbox" bind:checked={s.upstream_proxy_enabled} />
          </label>

          {#if s.upstream_proxy_enabled}
            <label class="setting-field">
              <span class="setting-title">Upstream Proxy Type</span>
              <select bind:value={s.upstream_proxy_type}>
                <option value="SOCKS5">SOCKS5</option>
                <option value="HTTP">HTTP CONNECT</option>
              </select>
            </label>
            <div class="row-fields">
              <label class="setting-field half">
                <span class="setting-title">Proxy Host</span>
                <input type="text" bind:value={s.upstream_proxy_host} placeholder="127.0.0.1" />
              </label>
              <label class="setting-field half">
                <span class="setting-title">Proxy Port</span>
                <input type="number" min="1" max="65535" bind:value={s.upstream_proxy_port} />
              </label>
            </div>
            <div class="row-fields">
              <label class="setting-field half">
                <span class="setting-title">Username (Optional)</span>
                <input type="text" bind:value={s.upstream_proxy_username} />
              </label>
              <label class="setting-field half">
                <span class="setting-title">Password (Optional)</span>
                <input type="password" bind:value={s.upstream_proxy_password} />
              </label>
            </div>
          {/if}
        {:else if activeTab === 'system'}
          <label class="setting-item">
            <div class="setting-text">
              <span class="setting-label">Launch on Windows Startup</span>
              <span class="setting-desc">Start FoxyVPN in background when Windows logs on</span>
            </div>
            <input type="checkbox" bind:checked={s.launch_on_startup} />
          </label>

          <label class="setting-item">
            <div class="setting-text">
              <span class="setting-label">Minimize to System Tray</span>
              <span class="setting-desc">Minimizing sends application to taskbar notification area</span>
            </div>
            <input type="checkbox" bind:checked={s.minimize_to_tray} />
          </label>

          <label class="setting-item">
            <div class="setting-text">
              <span class="setting-label">Close to System Tray</span>
              <span class="setting-desc">Closing window keeps VPN tunnel running in background</span>
            </div>
            <input type="checkbox" bind:checked={s.close_to_tray} />
          </label>
        {/if}
      </div>

      <div class="modal-footer">
        <button class="btn-quit" on:click={handleQuit}>Quit Application</button>
        <button class="btn-cancel" on:click={onClose}>Close</button>
        <button class="btn-save" on:click={handleSave} disabled={saving}>
          {saving ? 'Saving...' : 'Apply & Save'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.75);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .modal-box {
    width: 92%;
    max-width: 520px;
    height: 82vh;
    max-height: 640px;
    background: #0f121d;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: 0 20px 50px rgba(0, 0, 0, 0.6);
  }
  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 18px 20px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  }
  .modal-header h3 {
    margin: 0;
    font-size: 1.05rem;
    font-weight: 700;
    color: white;
  }
  .close-btn {
    background: transparent;
    border: none;
    color: var(--text-dim);
    font-size: 1.1rem;
    cursor: pointer;
    padding: 4px;
    border-radius: 6px;
  }
  .close-btn:hover {
    color: white;
    background: rgba(255, 255, 255, 0.08);
  }
  .tab-nav {
    display: flex;
    background: rgba(255, 255, 255, 0.03);
    padding: 6px 12px;
    gap: 6px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    overflow-x: auto;
  }
  .tab-nav button {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: 0.78rem;
    font-weight: 600;
    padding: 6px 12px;
    border-radius: 8px;
    cursor: pointer;
    white-space: nowrap;
    transition: all 0.2s;
  }
  .tab-nav button:hover {
    color: white;
    background: rgba(255, 255, 255, 0.06);
  }
  .tab-nav button.active {
    background: rgba(255, 87, 34, 0.15);
    color: var(--fox-primary);
  }
  .save-toast {
    background: rgba(16, 185, 129, 0.2);
    border: 1px solid rgba(16, 185, 129, 0.4);
    color: #10b981;
    font-size: 0.8rem;
    padding: 6px 14px;
    margin: 8px 16px 0;
    border-radius: 6px;
    text-align: center;
  }
  .tab-body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 16px 20px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .setting-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 14px;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 10px;
    cursor: pointer;
    flex-shrink: 0;
  }
  .setting-item input[type="checkbox"] {
    accent-color: var(--fox-primary);
    width: 18px;
    height: 18px;
    cursor: pointer;
  }
  .setting-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding-right: 12px;
  }
  .setting-label {
    font-size: 0.85rem;
    font-weight: 600;
    color: white;
  }
  .setting-desc {
    font-size: 0.72rem;
    color: var(--text-dim);
  }
  .setting-field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .setting-field .setting-title {
    font-size: 0.78rem;
    color: var(--text-muted);
    font-weight: 500;
  }
  .setting-field input, .setting-field select {
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: white;
    padding: 8px 12px;
    border-radius: 8px;
    font-size: 0.82rem;
    outline: none;
  }
  .setting-field input:focus, .setting-field select:focus {
    border-color: var(--fox-primary);
  }
  .setting-field select option {
    background: #0f121d;
    color: white;
  }
  .hint {
    font-size: 0.7rem;
    color: var(--text-dim);
  }
  .row-fields {
    display: flex;
    gap: 12px;
  }
  .half {
    flex: 1;
  }
  .modal-footer {
    padding: 14px 20px;
    background: rgba(255, 255, 255, 0.02);
    border-top: 1px solid rgba(255, 255, 255, 0.08);
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 10px;
  }
  .btn-quit {
    background: rgba(239, 68, 68, 0.12);
    border: 1px solid rgba(239, 68, 68, 0.25);
    color: #ef4444;
    padding: 8px 14px;
    border-radius: 8px;
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
    margin-right: auto;
    transition: all 0.2s;
  }
  .btn-quit:hover {
    background: rgba(239, 68, 68, 0.22);
  }
  .btn-cancel {
    background: rgba(255, 255, 255, 0.08);
    border: none;
    color: var(--text-muted);
    padding: 8px 16px;
    border-radius: 8px;
    font-size: 0.82rem;
    cursor: pointer;
  }
  .btn-save {
    background: var(--fox-primary);
    border: none;
    color: white;
    padding: 8px 18px;
    border-radius: 8px;
    font-size: 0.82rem;
    font-weight: 600;
    cursor: pointer;
  }
  .btn-save:hover {
    filter: brightness(1.1);
  }
</style>
