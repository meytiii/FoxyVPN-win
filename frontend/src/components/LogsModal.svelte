<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { GetLogs, ClearLogs, ExportLogs, ExportLogsText } from '../../wailsjs/go/main/App.js';
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime.js';
  import type { logger } from '../../wailsjs/go/models';

  export let show = false;
  export let onClose: () => void;

  let logs: logger.LogEntry[] = [];
  let filter: 'ALL' | 'INFO' | 'PROBLEMS' = 'ALL';
  let copied = false;
  let exportMessage = '';
  let autoScroll = true;
  let logContainer: HTMLDivElement;

  async function loadLogs() {
    try {
      logs = await GetLogs(filter);
      if (autoScroll && logContainer) {
        setTimeout(() => {
          logContainer.scrollTop = logContainer.scrollHeight;
        }, 50);
      }
    } catch (e) {
      console.error('Failed loading logs:', e);
    }
  }

  $: if (show) {
    loadLogs();
  }

  $: if (filter) {
    loadLogs();
  }

  onMount(() => {
    EventsOn('vpn:log', (entry: logger.LogEntry) => {
      if (filter === 'ALL' || 
         (filter === 'INFO' && entry.level === 'INFO') || 
         (filter === 'PROBLEMS' && (entry.level === 'WARN' || entry.level === 'ERROR'))) {
        logs = [...logs, entry];
        if (logs.length > 2000) logs = logs.slice(-2000);
        if (autoScroll && logContainer) {
          setTimeout(() => {
            logContainer.scrollTop = logContainer.scrollHeight;
          }, 30);
        }
      }
    });
  });

  onDestroy(() => {
    EventsOff('vpn:log');
  });

  async function handleCopy() {
    try {
      const text = await ExportLogsText();
      await navigator.clipboard.writeText(text);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch (e) {
      console.error('Copy failed:', e);
    }
  }

  async function handleExport() {
    try {
      const path = await ExportLogs();
      exportMessage = `Saved to ${path}`;
      setTimeout(() => (exportMessage = ''), 3500);
    } catch (e) {
      exportMessage = 'Export failed';
      setTimeout(() => (exportMessage = ''), 3000);
    }
  }

  async function handleClear() {
    try {
      await ClearLogs();
      logs = [];
    } catch (e) {
      console.error('Clear failed:', e);
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
        <div class="header-title">
          <svg viewBox="0 0 24 24" class="header-icon" fill="none" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h16" />
          </svg>
          <h3>Diagnostic Logs</h3>
          <span class="count-badge">{logs.length}</span>
        </div>
        <button class="close-btn" on:click={onClose}>✕</button>
      </div>

      <div class="filter-bar">
        <div class="chips">
          <button class="chip" class:active={filter === 'ALL'} on:click={() => (filter = 'ALL')}>All</button>
          <button class="chip" class:active={filter === 'INFO'} on:click={() => (filter = 'INFO')}>Info</button>
          <button class="chip chip-problems" class:active={filter === 'PROBLEMS'} on:click={() => (filter = 'PROBLEMS')}>Problems</button>
        </div>
        <div class="actions">
          <button class="action-btn" title="Copy to clipboard" on:click={handleCopy}>
            {copied ? '✓ Copied' : 'Copy'}
          </button>
          <button class="action-btn" title="Export to .txt file" on:click={handleExport}>
            Export
          </button>
          <button class="action-btn action-danger" title="Clear log buffer" on:click={handleClear}>
            Clear
          </button>
        </div>
      </div>

      {#if exportMessage}
        <div class="toast-msg">{exportMessage}</div>
      {/if}

      <div class="logs-console" bind:this={logContainer}>
        {#if logs.length === 0}
          <div class="empty-state">No log entries captured yet.</div>
        {:else}
          {#each logs as item (item.id)}
            <div class="log-row level-{item.level.toLowerCase()}">
              <span class="log-time">{item.time_str}</span>
              <span class="log-level">{item.level}</span>
              <span class="log-tag">[{item.tag}]</span>
              <span class="log-msg">{item.message}</span>
            </div>
          {/each}
        {/if}
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
    max-width: 600px;
    height: 80vh;
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
    padding: 14px 18px;
    background: rgba(255, 255, 255, 0.03);
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .header-title {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .header-icon {
    width: 18px;
    height: 18px;
    color: var(--fox-accent);
  }
  .header-title h3 {
    font-size: 0.95rem;
    font-weight: 600;
  }
  .count-badge {
    font-size: 0.75rem;
    background: rgba(255, 255, 255, 0.1);
    padding: 2px 8px;
    border-radius: 10px;
    color: var(--text-muted);
  }
  .close-btn {
    background: none;
    border: none;
    color: var(--text-muted);
    font-size: 1.1rem;
    cursor: pointer;
  }
  .close-btn:hover {
    color: white;
  }
  .filter-bar {
    padding: 10px 16px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: rgba(0, 0, 0, 0.25);
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  }
  .chips {
    display: flex;
    gap: 6px;
  }
  .chip {
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--text-muted);
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 0.75rem;
    cursor: pointer;
  }
  .chip.active {
    background: rgba(255, 102, 0, 0.2);
    border-color: var(--fox-primary);
    color: white;
  }
  .chip-problems.active {
    background: rgba(239, 68, 68, 0.2);
    border-color: #ef4444;
  }
  .actions {
    display: flex;
    gap: 6px;
  }
  .action-btn {
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: var(--text-white);
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 0.75rem;
    cursor: pointer;
  }
  .action-btn:hover {
    background: rgba(255, 255, 255, 0.15);
  }
  .action-danger:hover {
    background: rgba(239, 68, 68, 0.3);
    border-color: #ef4444;
  }
  .toast-msg {
    padding: 6px 16px;
    background: rgba(16, 185, 129, 0.2);
    color: #10b981;
    font-size: 0.75rem;
    border-bottom: 1px solid rgba(16, 185, 129, 0.3);
  }
  .logs-console {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 12px 16px;
    font-family: var(--font-mono);
    font-size: 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 4px;
    background: #080a10;
  }
  .empty-state {
    color: var(--text-dim);
    margin: auto;
    font-style: italic;
  }
  .log-row {
    display: flex;
    gap: 8px;
    line-height: 1.4;
    word-break: break-all;
    flex-shrink: 0;
  }
  .log-time {
    color: var(--text-dim);
    flex-shrink: 0;
  }
  .log-level {
    flex-shrink: 0;
    font-weight: 600;
    width: 44px;
  }
  .log-tag {
    color: #38bdf8;
    flex-shrink: 0;
  }
  .log-msg {
    color: var(--text-white);
  }
  .level-info .log-level {
    color: #10b981;
  }
  .level-warn .log-level {
    color: #f59e0b;
  }
  .level-warn {
    background: rgba(245, 158, 11, 0.05);
  }
  .level-error .log-level {
    color: #ef4444;
  }
  .level-error {
    background: rgba(239, 68, 68, 0.1);
  }
</style>
