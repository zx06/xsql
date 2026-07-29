<script>
  import SectionHeader from './SectionHeader.svelte';

  let {
    selectedProfile = '',
    configPath = '',
    schemaLoading = false,
    tableCount = 0,
    schemaTables = [],
    selectedTable = null,
    onSelectTable
  } = $props();

  let filterText = $state('');

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

  let copiedTable = $state('');

  async function handleCopyTableName(event, tableName) {
    event.stopPropagation();
    try {
      await navigator.clipboard.writeText(tableName);
      copiedTable = tableName;
      setTimeout(() => {
        copiedTable = '';
      }, 1200);
    } catch {
      // ignore
    }
  }
</script>

<section class="flex min-h-0 flex-1 flex-col overflow-hidden">
  <SectionHeader label="Objects" meta={schemaLoading ? 'Loading…' : `${filteredTables.length}/${tableCount} tables`} />

  {#if selectedProfile !== '' && tableCount > 0}
    <div class="my-2">
      <input
        type="text"
        placeholder="Filter tables..."
        class="xsql-input px-2.5 py-1 text-xs"
        bind:value={filterText}
      />
    </div>
  {/if}

  {#if selectedProfile === ''}
    <p class="text-xs text-[var(--muted)] py-2">Select a profile to load tables.</p>
  {:else if tableCount === 0 && !schemaLoading}
    <p class="text-xs text-[var(--muted)] py-2">No schema data available.</p>
  {:else if filteredTables.length === 0}
    <p class="text-xs text-[var(--muted)] py-2">No tables match filter.</p>
  {:else}
    <div class="xsql-scroll flex-1 overflow-y-auto pr-1">
      <div class="grid gap-3">
        {#each Object.keys(groupedSchemas) as schemaName (schemaName)}
          <div class="grid gap-1">
            {#if Object.keys(groupedSchemas).length > 1 || schemaName !== ''}
              <div class="px-1 text-[11px] font-semibold uppercase tracking-wider text-[var(--muted)]">
                {schemaName || 'default'} ({groupedSchemas[schemaName].length})
              </div>
            {/if}

            <ul class="grid gap-1">
              {#each groupedSchemas[schemaName] as table (`${table.schema}.${table.name}`)}
                <li>
                  <div
                    role="button"
                    tabindex="0"
                    class={[
                      'group flex w-full items-center justify-between gap-1.5 rounded-lg border px-2.5 py-1.5 text-left text-xs transition cursor-pointer',
                      selectedTable?.schema === table.schema && selectedTable?.name === table.name
                        ? 'border-[var(--accent-border)] bg-[var(--accent-soft)] font-medium text-[var(--text)]'
                        : 'border-transparent text-[var(--text)] hover:border-[var(--panel-border)] hover:bg-[var(--accent-soft)]'
                    ]}
                    onclick={() => onSelectTable?.(table)}
                    onkeydown={(e) => e.key === 'Enter' && onSelectTable?.(table)}
                  >
                    <span class="truncate font-mono" title={`${table.schema}.${table.name}`}>
                      {table.name}
                    </span>

                    <div class="flex items-center gap-1 opacity-0 transition group-hover:opacity-100">
                      <span
                        role="button"
                        tabindex="0"
                        class="rounded px-1.5 py-0.5 text-[10px] text-[var(--muted)] hover:bg-[var(--panel-bg)] hover:text-[var(--text)] cursor-pointer"
                        title="Copy Table Name"
                        onclick={(e) => handleCopyTableName(e, `${table.schema}.${table.name}`)}
                        onkeydown={(e) => e.key === 'Enter' && handleCopyTableName(e, `${table.schema}.${table.name}`)}
                      >
                        {copiedTable === `${table.schema}.${table.name}` ? '✓' : '📋'}
                      </span>
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
</section>
