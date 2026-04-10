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
    onEnsureTableDetail,
    onFormat,
    onGetTableDetail,
    onRun,
    onSqlChange
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

<section class="xsql-panel flex min-h-0 min-w-0 flex-col overflow-hidden p-4">
  <div class="mb-3 flex items-start justify-between gap-3">
    <div class="min-w-0 flex-1">
      <SectionHeader label="Query" meta="read-only" />
    </div>
    <div class="flex shrink-0 items-center gap-2">
      <button
        class="xsql-button shrink-0 border-[var(--input-border)] bg-[var(--panel-inner)] text-[var(--text)] hover:bg-[var(--accent-soft)]"
        onclick={() => onFormat?.()}
        disabled={!sql.trim()}
        title="Format SQL (Shift+Alt+F)"
      >
        Format
      </button>
      <button
        class="xsql-button xsql-button-primary shrink-0"
        onclick={() => onRun?.()}
        disabled={queryLoading || !selectedProfile}
        title="Run query (Ctrl/Cmd+Enter)"
      >
        {queryLoading ? 'Running…' : 'Run'}
      </button>
    </div>
  </div>

  <div class="xsql-cm min-h-[7rem] flex-1 overflow-hidden" bind:this={editorHost}></div>
</section>
