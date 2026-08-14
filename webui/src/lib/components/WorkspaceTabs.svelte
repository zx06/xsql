<script>
  let {
    activeTab = 'results',
    rowCount = 0,
    columnCount = 0,
    selectedTableName = '',
    onTabChange,
    results,
    structure
  } = $props();
</script>

<section class="xsql-panel flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden p-3">
  <!-- Bottom Tabs Header -->
  <div class="mb-2 flex items-center justify-between border-b border-[var(--panel-border)] pb-2 shrink-0">
    <div class="flex items-center gap-1">
      <button
        class={['xsql-tab', activeTab === 'results' && 'xsql-tab-active']}
        onclick={() => onTabChange?.('results')}
      >
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect width="18" height="18" x="3" y="3" rx="2"/><path d="M3 9h18M3 15h18M9 3v18"/>
        </svg>
        <span>Data Results</span>
        {#if rowCount > 0}
          <span class="rounded bg-black/10 dark:bg-white/10 px-1 py-0.2 text-[10px] font-mono">
            {rowCount}
          </span>
        {/if}
      </button>

      <button
        class={['xsql-tab', activeTab === 'structure' && 'xsql-tab-active']}
        onclick={() => onTabChange?.('structure')}
      >
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
          <polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/>
        </svg>
        <span>Schema & DDL</span>
        {#if selectedTableName}
          <span class="max-w-[120px] truncate text-[10px] opacity-75 font-mono">
            ({selectedTableName})
          </span>
        {/if}
      </button>
    </div>
  </div>

  <!-- Tab Content -->
  {#if activeTab === 'results'}
    {@render results?.()}
  {:else}
    {@render structure?.()}
  {/if}
</section>
