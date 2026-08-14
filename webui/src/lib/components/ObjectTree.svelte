<script>
  let {
    selectedProfile = '',
    schemaLoading = false,
    tableCount = 0,
    schemaTables = [],
    selectedTable = null,
    onSelectTable
  } = $props();

  let filterText = $state('');
  let copiedTable = $state('');

  let filteredTables = $derived.by(() => {
    if (!filterText.trim()) return schemaTables;
    const q = filterText.trim().toLowerCase();
    return schemaTables.filter(
      (t) => t.name.toLowerCase().includes(q) || t.schema.toLowerCase().includes(q)
    );
  });

  // Group by schema
  let groupedSchemas = $derived.by(() => {
    const groups = {};
    for (const t of filteredTables) {
      if (!groups[t.schema]) {
        groups[t.schema] = [];
      }
      groups[t.schema].push(t);
    }
    return groups;
  });

  async function handleCopyTableName(event, fullName) {
    event.stopPropagation();
    try {
      await navigator.clipboard.writeText(fullName);
      copiedTable = fullName;
      setTimeout(() => {
        copiedTable = '';
      }, 1200);
    } catch {
      // ignore
    }
  }
</script>

<div class="flex min-h-0 flex-1 flex-col overflow-hidden">
  <!-- Search Input -->
  {#if selectedProfile !== '' && tableCount > 0}
    <div class="relative mb-2 shrink-0">
      <input
        type="text"
        placeholder="Filter tables..."
        class="xsql-input pl-7 pr-6 text-xs h-7"
        bind:value={filterText}
      />
      <svg class="absolute left-2 top-2 h-3.5 w-3.5 text-[var(--muted)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>
      </svg>
      {#if filterText}
        <button
          class="absolute right-2 top-1.5 text-xs text-[var(--muted)] hover:text-[var(--text)]"
          onclick={() => (filterText = '')}
        >
          ✕
        </button>
      {/if}
    </div>
  {/if}

  <!-- Tables List -->
  {#if selectedProfile === ''}
    <div class="flex flex-1 items-center justify-center p-4 text-center text-xs text-[var(--muted)]">
      Select a profile to load tables.
    </div>
  {:else if schemaLoading && tableCount === 0}
    <div class="flex flex-1 items-center justify-center gap-2 p-4 text-xs text-[var(--muted)]">
      <svg class="h-4 w-4 animate-spin text-[var(--accent)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/>
      </svg>
      <span>Loading tables…</span>
    </div>
  {:else if tableCount === 0}
    <div class="flex flex-1 items-center justify-center p-4 text-center text-xs text-[var(--muted)]">
      No tables found in this database.
    </div>
  {:else if filteredTables.length === 0}
    <div class="flex flex-1 items-center justify-center p-4 text-center text-xs text-[var(--muted)]">
      No tables matching "{filterText}"
    </div>
  {:else}
    <div class="xsql-scroll flex-1 overflow-y-auto pr-0.5">
      <div class="grid gap-2">
        {#each Object.keys(groupedSchemas) as schemaName (schemaName)}
          <div class="grid gap-0.5">
            {#if Object.keys(groupedSchemas).length > 1 || schemaName !== ''}
              <div class="sticky top-0 z-1 bg-[var(--panel-bg)]/90 backdrop-blur-xs px-1.5 py-1 text-[10px] font-bold uppercase tracking-wider text-[var(--muted)] flex items-center justify-between">
                <span>{schemaName || 'public'}</span>
                <span class="text-[9px] opacity-70">{groupedSchemas[schemaName].length}</span>
              </div>
            {/if}

            <ul class="grid gap-0.5">
              {#each groupedSchemas[schemaName] as table (`${table.schema}.${table.name}`)}
                {@const isSelected = selectedTable?.schema === table.schema && selectedTable?.name === table.name}
                {@const fullName = `${table.schema}.${table.name}`}
                <li>
                  <div
                    role="button"
                    tabindex="0"
                    class={[
                      'group flex w-full items-center justify-between gap-1.5 rounded-md px-2 py-1.5 text-left text-xs transition cursor-pointer',
                      isSelected
                        ? 'bg-[var(--accent-soft)] font-semibold text-[var(--accent)] border border-[var(--accent-border)]/30'
                        : 'text-[var(--text)] hover:bg-[var(--accent-soft)] hover:text-[var(--text)] border border-transparent'
                    ]}
                    onclick={() => onSelectTable?.(table)}
                    onkeydown={(e) => e.key === 'Enter' && onSelectTable?.(table)}
                  >
                    <div class="flex items-center gap-1.5 min-w-0">
                      <svg class={['h-3.5 w-3.5 shrink-0', isSelected ? 'text-[var(--accent)]' : 'text-[var(--muted)]']} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M3 3h18v18H3zM3 9h18M3 15h18M9 3v18"/>
                      </svg>
                      <span class="truncate font-mono text-[11px]" title={fullName}>
                        {table.name}
                      </span>
                    </div>

                    <div class="flex items-center gap-1 opacity-0 transition group-hover:opacity-100">
                      <button
                        type="button"
                        class="rounded p-0.5 text-[var(--muted)] hover:bg-[var(--panel-inner)] hover:text-[var(--text)]"
                        title="Copy Table Name"
                        onclick={(e) => handleCopyTableName(e, fullName)}
                      >
                        {#if copiedTable === fullName}
                          <svg class="h-3 w-3 text-emerald-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                            <polyline points="20 6 9 17 4 12"/>
                          </svg>
                        {:else}
                          <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>
                          </svg>
                        {/if}
                      </button>
                    </div>
                  </div>
                </li>
              {/each}
            </ul>
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>
