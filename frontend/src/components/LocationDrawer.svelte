<script lang="ts">
  import { onMount, tick } from 'svelte';
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
  let highlightedIndex = 0;
  let searchInputRef: HTMLInputElement | null = null;
  let listContainerRef: HTMLElement | null = null;

  function focusTrap(node: HTMLElement) {
    const focusableSelectors = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key !== 'Tab') return;
      const focusable = Array.from(node.querySelectorAll<HTMLElement>(focusableSelectors)).filter(
        el => !el.hasAttribute('disabled') && el.offsetParent !== null
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey) {
        if (document.activeElement === first) {
          last.focus();
          e.preventDefault();
        }
      } else {
        if (document.activeElement === last) {
          first.focus();
          e.preventDefault();
        }
      }
    }
    node.addEventListener('keydown', handleKeyDown);
    return {
      destroy() {
        node.removeEventListener('keydown', handleKeyDown);
      }
    };
  }

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
    highlightedIndex = 0;
    tick().then(() => searchInputRef?.focus());
  }

  onMount(() => {
    ensureCountries();
  });

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

  $: if (searchQuery !== undefined) {
    highlightedIndex = 0;
  }

  function toggleExpand(code: string) {
    expanded[code] = !expanded[code];
  }

  async function handlePick(countryCode: string, cityCode: string) {
    await SetSelectedCountry(countryCode, cityCode);
    onSelect(countryCode, cityCode);
    onClose();
  }

  function handleSearchKeyDown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (sortedCountries.length > 0) {
        highlightedIndex = Math.min(sortedCountries.length - 1, highlightedIndex + 1);
        scrollHighlightedIntoView();
      }
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (sortedCountries.length > 0) {
        highlightedIndex = Math.max(0, highlightedIndex - 1);
        scrollHighlightedIntoView();
      }
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (sortedCountries.length > 0 && sortedCountries[highlightedIndex]) {
        const c = sortedCountries[highlightedIndex];
        if (c.code === 'REC' || !c.cities || c.cities.length <= 1) {
          const city = c.cities && c.cities[0] ? c.cities[0].code : '';
          handlePick(c.code, city);
        } else {
          toggleExpand(c.code);
        }
      }
    }
  }

  function scrollHighlightedIntoView() {
    tick().then(() => {
      const el = listContainerRef?.querySelector<HTMLElement>(`.country-group[data-index="${highlightedIndex}"]`);
      if (el) {
        el.scrollIntoView({ block: 'nearest' });
      }
    });
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
    <div class="drawer-panel" role="document" use:focusTrap>
      <div class="drawer-header">
        <div class="header-left">
          <h3>Choose Location</h3>
          <span class="location-count">{countries.length} countries</span>
        </div>
        <div class="header-right">
          <button class="refresh-btn" class:spinning={pinging} title="Refresh all server pings" on:click={refreshPings}>
            ↻
          </button>
          <button class="close-btn" aria-label="Close" on:click={onClose}>✕</button>
        </div>
      </div>

      <div class="search-bar">
        <div class="search-input-wrapper">
          <input
            type="text"
            placeholder="Search country or city..."
            bind:value={searchQuery}
            bind:this={searchInputRef}
            on:keydown={handleSearchKeyDown}
          />
          {#if searchQuery.length > 0}
            <button 
              type="button" 
              class="clear-search-btn" 
              aria-label="Clear search" 
              on:click={() => { searchQuery = ''; searchInputRef?.focus(); }}
            >
              ✕
            </button>
          {/if}
        </div>
        <div class="sort-toggle">
          <button class:active={sortBy === 'latency'} on:click={() => (sortBy = 'latency')}>⚡ Ping</button>
          <button class:active={sortBy === 'name'} on:click={() => (sortBy = 'name')}>A–Z</button>
        </div>
      </div>

      <div class="country-list" bind:this={listContainerRef}>
        {#each sortedCountries as country, idx (country.code)}
          {@const isSelectedCountry = selectedCountryCode === country.code}
          {@const bestPing = getBestPingForCountry(country)}
          {@const isExpanded = expanded[country.code] || searchQuery.length > 0}
          {@const isHighlighted = idx === highlightedIndex}

          <div 
            class="country-group" 
            class:selected={isSelectedCountry}
            class:highlighted={isHighlighted}
            data-index={idx}
          >
            <div 
              class="country-row" 
              role="button" 
              tabindex="0"
              on:click={() => {
                highlightedIndex = idx;
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
                {#if country.code === 'REC'}
                  <span class="flag-pill rec">⚡ AUTO</span>
                {:else}
                  <span class="flag-pill">{country.code}</span>
                {/if}
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
  .search-input-wrapper {
    flex: 1;
    position: relative;
    display: flex;
    align-items: center;
  }
  .search-input-wrapper input {
    width: 100%;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.08);
    padding: 8px 32px 8px 12px;
    border-radius: 8px;
    color: white;
    font-size: 0.82rem;
    outline: none;
  }
  .search-input-wrapper input:focus {
    border-color: var(--fox-primary);
  }
  .clear-search-btn {
    position: absolute;
    right: 8px;
    background: transparent;
    border: none;
    color: var(--text-dim);
    font-size: 0.85rem;
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .clear-search-btn:hover {
    color: white;
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
    min-height: 0;
    overflow-y: auto;
    padding: 12px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .country-group {
    flex-shrink: 0;
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
  .country-group.highlighted {
    border-color: rgba(255, 255, 255, 0.25);
    background: rgba(255, 255, 255, 0.06);
  }
  .country-row {
    min-height: 52px;
    padding: 12px 14px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    cursor: pointer;
    flex-shrink: 0;
  }
  .country-info {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .flag-pill {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 32px;
    height: 24px;
    padding: 0 6px;
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 6px;
    font-size: 0.72rem;
    font-weight: 700;
    font-family: var(--font-mono);
    color: #e2e8f0;
    letter-spacing: 0.04em;
    flex-shrink: 0;
  }
  .flag-pill.rec {
    background: rgba(255, 87, 34, 0.15);
    border-color: rgba(255, 87, 34, 0.3);
    color: var(--fox-primary);
    font-size: 0.65rem;
    min-width: 60px;
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
    min-height: 40px;
    padding: 10px 14px 10px 38px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    cursor: pointer;
    border-bottom: 1px solid rgba(255, 255, 255, 0.02);
    transition: background 0.15s;
    flex-shrink: 0;
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
