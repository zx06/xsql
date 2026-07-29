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

<aside class={['xsql-panel flex h-full min-h-0 flex-col gap-3 overflow-hidden p-3 transition-all', collapsed && 'w-14 p-2']}>
  <div class="flex items-center justify-between gap-2">
    {#if !collapsed}
      <div class="flex min-w-0 items-center gap-2">
        <span class="inline-flex items-center rounded-full bg-[var(--tag-bg)] px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider text-[var(--tag-text)]">
          xsql
        </span>
        <strong class="min-w-0 truncate text-xs text-[var(--text)]">{selectedProfile || 'No profile'}</strong>
        {#if selectedProfileMeta}
          <span class="inline-flex items-center rounded bg-[var(--pill-bg)] px-1.5 py-0.5 text-[10px] uppercase font-mono text-[var(--pill-text)]">
            {selectedProfileMeta.db}
          </span>
        {/if}
      </div>
    {/if}

    <div class="flex items-center gap-1">
      {#if !collapsed}
        <button
          class="xsql-button border-[var(--input-border)] bg-[var(--panel-inner)] px-2 py-1 text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
          title="Open Graphical Config Manager"
          onclick={onOpenConfig}
        >
          配置
        </button>
      {/if}
      <button
        class="xsql-button border-[var(--input-border)] bg-[var(--panel-inner)] p-1.5 text-xs text-[var(--muted)] hover:text-[var(--text)]"
        title={collapsed ? 'Expand Sidebar' : 'Collapse Sidebar'}
        onclick={onToggleCollapse}
      >
        {collapsed ? '➡️' : '⬅️'}
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
