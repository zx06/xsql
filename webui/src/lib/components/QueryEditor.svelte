<script>
  import { onDestroy, onMount } from 'svelte';

  import { Compartment, EditorState, Prec } from '@codemirror/state';
  import { EditorView, keymap } from '@codemirror/view';

  import {
    createSQLAutocompletion,
    createSQLLanguageSupport,
    sqlEditorBaseExtensions
  } from '../sql-editor.js';
  import { getAvailableTemplates } from '../templates.js';

  let {
    sql = '',
    queryLoading = false,
    selectedProfile = '',
    sqlDialect = 'sql',
    completionCatalog = null,
    lastExecutionMs = null,
    queryHistoryCount = 0,
    isMaximized = false,
    onEnsureTableDetail,
    onFormat,
    onGetTableDetail,
    onRun,
    onSqlChange,
    onClear,
    onInsertSnippet,
    onToggleHistory,
    onToggleMaximize
  } = $props();

  let editorHost = null;
  let editorView = null;
  let syncingExternalValue = false;
  let snippetsMenuOpen = $state(false);
  let isMac = $state(true);

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
    if (typeof navigator !== 'undefined') {
      isMac = /Mac|iPhone|iPod|iPad/i.test(navigator.userAgent || navigator.platform || '');
    }

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

<section class="xsql-panel relative flex min-h-0 min-w-0 flex-1 flex-col p-3">
  <!-- Editor Action Bar -->
  <div class="mb-2 flex items-center justify-between gap-2 shrink-0">
    <div class="flex items-center gap-2">
      <div class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wider text-[var(--text)]">
        <svg class="h-3.5 w-3.5 text-[var(--accent)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
        </svg>
        <span>SQL Query</span>
      </div>

      {#if lastExecutionMs !== null}
        <span class="rounded bg-emerald-500/10 px-1.5 py-0.5 font-mono text-[10px] text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
          ⚡ {lastExecutionMs}ms
        </span>
      {/if}
    </div>

    <!-- Actions -->
    <div class="flex shrink-0 items-center gap-1.5">
      <!-- Snippets Menu -->
      <div class="relative">
        <button
          class="xsql-button px-2 py-1 text-xs"
          onclick={() => (snippetsMenuOpen = !snippetsMenuOpen)}
          title="Insert prebuilt SQL queries"
        >
          <svg class="h-3.5 w-3.5 text-[var(--muted)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/>
          </svg>
          <span>Templates ▾</span>
        </button>

        {#if snippetsMenuOpen}
          <!-- Backdrop overlay to handle outside click cleanly -->
          <div
            role="button"
            tabindex="-1"
            aria-label="Close templates dropdown"
            class="fixed inset-0 z-40"
            onclick={() => (snippetsMenuOpen = false)}
            onkeydown={(e) => e.key === 'Escape' && (snippetsMenuOpen = false)}
          ></div>

          <div
            role="menu"
            tabindex="0"
            class="xsql-scroll absolute right-0 top-full z-50 mt-1.5 max-h-80 flex w-60 flex-col overflow-y-auto rounded-xl border border-[var(--panel-border)] bg-[var(--panel-bg)] p-1.5 shadow-2xl backdrop-blur-md animate-fade-in"
          >
            <div class="px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-[var(--muted)] border-b border-[var(--panel-border)] mb-1 flex items-center justify-between">
              <span>{sqlDialect === 'postgresql' ? '🐘 PostgreSQL' : '🐬 MySQL'} Templates</span>
            </div>

            {#each getAvailableTemplates(sqlDialect) as item (item.id)}
              <button
                class="flex items-center justify-between gap-2 rounded-lg px-2.5 py-1.5 text-left text-xs text-[var(--text)] hover:bg-[var(--accent-soft)] transition"
                onclick={() => {
                  onInsertSnippet?.(item.id);
                  snippetsMenuOpen = false;
                }}
                title={item.description}
              >
                <span>{item.title}</span>
                {#if item.category}
                  <span class="text-[9px] text-[var(--muted)] font-mono uppercase bg-[var(--panel-inner)] px-1 py-0.2 rounded">
                    {item.category}
                  </span>
                {/if}
              </button>
            {/each}
          </div>
        {/if}
      </div>

      <!-- History -->
      <button
        class="xsql-button px-2 py-1 text-xs"
        onclick={() => onToggleHistory?.()}
        title="View query history"
      >
        <svg class="h-3.5 w-3.5 text-[var(--muted)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 14 14"/>
        </svg>
        <span>History ({queryHistoryCount})</span>
      </button>

      <!-- Format -->
      <button
        class="xsql-button px-2 py-1 text-xs"
        onclick={() => onFormat?.()}
        disabled={!sql.trim()}
        title={isMac ? "Format SQL (⌥+⇧+F)" : "Format SQL (Shift+Alt+F)"}
      >
        <svg class="h-3.5 w-3.5 text-[var(--muted)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <line x1="21" y1="10" x2="7" y2="10"/><line x1="21" y1="6" x2="3" y2="6"/><line x1="21" y1="14" x2="3" y2="14"/><line x1="21" y1="18" x2="7" y2="18"/>
        </svg>
        <span>Format</span>
      </button>

      <!-- Clear -->
      {#if sql.trim()}
        <button
          class="xsql-button px-2 py-1 text-xs text-[var(--muted)] hover:text-red-500"
          onclick={() => onClear?.()}
          title="Clear editor"
        >
          Clear
        </button>
      {/if}

      <!-- Maximize / Restore -->
      <button
        class="xsql-button px-2 py-1 text-xs text-[var(--muted)]"
        onclick={() => onToggleMaximize?.()}
        title={isMaximized ? 'Restore editor height' : 'Maximize editor'}
      >
        {#if isMaximized}
          <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="4 14 10 14 10 20"/><polyline points="20 10 14 10 14 4"/>
          </svg>
        {:else}
          <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="15 3 21 3 21 9"/><polyline points="9 21 3 21 3 15"/>
          </svg>
        {/if}
      </button>

      <!-- Run Button -->
      <button
        class="xsql-button xsql-button-primary px-3 py-1 text-xs font-semibold"
        onclick={() => onRun?.()}
        disabled={queryLoading || !selectedProfile}
        title={isMac ? "Run SQL query (⌘+Enter)" : "Run SQL query (Ctrl+Enter)"}
      >
        {#if queryLoading}
          <svg class="h-3.5 w-3.5 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <circle cx="12" cy="12" r="10" stroke-opacity="0.3"/><path d="M12 2a10 10 0 0 1 10 10"/>
          </svg>
          <span>Running…</span>
        {:else}
          <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="currentColor">
            <polygon points="5 3 19 12 5 21 5 3"/>
          </svg>
          <span>Run</span>
          <kbd class="hidden xl:inline text-[9px] opacity-75 bg-white/20 px-1 py-0.2 rounded font-mono">
            {isMac ? '⌘↵' : 'Ctrl+↵'}
          </kbd>
        {/if}
      </button>
    </div>
  </div>

  <!-- CodeMirror Container -->
  <div class="xsql-cm min-h-[4.5rem] flex-1 overflow-hidden rounded-lg border border-[var(--editor-border)] bg-[var(--editor-bg)]" bind:this={editorHost}></div>
</section>
