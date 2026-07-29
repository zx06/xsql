<script>
  import ObjectTree from './ObjectTree.svelte';
  import ThemeProfileControls from './ThemeProfileControls.svelte';

  let {
    selectedProfile = '',
    selectedProfileMeta = null,
    themeMode = 'auto',
    profiles = [],
    authRequired = false,
    authToken = '',
    pageLoading = false,
    configPath = '',
    schemaLoading = false,
    tableCount = 0,
    schemaTables = [],
    selectedTable = null,
    collapsed = false,
    onThemeChange,
    onProfileChange,
    onTokenChange,
    onSelectTable,
    onOpenConfig,
    onToggleCollapse
  } = $props();
</script>

<aside class={['xsql-panel flex h-full min-h-0 flex-col gap-3 overflow-hidden transition-all', collapsed ? 'w-14 p-2 items-center' : 'p-3']}>
  <div class={['flex items-center justify-between gap-2 w-full', collapsed && 'flex-col justify-center items-center gap-3 pt-1']}>
    {#if !collapsed}
      <div class="flex min-w-0 flex-1 items-center gap-1.5 overflow-hidden">
        <span class="inline-flex shrink-0 items-center rounded-full bg-[var(--tag-bg)] px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider text-[var(--tag-text)]">
          xsql
        </span>
        <strong class="min-w-0 truncate text-xs text-[var(--text)]">{selectedProfile || 'No profile'}</strong>
        {#if selectedProfileMeta}
          <span class="inline-flex shrink-0 items-center rounded bg-[var(--pill-bg)] px-1.5 py-0.5 text-[10px] uppercase font-mono text-[var(--pill-text)]">
            {selectedProfileMeta.db}
          </span>
        {/if}
      </div>
    {:else}
      <span class="inline-flex items-center rounded-full bg-[var(--tag-bg)] px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-wider text-[var(--tag-text)]" title="xsql web">
        xsql
      </span>
    {/if}

    <div class={['flex shrink-0 items-center gap-1.5', collapsed && 'flex-col']}>
      {#if !collapsed}
        <button
          class="xsql-button shrink-0 whitespace-nowrap border-[var(--input-border)] bg-[var(--panel-inner)] px-2 py-1 text-xs text-[var(--text)] hover:bg-[var(--accent-soft)] flex items-center gap-1"
          title="Open Graphical Config Manager"
          onclick={onOpenConfig}
        >
          <svg class="h-3.5 w-3.5 shrink-0 text-[var(--muted)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.38a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
            <circle cx="12" cy="12" r="3"/>
          </svg>
          <span class="whitespace-nowrap">配置</span>
        </button>
      {:else}
        <button
          class="flex h-7 w-7 items-center justify-center rounded-lg border border-[var(--input-border)] bg-[var(--panel-inner)] text-[var(--muted)] transition hover:bg-[var(--accent-soft)] hover:text-[var(--text)]"
          title="Open Graphical Config Manager"
          onclick={onOpenConfig}
        >
          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.38a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
            <circle cx="12" cy="12" r="3"/>
          </svg>
        </button>
      {/if}

      <button
        class="flex h-7 w-7 items-center justify-center rounded-lg border border-[var(--input-border)] bg-[var(--panel-inner)] text-[var(--muted)] transition hover:bg-[var(--accent-soft)] hover:text-[var(--text)]"
        title={collapsed ? 'Expand Sidebar' : 'Collapse Sidebar'}
        onclick={onToggleCollapse}
      >
        {#if collapsed}
          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect width="18" height="18" x="3" y="3" rx="2"/>
            <path d="M9 3v18"/>
            <path d="m13 9 3 3-3 3"/>
          </svg>
        {:else}
          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect width="18" height="18" x="3" y="3" rx="2"/>
            <path d="M9 3v18"/>
            <path d="m15 9-3 3 3 3"/>
          </svg>
        {/if}
      </button>
    </div>
  </div>

  {#if !collapsed}
    <ThemeProfileControls
      {themeMode}
      {selectedProfile}
      {profiles}
      {authRequired}
      {authToken}
      disabled={pageLoading}
      {onThemeChange}
      {onProfileChange}
      {onTokenChange}
    />

    <ObjectTree
      {selectedProfile}
      {configPath}
      {schemaLoading}
      {tableCount}
      {schemaTables}
      {selectedTable}
      {onSelectTable}
    />
  {/if}
</aside>
