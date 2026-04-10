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
    onThemeChange,
    onProfileChange,
    onTokenChange,
    onSelectTable
  } = $props();
</script>

<aside class="xsql-panel grid h-full min-h-0 min-w-0 grid-rows-[auto_auto_minmax(0,1fr)] gap-3 overflow-hidden p-3">
  <div class="flex min-w-0 flex-wrap items-center gap-2">
    <span class="inline-flex items-center rounded-full bg-[var(--tag-bg)] px-2 py-1 text-[10px] uppercase tracking-[0.16em] text-[var(--tag-text)]">
      xsql web
    </span>
    <strong class="min-w-0 truncate text-sm text-[var(--text)]">{selectedProfile || 'No profile selected'}</strong>
    {#if selectedProfileMeta}
      <span class="inline-flex items-center rounded-full bg-[var(--pill-bg)] px-2 py-1 text-[10px] uppercase tracking-[0.16em] text-[var(--pill-text)]">
        {selectedProfileMeta.db}
      </span>
    {/if}
  </div>

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
</aside>
