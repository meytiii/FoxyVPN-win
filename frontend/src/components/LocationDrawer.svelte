<script lang="ts">
  import { onMount } from 'svelte';
  import { SetSelectedCountry, PingAllServers, GetCountries } from '../../wailsjs/go/main/App.js';
  import type { remotesettings } from '../../wailsjs/go/models';

  export let show = false;
  export let countries: remotesettings.VpnCountry[] = [];
  export let selectedCountryCode = 'REC';
  export let selectedCityCode = 'REC';
  export let onClose: () => void;
  export let onSelect: (country: string, city: string) => void;

  let searchQuery = '';
  let sortBy: 'latency' | 'name' = 'latency';
  let pings: Record<string, number> = {};
  let pinging = false;
  let expanded: Record<string, boolean> = {};

  async function ensureCountries() {
    if (!countries || countries.length === 0) {
      try {
        const loaded = await GetCountries();
        if (loaded && loaded.length > 0) countries = loaded;
      } catch (e) {
        console.error('Failed loading countries:', e);
      }
    }
  }

  async function refreshPings() {
    pinging = true;
    try {
      const results = await PingAllServers();
      if (results) pings = results;
    } catch (e) {
      console.warn('Ping all servers failed:', e);
    } finally {
      pinging = false;
    }
  }

  $: if (show) {
    ensureCountries();
    refreshPings();
  }

  onMount(() => {
    ensureCountries();
  });

  function getCountryFlag(code: string): string {
    if (!code || code === 'REC' || code.length !== 2) return '🌐';
    const upper = code.toUpperCase();
    const first = upper.charCodeAt(0) - 65 + 0x1f1e6;
    const second = upper.charCodeAt(1) - 65 + 0x1f1e6;
    return String.fromCodePoint(first, second);
  }

  function getBestPingForCountry(country: remotesettings.VpnCountry): number {
    if (country.code === 'REC') {
      const recPing = pings['REC:REC'];
      return recPing !== undefined ? recPing : 9999;
    }
    let best = 9999;
    if (country.cities) {
      for (const city of country.cities) {
        const key = `${country.code}:${city.code}`;
        const p = pings[key];
        if (p !== undefined && p > 0 && p < best) {
          best = p;
        }
      }
    }
    return best;
  }

  $: filteredCountries = countries.filter(c => {
    if (!searchQuery) return true;
    const q = searchQuery.toLowerCase();
    if (c.name.toLowerCase().includes(q) || c.code.toLowerCase().includes(q)) return true;
    if (c.cities && c.cities.some(ci => ci.name.toLowerCase().includes(q) || ci.code.toLowerCase().includes(q))) return true;
    return false;
  });

  $: sortedCountries = [...filteredCountries].sort((a, b) => {
    if (a.code === 'REC') return -1;
    if (b.code === 'REC') return 1;
    if (sortBy === 'latency') {
      const pingA = getBestPingForCountry(a);
      const pingB = getBestPingForCountry(b);
      return pingA - pingB;
    }
    return a.name.localeCompare(b.name);
  });

  function toggleExpand(code: string) {
    expanded[code] = !expanded[code];
  }

  async function handlePick(countryCode: string, cityCode: string) {
    await SetSelectedCountry(countryCode, cityCode);
    onSelect(countryCode, cityCode);
    onClose();
  }
</script>

{#if show}
  <div 
    class="drawer-overlay" 
    role="dialog" 
    aria-modal="true" 
    tabindex="-1"
    on:click={(e) => { if (e.target === e.currentTarget) onClose(); }}
    on:keydown={(e) => { if (e.key === 'Escape') onClose(); }}
  >
    <div class="drawer-panel" role="document">
      <div class="drawer-header">
        <div class="header-left">
          <h3>Choose Location</h3>
          <span class="location-count">{countries.length} countries</span>
        </div>
        <div class="header-right">
          <button class="refresh-btn" class:spinning={pinging} title="Refresh all server pings" on:click={refreshPings}>
            ↻
          </button>
          <button class="close-btn" on:click={onClose}>✕</button>
        </div>
      </div>

      <div class="search-bar">
        <input
          type="text"
          placeholder="Search country or city..."
          bind:value={searchQuery}
        />
        <div class="sort-toggle">
          <button class:active={sortBy === 'latency'} on:click={() => (sortBy = 'latency')}>⚡ Ping</button>
          <button class:active={sortBy === 'name'} on:click={() => (sortBy = 'name')}>A–Z</button>
        </div>
      </div>

      <div class="country-list">
        {#each sortedCountries as country (country.code)}
          {@const isSelectedCountry = selectedCountryCode === country.code}
          {@const bestPing = getBestPingForCountry(country)}
          {@const isExpanded = expanded[country.code] || searchQuery.length > 0}

          <div class="country-group" class:selected={isSelectedCountry}>
            <div 
              class="country-row" 
              role="button" 
              tabindex="0"
              on:click={() => {
                if (country.code === 'REC' || !country.cities || country.cities.length <= 1) {
                  const city = country.cities && country.cities[0] ? country.cities[0].code : '';
                  handlePick(country.code, city);
                } else {
                  toggleExpand(country.code);
                }
              }}
              on:keydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  if (country.code === 'REC' || !country.cities || country.cities.length <= 1) {
                    const city = country.cities && country.cities[0] ? country.cities[0].code : '';
                    handlePick(country.code, city);
                  } else {
                    toggleExpand(country.code);
                  }
                }
              }}
            >
              <div class="country-info">
                <span class="flag-icon">{getCountryFlag(country.code)}</span>
                <div class="name-box">
                  <span class="country-name">{country.name}</span>
                  {#if country.code === 'REC'}
                    <span class="sub-label">Automatically routes to nearest fast node</span>
                  {:else if country.cities && country.cities.length > 1}
                    <span class="sub-label">{country.cities.length} cities available</span>
                  {/if}
                </div>
              </div>

              <div class="country-meta">
                {#if bestPing < 9999 && bestPing > 0}
                  <span class="ping-chip" class:fast={bestPing < 80} class:mid={bestPing >= 80 && bestPing < 160} class:slow={bestPing >= 160}>
                    {bestPing} ms
                  </span>
                {:else if pinging}
                  <span class="ping-chip pinging">pinging…</span>
                {/if}

                {#if country.cities && country.cities.length > 1 && country.code !== 'REC'}
                  <span class="expand-caret" class:open={isExpanded}>▼</span>
                {:else if isSelectedCountry}
                  <span class="check-icon">✓</span>
                {/if}
              </div>
            </div>

            {#if isExpanded && country.cities && country.cities.length > 1}
              <div class="city-sublist">
                {#each country.cities as city (city.code)}
                  {@const cityKey = `${country.code}:${city.code}`}
                  {@const cityPing = pings[cityKey]}
                  {@const isSelectedCity = isSelectedCountry && selectedCityCode === city.code}

                  <div 
                    class="city-row" 
                    role="button" 
                    tabindex="0"
                    class:selected={isSelectedCity} 
                    on:click={() => handlePick(country.code, city.code)}
                    on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') handlePick(country.code, city.code); }}
                  >
                    <div class="city-info">
                      <span class="city-bullet">↳</span>
                      <span class="city-name">{city.name || city.code}</span>
                      <span class="server-count">({city.servers?.length || 1} node)</span>
                    </div>

                    <div class="city-meta">
                      {#if cityPing !== undefined && cityPing > 0}
                        <span class="ping-chip" class:fast={cityPing < 80} class:mid={cityPing >= 80 && cityPing < 160} class:slow={cityPing >= 160}>
                          {cityPing} ms
                        </span>
                      {:else if pinging}
                        <span class="ping-chip pinging">…</span>
                      {/if}
                      {#if isSelectedCity}
                        <span class="check-icon">✓</span>
                      {/if}
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  </div>
{/if}

<style>
  .drawer-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.8);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .drawer-panel {
    width: 92%;
    max-width: 420px;
    height: 85vh;
    max-height: 640px;
    background: #0f121d;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: 0 20px 50px rgba(0, 0, 0, 0.6);
  }
  .drawer-header {
    padding: 18px 20px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  }
  .header-left h3 {
    font-size: 1.05rem;
    font-weight: 700;
    color: white;
    margin: 0;
  }
  .location-count {
    font-size: 0.75rem;
    color: var(--text-dim);
  }
  .header-right {
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .refresh-btn {
    background: rgba(255, 255, 255, 0.08);
    border: none;
    color: var(--text-muted);
    font-size: 1.1rem;
    width: 32px;
    height: 32px;
    border-radius: 8px;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s;
  }
  .refresh-btn:hover {
    color: white;
    background: rgba(255, 255, 255, 0.15);
  }
  .refresh-btn.spinning {
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
  .close-btn {
    background: none;
    border: none;
    color: var(--text-muted);
    font-size: 1.2rem;
    cursor: pointer;
    padding: 4px;
    border-radius: 6px;
  }
  .close-btn:hover {
    color: white;
    background: rgba(255, 255, 255, 0.08);
  }
  .search-bar {
    padding: 12px 18px;
    display: flex;
    gap: 8px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  }
  .search-bar input {
    flex: 1;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.08);
    padding: 8px 12px;
    border-radius: 8px;
    color: white;
    font-size: 0.82rem;
    outline: none;
  }
  .search-bar input:focus {
    border-color: var(--fox-primary);
  }
  .sort-toggle {
    display: flex;
    background: rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    padding: 2px;
  }
  .sort-toggle button {
    background: none;
    border: none;
    color: var(--text-dim);
    font-size: 0.72rem;
    padding: 4px 8px;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s;
  }
  .sort-toggle button.active {
    background: rgba(255, 255, 255, 0.15);
    color: white;
    font-weight: 600;
  }
  .country-list {
    flex: 1;
    overflow-y: auto;
    padding: 12px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .country-group {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 12px;
    overflow: hidden;
    transition: all 0.2s;
  }
  .country-group:hover {
    background: rgba(255, 255, 255, 0.04);
    border-color: rgba(255, 255, 255, 0.1);
  }
  .country-group.selected {
    border-color: rgba(255, 102, 0, 0.4);
    background: rgba(255, 102, 0, 0.06);
  }
  .country-row {
    padding: 12px 14px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    cursor: pointer;
  }
  .country-info {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .flag-icon {
    font-size: 1.4rem;
  }
  .name-box {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .country-name {
    font-size: 0.88rem;
    font-weight: 600;
    color: white;
  }
  .sub-label {
    font-size: 0.7rem;
    color: var(--text-dim);
  }
  .country-meta {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .ping-chip {
    font-size: 0.7rem;
    font-weight: 600;
    padding: 2px 7px;
    border-radius: 4px;
    font-family: var(--font-mono);
  }
  .ping-chip.fast {
    background: rgba(16, 185, 129, 0.15);
    color: #10b981;
  }
  .ping-chip.mid {
    background: rgba(245, 158, 11, 0.15);
    color: #f59e0b;
  }
  .ping-chip.slow {
    background: rgba(239, 68, 68, 0.15);
    color: #ef4444;
  }
  .ping-chip.pinging {
    background: rgba(255, 255, 255, 0.05);
    color: var(--text-dim);
  }
  .expand-caret {
    font-size: 0.65rem;
    color: var(--text-dim);
    transition: transform 0.2s;
  }
  .expand-caret.open {
    transform: rotate(180deg);
  }
  .check-icon {
    color: var(--fox-primary);
    font-weight: 800;
  }
  .city-sublist {
    background: rgba(0, 0, 0, 0.25);
    border-top: 1px solid rgba(255, 255, 255, 0.04);
    display: flex;
    flex-direction: column;
  }
  .city-row {
    padding: 10px 14px 10px 38px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    cursor: pointer;
    border-bottom: 1px solid rgba(255, 255, 255, 0.02);
    transition: background 0.15s;
  }
  .city-row:hover {
    background: rgba(255, 255, 255, 0.03);
  }
  .city-row.selected {
    background: rgba(255, 102, 0, 0.1);
  }
  .city-info {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .city-bullet {
    color: var(--text-dim);
    font-size: 0.8rem;
  }
  .city-name {
    font-size: 0.8rem;
    color: white;
  }
  .server-count {
    font-size: 0.68rem;
    color: var(--text-dim);
  }
  .city-meta {
    display: flex;
    align-items: center;
    gap: 6px;
  }
</style>
