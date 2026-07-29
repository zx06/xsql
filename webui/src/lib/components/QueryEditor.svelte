<script>
  import { onDestroy, onMount } from 'svelte';

  import { Compartment, EditorState, Prec } from '@codemirror/state';
  import { EditorView, keymap } from '@codemirror/view';

  import SectionHeader from './SectionHeader.svelte';
  import {
    createSQLAutocompletion,
    createSQLLanguageSupport,
    sqlEditorBaseExtensions
  } from '../sql-editor.js';

  let {
    sql = '',
    queryLoading = false,
    selectedProfile = '',
    sqlDialect = 'sql',
    completionCatalog = null,
    lastExecutionMs = null,
    queryHistoryCount = 0,
    onEnsureTableDetail,
    onFormat,
    onGetTableDetail,
    onRun,
    onSqlChange,
    onToggleHistory
  } = $props();

  let editorHost = null;
  let editorView = null;
  let syncingExternalValue = false;

  const languageCompartment = new Compartment();
  const completionCompartment = new Compartment();
  const queryKeymap = Prec.highest(keymap.of([
    {
      key: 'Mod-Enter',
      preventDefault: true,
      run: () => {
        if (queryLoading || !selectedProfile) {
          return true;
        }
        onRun?.();
        return true;
      }
    },
    {
      key: 'Shift-Alt-f',
      preventDefault: true,
      run: () => {
        if (!sql.trim()) {
          return true;
        }
        onFormat?.();
        return true;
      }
    },
    {
      key: 'Mod-Shift-f',
      preventDefault: true,
      run: () => {
        if (!sql.trim()) {
          return true;
        }
        onFormat?.();
        return true;
      }
    }
  ]));

  function buildEditorExtensions() {
    return [
      ...sqlEditorBaseExtensions,
      queryKeymap,
      EditorView.updateListener.of((update) => {
        if (!update.docChanged || syncingExternalValue) {
          return;
        }
        onSqlChange?.(update.state.doc.toString());
      }),
      languageCompartment.of(createSQLLanguageSupport(sqlDialect)),
      completionCompartment.of(
        createSQLAutocompletion({
          dialectName: sqlDialect,
          getCatalog: () => completionCatalog,
          getTableDetail: (schemaName, tableName) => onGetTableDetail?.(schemaName, tableName) || null,
          ensureTableDetail: async (schemaName, tableName) => onEnsureTableDetail?.(schemaName, tableName) || null
        })
      )
    ];
  }

  function syncEditorDocument(nextValue) {
    if (!editorView) {
      return;
    }
    const currentValue = editorView.state.doc.toString();
    if (currentValue === nextValue) {
      return;
    }

    syncingExternalValue = true;
    editorView.dispatch({
      changes: {
        from: 0,
        to: currentValue.length,
        insert: nextValue
      }
    });
    syncingExternalValue = false;
  }

  function reconfigureEditor() {
    if (!editorView) {
      return;
    }
    editorView.dispatch({
      effects: [
        languageCompartment.reconfigure(createSQLLanguageSupport(sqlDialect)),
        completionCompartment.reconfigure(
          createSQLAutocompletion({
            dialectName: sqlDialect,
            getCatalog: () => completionCatalog,
            getTableDetail: (schemaName, tableName) => onGetTableDetail?.(schemaName, tableName) || null,
            ensureTableDetail: async (schemaName, tableName) => onEnsureTableDetail?.(schemaName, tableName) || null
          })
        )
      ]
    });
  }

  onMount(() => {
    editorView = new EditorView({
      parent: editorHost,
      state: EditorState.create({
        doc: sql,
        extensions: buildEditorExtensions()
      })
    });
  });

  onDestroy(() => {
    editorView?.destroy();
  });

  $effect(() => {
    syncEditorDocument(sql);
  });

  $effect(() => {
    sqlDialect;
    completionCatalog;
    onEnsureTableDetail;
    onGetTableDetail;
    reconfigureEditor();
  });
</script>

<section class="xsql-panel flex min-h-0 min-w-0 flex-col overflow-hidden p-3.5">
  <div class="mb-2.5 flex items-center justify-between gap-3">
    <div class="flex items-center gap-3">
      <SectionHeader label="SQL Editor" meta="read-only mode" />
      {#if lastExecutionMs !== null}
        <span class="rounded bg-[var(--pill-bg)] px-2 py-0.5 font-mono text-[11px] text-[var(--pill-text)]">
          {lastExecutionMs}ms
        </span>
      {/if}
    </div>

    <div class="flex shrink-0 items-center gap-2">
      <button
        class="xsql-button shrink-0 border-[var(--input-border)] bg-[var(--panel-inner)] px-2.5 py-1 text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
        onclick={() => onToggleHistory?.()}
        title="Open Query History"
      >
        📜 History ({queryHistoryCount})
      </button>

      <button
        class="xsql-button shrink-0 border-[var(--input-border)] bg-[var(--panel-inner)] px-2.5 py-1 text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
        onclick={() => onFormat?.()}
        disabled={!sql.trim()}
        title="Format SQL (Shift+Alt+F)"
      >
        Format
      </button>

      <button
        class="xsql-button xsql-button-primary shrink-0 px-3 py-1 text-xs"
        onclick={() => onRun?.()}
        disabled={queryLoading || !selectedProfile}
        title="Run query (Cmd/Ctrl+Enter)"
      >
        {queryLoading ? 'Running…' : '▶ Run'}
      </button>
    </div>
  </div>

  <div class="xsql-cm min-h-[5rem] flex-1 overflow-hidden rounded-lg border border-[var(--editor-border)] bg-[var(--editor-bg)]" bind:this={editorHost}></div>
</section>
