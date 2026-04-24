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
</script>

<section class="flex min-h-0 flex-col overflow-hidden">
  <SectionHeader label="Objects" meta={schemaLoading ? 'Loading…' : `${tableCount} tables`} />

  {#if configPath}
    <p class="mb-3 truncate text-xs text-[var(--muted)]">{configPath}</p>
  {/if}

  {#if selectedProfile === ''}
    <p class="text-sm text-[var(--muted)]">Select a profile to load tables.</p>
  {:else if tableCount === 0 && !schemaLoading}
    <p class="text-sm text-[var(--muted)]">No schema data available.</p>
  {:else}
    <ul class="xsql-scroll grid min-h-0 flex-1 gap-1 overflow-y-auto overflow-x-hidden pr-1">
      {#each schemaTables as table (`${table.schema}.${table.name}`)}
        <li>
          <button
            class={[
              'flex min-w-0 w-full items-center justify-between gap-2 rounded-lg border px-3 py-2 text-left text-sm transition',
              selectedTable?.schema === table.schema && selectedTable?.name === table.name
                ? 'border-[var(--accent-border)] bg-[var(--accent-soft)] text-[var(--text)]'
                : 'border-transparent text-[var(--text)] hover:border-[var(--panel-border)] hover:bg-[var(--accent-soft)]'
            ]}
            onclick={() => onSelectTable?.(table)}
          >
            <span class="min-w-0 flex-1 truncate" title={`${table.schema}.${table.name}`}>{table.schema}.{table.name}</span>
            <span class="shrink-0 text-[11px] uppercase tracking-[0.12em] text-[var(--muted)]">
              {table.comment ? 'info' : ''}
            </span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</section>
