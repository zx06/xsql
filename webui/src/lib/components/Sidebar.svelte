<script>
  import ObjectTree from './ObjectTree.svelte';

  let {
    selectedProfile = '',
    selectedProfileMeta = null,
    schemaLoading = false,
    tableCount = 0,
    schemaTables = [],
    selectedTable = null,
    collapsed = false,
    onSelectTable,
    onRefreshTables,
    onToggleCollapse
  } = $props();
</script>

<aside class={['xsql-panel flex h-full min-h-0 flex-col overflow-hidden transition-all', collapsed ? 'w-12 p-2 items-center' : 'p-3']}>
  <!-- Sidebar Header -->
  <div class={['flex items-center justify-between border-b border-[var(--panel-border)] pb-2.5 w-full', collapsed && 'flex-col justify-center items-center gap-2 border-none pb-0']}>
    {#if !collapsed}
      <div class="flex items-center gap-2 min-w-0">
        <svg class="h-4 w-4 text-[var(--accent)] shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <ellipse cx="12" cy="5" rx="9" ry="3"/>
          <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
          <path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3"/>
        </svg>
        <span class="text-xs font-bold uppercase tracking-wider text-[var(--text)]">Schema</span>
        <span class="rounded-full bg-[var(--pill-bg)] px-1.5 py-0.2 text-[10px] font-mono text-[var(--muted)]">
          {tableCount}
        </span>
      </div>
    {/if}

    <div class="flex items-center gap-1">
      {#if !collapsed}
        <button
          class="flex h-6 w-6 items-center justify-center rounded-lg text-[var(--muted)] transition hover:bg-[var(--accent-soft)] hover:text-[var(--text)]"
          title="Refresh Tables"
          onclick={onRefreshTables}
          disabled={schemaLoading}
        >
          <svg class={['h-3.5 w-3.5', schemaLoading && 'animate-spin text-[var(--accent)]']} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/>
          </svg>
        </button>
      {/if}

      <button
        class="flex h-6 w-6 items-center justify-center rounded-lg text-[var(--muted)] transition hover:bg-[var(--accent-soft)] hover:text-[var(--text)]"
        title={collapsed ? 'Expand Sidebar' : 'Collapse Sidebar'}
        onclick={onToggleCollapse}
      >
        {#if collapsed}
          <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect width="18" height="18" x="3" y="3" rx="2"/><path d="M9 3v18"/><path d="m13 9 3 3-3 3"/>
          </svg>
        {:else}
          <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect width="18" height="18" x="3" y="3" rx="2"/><path d="M9 3v18"/><path d="m15 9-3 3 3 3"/>
          </svg>
        {/if}
      </button>
    </div>
  </div>

  <!-- Objects Content -->
  {#if !collapsed}
    <div class="flex min-h-0 flex-1 flex-col pt-2 overflow-hidden">
      <ObjectTree
        {selectedProfile}
        {schemaLoading}
        {tableCount}
        {schemaTables}
        {selectedTable}
        {onSelectTable}
      />
    </div>
  {:else}
    <div class="mt-4 flex flex-col items-center gap-3">
      <button
        class="flex h-8 w-8 items-center justify-center rounded-lg border border-[var(--panel-border)] text-[var(--muted)] hover:text-[var(--accent)] hover:bg-[var(--accent-soft)]"
        title="Expand to view tables"
        onclick={onToggleCollapse}
      >
        <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M3 3h18v18H3zM9 3v18M14 9l3 3-3 3"/>
        </svg>
      </button>
    </div>
  {/if}
</aside>
