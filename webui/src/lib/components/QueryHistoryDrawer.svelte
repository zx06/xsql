<script>
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

  function formatTime(timestamp) {
    return timestamp || '';
  }

  let filteredHistory = $derived.by(() => {
    if (selectedFilter === '__current__') {
      if (!selectedProfile) return history;
      return history.filter((item) => item.profile === selectedProfile);
    }
    if (selectedFilter === '__all__') {
      return history;
    }
    return history.filter((item) => item.profile === selectedFilter);
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
</script>

{#if isOpen}
  <div class="fixed inset-0 z-40 flex justify-end bg-black/40 backdrop-blur-xs transition-opacity">
    <div
      role="button"
      tabindex="-1"
      aria-label="Close modal overlay"
      class="absolute inset-0"
      onclick={onClose}
      onkeydown={(e) => e.key === 'Escape' && onClose()}
    ></div>
    <aside class="relative z-50 flex h-full w-96 max-w-full flex-col border-l border-[var(--panel-border)] bg-[var(--panel-bg)] p-4 shadow-2xl">
      <div class="mb-3 flex items-center justify-between border-b border-[var(--panel-border)] pb-3">
        <div class="flex items-center gap-2">
          <strong class="text-base text-[var(--text)]">Query History</strong>
          <span class="rounded-full bg-[var(--pill-bg)] px-2 py-0.5 text-xs text-[var(--pill-text)]">
            {filteredHistory.length}
          </span>
        </div>
        <div class="flex items-center gap-2">
          {#if filteredHistory.length > 0}
            <button
              class="text-xs text-[var(--muted)] hover:text-[var(--error-text)]"
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

      <!-- Profile Filter Select -->
      <div class="mb-3 flex items-center justify-between gap-2 border-b border-[var(--panel-border)] pb-3">
        <label for="history-profile-filter" class="text-xs font-medium text-[var(--muted)] shrink-0">
          Filter by Profile:
        </label>
        <select
          id="history-profile-filter"
          class="flex-1 rounded border border-[var(--panel-border)] bg-[var(--panel-inner)] px-2 py-1 text-xs text-[var(--text)] outline-none focus:border-[var(--accent-border)]"
          bind:value={selectedFilter}
        >
          <option value="__all__">All Profiles</option>
          <option value="__current__">
            Current Profile ({selectedProfile || 'None'})
          </option>
          {#each profiles as p (p.name)}
            {#if p.name !== selectedProfile}
              <option value={p.name}>{p.name}</option>
            {/if}
          {/each}
        </select>
      </div>

      {#if filteredHistory.length === 0}
        <div class="flex flex-1 items-center justify-center text-center text-sm text-[var(--muted)] px-4">
          {#if selectedFilter === '__current__'}
            No query history for current profile ({selectedProfile || 'None'}).
          {:else if selectedFilter === '__all__'}
            No query history recorded yet.
          {:else}
            No query history for profile "{selectedFilter}".
          {/if}
        </div>
      {:else}
        <div class="xsql-scroll flex-1 overflow-y-auto pr-1">
          <div class="grid gap-2.5">
            {#each filteredHistory as item (item.id)}
              <div
                role="button"
                tabindex="0"
                class="group flex flex-col gap-1.5 rounded-lg border border-[var(--panel-border)] bg-[var(--panel-inner)] p-3 transition hover:border-[var(--accent-border)] hover:shadow-sm cursor-pointer"
                onclick={() => handleSelect(item)}
                onkeydown={(e) => {
                  if (e.key === 'Enter') {
                    handleSelect(item);
                  }
                }}
              >
                <div class="flex items-center justify-between gap-2 text-xs">
                  <div class="flex items-center gap-1.5">
                    <span class="rounded bg-[var(--pill-bg)] px-1.5 py-0.5 font-medium text-[var(--pill-text)]">
                      {item.profile || 'default'}
                    </span>
                    {#if item.profile && item.profile !== selectedProfile}
                      <span class="text-[10px] text-[var(--accent-border)] font-semibold">
                        (Click to switch)
                      </span>
                    {/if}
                  </div>
                  <span class="text-[var(--muted)]">{formatTime(item.timestamp)}</span>
                </div>

                <pre class="m-0 max-h-20 overflow-hidden font-mono text-xs text-[var(--text)] whitespace-pre-wrap break-all">{item.sql}</pre>

                <div class="mt-1 flex items-center justify-between border-t border-[var(--table-border)] pt-1.5 text-[11px]">
                  {#if item.error}
                    <span class="text-[var(--error-text)] truncate max-w-[200px]">Error: {item.error}</span>
                  {:else}
                    <span class="text-[var(--muted)]">{item.rowCount} rows</span>
                  {/if}
                  <span class="font-mono text-[var(--muted)]">{item.durationMs}ms</span>
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </aside>
  </div>
{/if}
