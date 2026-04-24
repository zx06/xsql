<script>
  import SectionHeader from './SectionHeader.svelte';

  let {
    selectedTable = null,
    selectedTableDetail = null,
    selectedTableName = '',
    structureLoading = false
  } = $props();
</script>

<section class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
  <SectionHeader label="Table Structure" meta={selectedTableName} />

  {#if selectedTable}
    {#if structureLoading}
      <p class="text-sm text-[var(--muted)]">Loading table structure…</p>
    {:else if selectedTableDetail}
      {#if selectedTableDetail.comment}
        <p class="mb-3 text-sm text-[var(--muted)]">{selectedTableDetail.comment}</p>
      {/if}
      <div class="xsql-scroll min-h-0 flex-1 overflow-auto rounded-lg border border-[var(--table-border)]">
        <table class="xsql-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Null</th>
              <th>Key</th>
              <th>Default / Comment</th>
            </tr>
          </thead>
          <tbody>
            {#each selectedTableDetail.columns || [] as column (column.name)}
              <tr>
                <td>{column.name}</td>
                <td>{column.type}</td>
                <td>{column.nullable ? 'YES' : 'NO'}</td>
                <td>{column.primary_key ? 'PK' : ''}</td>
                <td>{column.default || column.comment || ''}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p class="text-sm text-[var(--muted)]">Select a table to inspect its structure.</p>
    {/if}
  {:else}
    <p class="text-sm text-[var(--muted)]">Pick a table from the object tree.</p>
  {/if}
</section>
