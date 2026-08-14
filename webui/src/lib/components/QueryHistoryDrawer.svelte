<script>
  import { copyText } from '../result-grid.js';

  let {
    isOpen = false,
    history = [],
    profiles = [],
    selectedProfile = '',
    onClose,
    onSelectHistoryItem,
    onSelectSQL,
    onClear
  } = $props();

  let selectedFilter = $state('__all__');
  let searchQuery = $state('');
  let copiedId = $state('');

  let filteredHistory = $derived.by(() => {
    let list = history;
    if (selectedFilter === '__current__') {
      if (selectedProfile) {
        list = list.filter((item) => item.profile === selectedProfile);
      }
    } else if (selectedFilter !== '__all__') {
      list = list.filter((item) => item.profile === selectedFilter);
    }

    if (searchQuery.trim()) {
      const q = searchQuery.trim().toLowerCase();
      list = list.filter((item) => item.sql.toLowerCase().includes(q));
    }
    return list;
  });

  let effectiveClearFilter = $derived.by(() => {
    if (selectedFilter === '__current__') {
      return selectedProfile || '__all__';
    }
    return selectedFilter;
  });

  function handleSelect(item) {
    if (onSelectHistoryItem) {
      onSelectHistoryItem(item);
    } else if (onSelectSQL) {
      onSelectSQL(item.sql);
    }
    onClose?.();
  }

  async function handleCopySQL(e, item) {
    e.stopPropagation();
    try {
      await copyText(item.sql);
      copiedId = item.id;
      setTimeout(() => {
        copiedId = '';
      }, 1200);
    } catch {
      // ignore
    }
  }
</script>

{#if isOpen}
  <div class="fixed inset-0 z-40 flex justify-end bg-black/50 backdrop-blur-xs transition-opacity animate-fade-in">
    <div
      role="button"
      tabindex="-1"
      aria-label="Close modal overlay"
      class="absolute inset-0"
      onclick={onClose}
      onkeydown={(e) => e.key === 'Escape' && onClose()}
    ></div>
    <aside class="relative z-50 flex h-full w-[26rem] max-w-full flex-col border-l border-[var(--panel-border)] bg-[var(--panel-bg)] p-4 shadow-2xl">
      <!-- Header -->
      <div class="mb-3 flex items-center justify-between border-b border-[var(--panel-border)] pb-3">
        <div class="flex items-center gap-2">
          <svg class="h-4 w-4 text-[var(--accent)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 14 14"/>
          </svg>
          <strong class="text-sm font-semibold text-[var(--text)]">Query History</strong>
          <span class="rounded-full bg-[var(--pill-bg)] px-2 py-0.2 text-[10px] font-mono text-[var(--pill-text)]">
            {filteredHistory.length}
          </span>
        </div>
        <div class="flex items-center gap-2">
          {#if filteredHistory.length > 0}
            <button
              class="text-xs text-[var(--muted)] hover:text-red-500 transition"
              onclick={() => onClear?.(effectiveClearFilter)}
            >
              Clear
            </button>
          {/if}
          <button
            class="rounded-lg p-1 text-[var(--muted)] hover:bg-[var(--accent-soft)] hover:text-[var(--text)]"
            onclick={onClose}
          >
            ✕
          </button>
        </div>
      </div>

      <!-- Search and Filter -->
      <div class="grid gap-2 mb-3 border-b border-[var(--panel-border)] pb-3">
        <div class="relative">
          <input
            type="text"
            placeholder="Search in history SQL..."
            class="xsql-input pl-7 text-xs h-7"
            bind:value={searchQuery}
          />
          <svg class="absolute left-2 top-2 h-3.5 w-3.5 text-[var(--muted)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>
          </svg>
        </div>

        <div class="flex items-center justify-between gap-2">
          <span class="text-[11px] text-[var(--muted)] shrink-0">Profile:</span>
          <select
            class="flex-1 rounded border border-[var(--input-border)] bg-[var(--input-bg)] px-2 py-1 text-xs text-[var(--text)] outline-none"
            bind:value={selectedFilter}
          >
            <option value="__all__">All Profiles</option>
            <option value="__current__">
              Current ({selectedProfile || 'None'})
            </option>
            {#each profiles as p (p.name)}
              {#if p.name !== selectedProfile}
                <option value={p.name}>{p.name}</option>
              {/if}
            {/each}
          </select>
        </div>
      </div>

      <!-- History List -->
      {#if filteredHistory.length === 0}
        <div class="flex flex-1 flex-col items-center justify-center p-8 text-center text-xs text-[var(--muted)]">
          <svg class="h-8 w-8 text-[var(--muted)] opacity-40 mb-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
          <p>No queries found matching the filter.</p>
        </div>
      {:else}
        <div class="xsql-scroll flex-1 overflow-y-auto pr-1">
          <div class="grid gap-2">
            {#each filteredHistory as item (item.id)}
              <div
                role="button"
                tabindex="0"
                class="group flex flex-col gap-1.5 rounded-lg border border-[var(--panel-border)] bg-[var(--panel-inner)] p-2.5 transition hover:border-[var(--accent-border)] hover:shadow-sm cursor-pointer"
                onclick={() => handleSelect(item)}
                onkeydown={(e) => e.key === 'Enter' && handleSelect(item)}
              >
                <!-- Top metadata row -->
                <div class="flex items-center justify-between gap-2 text-xs">
                  <div class="flex items-center gap-1.5 min-w-0">
                    <span class="rounded bg-[var(--pill-bg)] px-1.5 py-0.2 font-mono text-[10px] text-[var(--pill-text)]">
                      {item.profile || 'default'}
                    </span>
                    {#if item.error}
                      <span class="rounded bg-red-500/10 px-1 py-0.2 text-[9px] font-bold text-red-500 border border-red-500/20">
                        ERR
                      </span>
                    {:else}
                      <span class="rounded bg-emerald-500/10 px-1 py-0.2 text-[9px] font-bold text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                        OK
                      </span>
                    {/if}
                  </div>
                  <div class="flex items-center gap-1">
                    <span class="text-[10px] text-[var(--muted)]">{item.timestamp}</span>
                    <button
                      type="button"
                      class="rounded p-0.5 text-[var(--muted)] hover:text-[var(--text)] hover:bg-[var(--panel-bg)] opacity-0 group-hover:opacity-100 transition"
                      title="Copy SQL"
                      onclick={(e) => handleCopySQL(e, item)}
                    >
                      {#if copiedId === item.id}
                        <span class="text-[10px] text-emerald-500">✓</span>
                      {:else}
                        <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>
                        </svg>
                      {/if}
                    </button>
                  </div>
                </div>

                <!-- SQL Body -->
                <pre class="m-0 max-h-16 overflow-hidden font-mono text-[11px] text-[var(--text)] whitespace-pre-wrap break-all leading-4">{item.sql}</pre>

                <!-- Bottom status row -->
                <div class="flex items-center justify-between border-t border-[var(--panel-border)] pt-1 text-[10px] text-[var(--muted)]">
                  {#if item.error}
                    <span class="text-red-500 truncate max-w-[200px]">{item.error}</span>
                  {:else}
                    <span>{item.rowCount} rows</span>
                  {/if}
                  <span class="font-mono">{item.durationMs}ms</span>
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </aside>
  </div>
{/if}
