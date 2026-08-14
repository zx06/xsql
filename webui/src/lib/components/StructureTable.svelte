<script>
  import { onDestroy, onMount } from 'svelte';
  import { EditorState } from '@codemirror/state';
  import { EditorView } from '@codemirror/view';

  import { copyText } from '../result-grid.js';
  import { createSQLLanguageSupport, sqlEditorBaseExtensions } from '../sql-editor.js';

  let {
    selectedTable = null,
    selectedTableDetail = null,
    selectedTableName = '',
    sqlDialect = 'sql',
    structureLoading = false,
    onGenerateQuery,
    onGenerateDDL
  } = $props();

  let activeView = $state('columns'); // 'columns' | 'ddl'
  let ddlCopied = $state(false);

  let ddlEditorHost = $state(null);
  let ddlEditorView = null;

  let currentDDL = $derived(selectedTableDetail ? onGenerateDDL?.(selectedTableDetail) || '' : '');

  function initOrUpdateDDLEditor(content) {
    if (!ddlEditorHost) return;

    if (!ddlEditorView) {
      ddlEditorView = new EditorView({
        parent: ddlEditorHost,
        state: EditorState.create({
          doc: content,
          extensions: [
            ...sqlEditorBaseExtensions,
            EditorView.editable.of(false),
            EditorState.readOnly.of(true),
            createSQLLanguageSupport(sqlDialect)
          ]
        })
      });
    } else {
      const cur = ddlEditorView.state.doc.toString();
      if (cur !== content) {
        ddlEditorView.dispatch({
          changes: { from: 0, to: cur.length, insert: content }
        });
      }
    }
  }

  $effect(() => {
    if (activeView === 'ddl' && currentDDL && ddlEditorHost) {
      initOrUpdateDDLEditor(currentDDL);
    }
  });

  onDestroy(() => {
    ddlEditorView?.destroy();
    ddlEditorView = null;
  });

  async function handleCopyDDL() {
    if (!selectedTableDetail) return;
    const ddl = onGenerateDDL?.(selectedTableDetail);
    if (!ddl) return;
    await copyText(ddl);
    ddlCopied = true;
    setTimeout(() => {
      ddlCopied = false;
    }, 1500);
  }
</script>

<div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
  <!-- Structure Header & Actions -->
  <div class="mb-2.5 flex items-center justify-between gap-3 shrink-0">
    <div class="flex items-center gap-2 min-w-0">
      <span class="text-xs font-bold uppercase tracking-wider text-[var(--text)]">Structure</span>
      {#if selectedTableName}
        <span class="truncate rounded bg-[var(--pill-bg)] px-2 py-0.5 font-mono text-xs font-semibold text-[var(--text)]">
          {selectedTableName}
        </span>
      {/if}
    </div>

    {#if selectedTableDetail}
      <div class="flex items-center gap-2">
        <!-- Sub tabs: Columns vs DDL -->
        <div class="flex rounded-lg border border-[var(--panel-border)] bg-[var(--panel-inner)] p-0.5">
          <button
            class={['rounded-md px-2.5 py-1 text-xs font-medium transition', activeView === 'columns' ? 'bg-[var(--panel-bg)] text-[var(--text)] shadow-xs font-semibold' : 'text-[var(--muted)] hover:text-[var(--text)]']}
            onclick={() => (activeView = 'columns')}
          >
            Columns ({selectedTableDetail.columns?.length || 0})
          </button>
          <button
            class={['rounded-md px-2.5 py-1 text-xs font-medium transition', activeView === 'ddl' ? 'bg-[var(--panel-bg)] text-[var(--text)] shadow-xs font-semibold' : 'text-[var(--muted)] hover:text-[var(--text)]']}
            onclick={() => (activeView = 'ddl')}
          >
            DDL Schema
          </button>
        </div>

        <!-- Quick Query buttons -->
        <button
          class="xsql-button px-2 py-1 text-xs text-[var(--text)]"
          title="Generate SELECT query for this table"
          onclick={() => onGenerateQuery?.(`SELECT * FROM ${selectedTableName} LIMIT 50;`)}
        >
          <span>Query Table</span>
        </button>
      </div>
    {/if}
  </div>

  <!-- Content Area -->
  {#if !selectedTable}
    <div class="flex flex-1 flex-col items-center justify-center p-8 text-center text-xs text-[var(--muted)]">
      <svg class="h-8 w-8 text-[var(--muted)] opacity-40 mb-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <path d="M3 3h18v18H3zM9 3v18M3 9h18"/>
      </svg>
      <p class="font-medium text-[var(--text-secondary)]">No table selected</p>
      <p class="mt-1 opacity-70">Select any table from the left schema sidebar to inspect its structure and DDL.</p>
    </div>
  {:else if structureLoading}
    <div class="flex flex-1 items-center justify-center p-8 text-xs text-[var(--muted)]">
      <svg class="h-4 w-4 animate-spin text-[var(--accent)] mr-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/>
      </svg>
      Loading table columns & schema…
    </div>
  {:else if selectedTableDetail}
    {#if activeView === 'columns'}
      <!-- Columns View -->
      <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
        {#if selectedTableDetail.comment}
          <div class="mb-2 rounded-lg bg-[var(--panel-inner)] px-3 py-1.5 text-xs text-[var(--muted)] italic">
            Table Comment: {selectedTableDetail.comment}
          </div>
        {/if}

        <div class="xsql-scroll min-h-0 flex-1 overflow-auto rounded-lg border border-[var(--table-border)] bg-[var(--table-bg)]">
          <table class="xsql-table xsql-table-compact">
            <thead>
              <tr>
                <th class="w-10 text-center">#</th>
                <th>Column Name</th>
                <th>Data Type</th>
                <th class="w-20">Nullable</th>
                <th class="w-20">Key</th>
                <th>Default / Details</th>
              </tr>
            </thead>
            <tbody>
              {#each selectedTableDetail.columns || [] as col, idx (col.name)}
                <tr class="hover:bg-[var(--table-row-hover)]">
                  <td class="text-center font-mono text-[10px] text-[var(--muted)] opacity-50 bg-[var(--table-head-bg)]/40">
                    {idx + 1}
                  </td>
                  <td class="font-mono font-semibold text-xs text-[var(--text)]">
                    <div class="flex items-center gap-1.5">
                      {#if col.primary_key}
                        <span class="text-amber-500" title="Primary Key">🔑</span>
                      {/if}
                      <span>{col.name}</span>
                    </div>
                  </td>
                  <td>
                    <span class="rounded bg-blue-500/10 px-1.5 py-0.5 font-mono text-[11px] text-blue-600 dark:text-blue-400 font-medium">
                      {col.type}
                    </span>
                  </td>
                  <td>
                    <span class={col.nullable ? 'text-[var(--muted)]' : 'font-semibold text-amber-600 dark:text-amber-400'}>
                      {col.nullable ? 'YES' : 'NO'}
                    </span>
                  </td>
                  <td>
                    {#if col.primary_key}
                      <span class="rounded bg-amber-500/15 px-1.5 py-0.5 font-mono text-[10px] font-bold text-amber-600 dark:text-amber-400">
                        PK
                      </span>
                    {:else}
                      <span class="text-[var(--muted)] opacity-40">-</span>
                    {/if}
                  </td>
                  <td class="text-xs text-[var(--muted)]">
                    {#if col.default}
                      <span class="font-mono text-[11px] text-[var(--text)] bg-[var(--panel-inner)] px-1 py-0.2 rounded">
                        {col.default}
                      </span>
                    {/if}
                    {#if col.comment}
                      <span class="italic ml-2">{col.comment}</span>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {:else}
      <!-- DDL View with CodeMirror SQL Highlighting -->
      <div class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-[var(--panel-border)] bg-[var(--editor-bg)]">
        <div class="flex items-center justify-between border-b border-[var(--panel-border)] bg-[var(--panel-inner)] px-3 py-1.5 shrink-0">
          <span class="text-xs font-mono text-[var(--muted)]">CREATE TABLE definition</span>
          <button
            class="xsql-button px-2.5 py-0.5 text-xs text-[var(--text)]"
            onclick={handleCopyDDL}
          >
            {ddlCopied ? '✓ Copied' : 'Copy DDL'}
          </button>
        </div>
        <div class="xsql-cm min-h-0 flex-1 overflow-hidden" bind:this={ddlEditorHost}></div>
      </div>
    {/if}
  {:else}
    <div class="flex flex-1 items-center justify-center p-8 text-center text-xs text-[var(--muted)]">
      Unable to load table structure details.
    </div>
  {/if}
</div>
