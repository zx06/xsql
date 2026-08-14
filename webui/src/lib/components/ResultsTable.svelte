<script>
  import { onDestroy, onMount } from 'svelte';

  import {
    buildSelectedResultCell,
    copyText,
    formatResultCellValue,
    inferColumnType,
    isCellTruncated,
    tryParseJson
  } from '../result-grid.js';

  let {
    pageLoading = false,
    columns = [],
    rows = [],
    rowCount = 0,
    sortColumn = '',
    sortDirection = 'asc',
    resultFilter = '',
    lastExecutionMs = null,
    onToggleSortColumn,
    onFilterChange,
    onExport,
    onCopyMarkdown,
    onCopyInsertSQL,
    onOpenJsonModal
  } = $props();

  let selectedCell = $state(null);
  let hoveredRow = $state(-1);
  let hoveredColumn = $state('');
  let copied = $state(false);
  let tooltip = $state(null);
  let copyResetTimer = null;
  let exportMenuOpen = $state(false);
  let copyMenuOpen = $state(false);
  let cellViewMode = $state('formatted'); // 'formatted' | 'raw'

  // Column types map
  let columnTypes = $derived.by(() => {
    const types = {};
    for (const col of columns) {
      types[col] = inferColumnType(col, rows);
    }
    return types;
  });

  function clearTooltip() {
    tooltip = null;
  }

  function clearSelection() {
    selectedCell = null;
    copied = false;
  }

  function updateTooltipPosition(target, content) {
    const rect = target.getBoundingClientRect();
    const width = Math.min(420, Math.max(220, window.innerWidth - 24));
    const left = Math.min(Math.max(12, rect.left), window.innerWidth - width - 12);
    const estimatedHeight = 180;
    const preferredTop = rect.bottom + 10;
    const top = preferredTop + estimatedHeight > window.innerHeight
      ? Math.max(12, rect.top - estimatedHeight - 10)
      : preferredTop;

    tooltip = {
      top,
      left,
      maxWidth: width,
      content
    };
  }

  function handleCellEnter(event, rowIndex, columnName, value) {
    hoveredRow = rowIndex;
    hoveredColumn = columnName;

    const formatted = formatResultCellValue(value);
    if (!formatted.isLong || !isCellTruncated(event.currentTarget)) {
      clearTooltip();
      return;
    }

    updateTooltipPosition(event.currentTarget, formatted.tooltipText);
  }

  function handleCellLeave() {
    hoveredRow = -1;
    hoveredColumn = '';
    clearTooltip();
  }

  function handleCellClick(rowIndex, columnName, value) {
    selectedCell = buildSelectedResultCell({ rowIndex, columnName, value });
    copied = false;
    cellViewMode = 'formatted';
    clearTooltip();
  }

  async function handleCopySelectedCell() {
    if (!selectedCell) {
      return;
    }

    try {
      const textToCopy = cellViewMode === 'formatted' && selectedCell.kind === 'json'
        ? selectedCell.fullText
        : (selectedCell.kind === 'null' ? 'NULL' : String(selectedCell.raw ?? ''));
      await copyText(textToCopy);
      copied = true;
      clearTimeout(copyResetTimer);
      copyResetTimer = setTimeout(() => {
        copied = false;
      }, 1500);
    } catch {
      copied = false;
    }
  }

  function cellButtonClass(rowIndex, columnName) {
    const isSelected = selectedCell?.rowIndex === rowIndex && selectedCell?.columnName === columnName;
    const isHovered = hoveredRow === rowIndex || hoveredColumn === columnName;

    return [
      'block w-full overflow-hidden text-ellipsis whitespace-nowrap rounded px-1.5 py-0.5 text-left outline-none transition font-mono text-xs',
      isSelected && 'bg-[var(--accent-soft)] text-[var(--accent)] font-semibold ring-1 ring-[var(--accent-border)]',
      !isSelected && isHovered && 'bg-[var(--panel-inner)]'
    ];
  }

  function headerClass(columnName) {
    return [
      'cursor-pointer hover:bg-[var(--accent-soft)] transition select-none',
      hoveredColumn === columnName && 'text-[var(--text)]',
      sortColumn === columnName && 'text-[var(--accent)] font-bold'
    ];
  }

  function rowClass(rowIndex) {
    return [
      hoveredRow === rowIndex && 'bg-[var(--table-row-hover)]'
    ];
  }

  function resetResultState() {
    clearSelection();
    handleCellLeave();
  }

  onMount(() => {
    const handleKeyDown = (event) => {
      if (event.key !== 'Escape') {
        return;
      }
      if (!selectedCell) {
        return;
      }
      event.preventDefault();
      clearSelection();
      clearTooltip();
      exportMenuOpen = false;
      copyMenuOpen = false;
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  });

  onDestroy(() => {
    clearTimeout(copyResetTimer);
  });

  $effect(() => {
    columns;
    rows;

    clearTooltip();
    hoveredRow = -1;
    hoveredColumn = '';

    if (!selectedCell) {
      return;
    }
    if (selectedCell.rowIndex >= rows.length || !columns.includes(selectedCell.columnName)) {
      resetResultState();
      return;
    }

    const nextValue = rows[selectedCell.rowIndex]?.[selectedCell.columnName];
    const nextSelectedCell = buildSelectedResultCell({
      rowIndex: selectedCell.rowIndex,
      columnName: selectedCell.columnName,
      value: nextValue
    });
    if (
      nextSelectedCell.kind !== selectedCell.kind ||
      nextSelectedCell.previewText !== selectedCell.previewText ||
      nextSelectedCell.fullText !== selectedCell.fullText ||
      nextSelectedCell.isEmptyString !== selectedCell.isEmptyString
    ) {
      selectedCell = nextSelectedCell;
    }
  });
</script>

<div class="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
  <!-- Controls Bar -->
  <div class="mb-2 flex items-center justify-between gap-3 shrink-0">
    <div class="flex items-center gap-2">
      <span class="text-xs font-bold uppercase tracking-wider text-[var(--text)]">Query Results</span>
      <span class="rounded bg-[var(--pill-bg)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--pill-text)]">
        {rows.length} {rows.length === 1 ? 'row' : 'rows'}
        {#if columns.length > 0}
          · {columns.length} cols
        {/if}
      </span>
      {#if resultFilter && rowCount !== rows.length}
        <span class="text-[10px] text-[var(--muted)]">
          (filtered from {rowCount})
        </span>
      {/if}
    </div>

    {#if columns.length > 0}
      <div class="flex items-center gap-2">
        <!-- Result filter -->
        <div class="relative">
          <input
            type="text"
            placeholder="Search rows..."
            class="xsql-input pl-6 pr-6 text-xs h-7 w-40 sm:w-48"
            value={resultFilter}
            oninput={(e) => onFilterChange?.(e.currentTarget.value)}
          />
          <svg class="absolute left-2 top-2 h-3 w-3 text-[var(--muted)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>
          </svg>
          {#if resultFilter}
            <button
              class="absolute right-2 top-1.5 text-xs text-[var(--muted)] hover:text-[var(--text)]"
              onclick={() => onFilterChange?.('')}
            >
              ✕
            </button>
          {/if}
        </div>

        <!-- Copy Menu (Markdown Table / INSERT SQL) -->
        <div class="relative">
          <button
            class="xsql-button px-2.5 py-1 text-xs"
            onclick={() => {
              copyMenuOpen = !copyMenuOpen;
              exportMenuOpen = false;
            }}
            title="Copy formatted result"
          >
            <svg class="h-3.5 w-3.5 text-[var(--muted)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>
            </svg>
            <span>Copy ▾</span>
          </button>

          {#if copyMenuOpen}
            <div
              role="menu"
              tabindex="0"
              class="absolute right-0 top-full z-30 mt-1 flex w-44 flex-col rounded-xl border border-[var(--panel-border)] bg-[var(--panel-bg)] p-1 shadow-xl backdrop-blur-md animate-fade-in"
              onmouseleave={() => (copyMenuOpen = false)}
            >
              <button
                class="flex items-center gap-2 rounded-lg px-3 py-1.5 text-left text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
                onclick={() => {
                  onCopyMarkdown?.();
                  copyMenuOpen = false;
                }}
              >
                <span>📝 Markdown Table</span>
              </button>
              <button
                class="flex items-center gap-2 rounded-lg px-3 py-1.5 text-left text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
                onclick={() => {
                  onCopyInsertSQL?.();
                  copyMenuOpen = false;
                }}
              >
                <span>💾 SQL INSERT statements</span>
              </button>
            </div>
          {/if}
        </div>

        <!-- Export Menu -->
        <div class="relative">
          <button
            class="xsql-button px-2.5 py-1 text-xs"
            onclick={() => {
              exportMenuOpen = !exportMenuOpen;
              copyMenuOpen = false;
            }}
            title="Export results to file"
          >
            <svg class="h-3.5 w-3.5 text-[var(--muted)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>
            </svg>
            <span>Export ▾</span>
          </button>

          {#if exportMenuOpen}
            <div
              role="menu"
              tabindex="0"
              class="absolute right-0 top-full z-30 mt-1 flex w-40 flex-col rounded-xl border border-[var(--panel-border)] bg-[var(--panel-bg)] p-1 shadow-xl backdrop-blur-md animate-fade-in"
              onmouseleave={() => (exportMenuOpen = false)}
            >
              <button
                class="rounded-lg px-3 py-1.5 text-left text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
                onclick={() => {
                  onExport?.('csv');
                  exportMenuOpen = false;
                }}
              >
                Export CSV (.csv)
              </button>
              <button
                class="rounded-lg px-3 py-1.5 text-left text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
                onclick={() => {
                  onExport?.('json');
                  exportMenuOpen = false;
                }}
              >
                Export JSON (.json)
              </button>
              <button
                class="rounded-lg px-3 py-1.5 text-left text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
                onclick={() => {
                  onExport?.('tsv');
                  exportMenuOpen = false;
                }}
              >
                Export TSV (.tsv)
              </button>
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>

  <!-- Main Table & Cell Inspector Grid -->
  {#if pageLoading}
    <div class="flex flex-1 items-center justify-center p-8 text-xs text-[var(--muted)]">
      <svg class="h-4 w-4 animate-spin text-[var(--accent)] mr-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/>
      </svg>
      Loading…
    </div>
  {:else if columns.length === 0}
    <div class="flex flex-1 flex-col items-center justify-center p-8 text-center text-xs text-[var(--muted)]">
      <svg class="h-8 w-8 text-[var(--muted)] opacity-40 mb-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <rect width="18" height="18" x="3" y="3" rx="2"/><path d="M3 9h18M3 15h18M9 3v18"/>
      </svg>
      <p class="font-medium text-[var(--text-secondary)]">No active query results</p>
      <p class="mt-1 opacity-70">Execute a query above (⌘+Enter) or click any table in schema tree to preview data.</p>
    </div>
  {:else}
    <div class={['grid min-h-0 min-w-0 flex-1 gap-2', selectedCell ? 'grid-rows-[minmax(0,1fr)_12rem]' : 'grid-rows-[minmax(0,1fr)]']}>
      <!-- Scrollable Data Grid -->
      <div class="xsql-scroll min-h-0 overflow-auto rounded-lg border border-[var(--table-border)] bg-[var(--table-bg)]">
        <table class="xsql-table xsql-table-compact">
          <thead>
            <tr>
              <!-- Row number index column -->
              <th class="w-10 text-center font-mono text-[10px] text-[var(--muted)] opacity-60">#</th>
              {#each columns as column (column)}
                {@const type = columnTypes[column]}
                <th class={headerClass(column)} onclick={() => onToggleSortColumn?.(column)}>
                  <div class="flex items-center justify-between gap-1.5">
                    <div class="flex items-center gap-1 min-w-0">
                      <!-- Column type indicator icon -->
                      <span class="rounded bg-black/5 dark:bg-white/10 px-1 py-0.2 font-mono text-[9px] text-[var(--muted)] font-normal">
                        {#if type === 'number'}#
                        {:else if type === 'boolean'}☑
                        {:else if type === 'datetime' || type === 'date'}📅
                        {:else if type === 'json'}&#123;&#125;
                        {:else if type === 'id'}🔑
                        {:else}T
                        {/if}
                      </span>
                      <span class="truncate font-sans font-semibold text-xs text-[var(--text)]">{column}</span>
                    </div>
                    {#if sortColumn === column}
                      <span class="text-[10px] text-[var(--accent)] font-bold">{sortDirection === 'asc' ? '▲' : '▼'}</span>
                    {/if}
                  </div>
                </th>
              {/each}
            </tr>
          </thead>
          <tbody>
            {#each rows as row, rowIndex (rowIndex)}
              <tr class={rowClass(rowIndex)}>
                <!-- Row index -->
                <td class="text-center font-mono text-[10px] text-[var(--muted)] select-none opacity-50 bg-[var(--table-head-bg)]/40">
                  {rowIndex + 1}
                </td>
                {#each columns as column (column)}
                  {@const formatted = formatResultCellValue(row[column])}
                  <td>
                    <button
                      class={cellButtonClass(rowIndex, column)}
                      onclick={() => handleCellClick(rowIndex, column, row[column])}
                      ondblclick={() => onOpenJsonModal?.(row[column])}
                      onmouseenter={(event) => handleCellEnter(event, rowIndex, column, row[column])}
                      onmouseleave={handleCellLeave}
                    >
                      {#if formatted.kind === 'null'}
                        <span class="italic text-[var(--muted)] font-sans text-[11px]">NULL</span>
                      {:else if formatted.kind === 'boolean'}
                        <span class={formatted.raw ? 'text-emerald-500 font-semibold' : 'text-red-400 font-semibold'}>
                          {formatted.previewDisplay}
                        </span>
                      {:else if formatted.kind === 'number'}
                        <span class="text-amber-600 dark:text-amber-400">
                          {formatted.previewDisplay}
                        </span>
                      {:else if formatted.kind === 'json'}
                        <span class="text-blue-600 dark:text-blue-400 font-medium">
                          {formatted.previewDisplay}
                        </span>
                      {:else}
                        <span>{formatted.previewDisplay}</span>
                      {/if}
                    </button>
                  </td>
                {/each}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      <!-- Selected Cell Bottom Inspector Panel -->
      {#if selectedCell}
        <section class="grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden rounded-lg border border-[var(--panel-border)] bg-[var(--panel-inner)] shadow-md animate-fade-in">
          <div class="flex items-center justify-between gap-3 border-b border-[var(--panel-border)] px-3 py-1.5 bg-[var(--panel-bg)]">
            <div class="flex items-center gap-2 min-w-0">
              <span class="font-mono font-bold text-xs text-[var(--text)]">{selectedCell.columnName}</span>
              <span class="rounded bg-[var(--pill-bg)] px-1.5 py-0.2 font-mono text-[10px] uppercase font-bold text-[var(--pill-text)]">
                {selectedCell.kind}
              </span>
              <span class="text-[11px] text-[var(--muted)]">Row {selectedCell.rowIndex + 1}</span>

              {#if selectedCell.kind === 'json'}
                <div class="flex rounded-md border border-[var(--panel-border)] bg-[var(--panel-inner)] p-0.5 ml-2">
                  <button
                    class={['rounded px-2 py-0.5 text-[10px] font-medium transition', cellViewMode === 'formatted' ? 'bg-[var(--panel-bg)] text-[var(--accent)] font-semibold shadow-xs' : 'text-[var(--muted)] hover:text-[var(--text)]']}
                    onclick={() => (cellViewMode = 'formatted')}
                  >
                    Formatted JSON
                  </button>
                  <button
                    class={['rounded px-2 py-0.5 text-[10px] font-medium transition', cellViewMode === 'raw' ? 'bg-[var(--panel-bg)] text-[var(--text)] font-semibold shadow-xs' : 'text-[var(--muted)] hover:text-[var(--text)]']}
                    onclick={() => (cellViewMode = 'raw')}
                  >
                    Raw
                  </button>
                </div>
              {/if}
            </div>

            <div class="flex items-center gap-1.5">
              {#if selectedCell.kind === 'json' || selectedCell.fullText.length > 40}
                <button
                  class="xsql-button px-2 py-0.5 text-xs text-[var(--text)]"
                  onclick={() => onOpenJsonModal?.(selectedCell.raw ?? selectedCell.fullText)}
                >
                  🔍 Format View
                </button>
              {/if}
              <button
                class="xsql-button px-2 py-0.5 text-xs text-[var(--text)]"
                onclick={handleCopySelectedCell}
              >
                {copied ? '✓ Copied' : 'Copy'}
              </button>
              <button
                class="rounded p-1 text-[var(--muted)] hover:text-[var(--text)]"
                onclick={clearSelection}
              >
                ✕
              </button>
            </div>
          </div>

          <div class="xsql-scroll min-h-0 overflow-auto p-3 text-xs text-[var(--text)] bg-[var(--editor-bg)]">
            {#if selectedCell.isEmptyString}
              <span class="italic text-[var(--muted)]">Empty string</span>
            {:else if selectedCell.kind === 'json' && cellViewMode === 'formatted'}
              <pre class="m-0 font-mono text-[11px] leading-5 whitespace-pre-wrap break-all">{@html selectedCell.highlightedHtml}</pre>
            {:else}
              <pre class="m-0 font-mono text-[11px] leading-5 whitespace-pre-wrap break-all">{selectedCell.fullText}</pre>
            {/if}
          </div>
        </section>
      {/if}
    </div>

    <!-- Long Content Tooltip -->
    {#if tooltip}
      <div
        class="pointer-events-none fixed z-50 max-h-52 overflow-hidden rounded-xl border border-[var(--panel-border)] bg-[var(--panel-bg)] px-3 py-2 text-xs leading-5 text-[var(--text)] shadow-2xl backdrop-blur-md font-mono animate-fade-in"
        style={`top:${tooltip.top}px;left:${tooltip.left}px;max-width:${tooltip.maxWidth}px;`}
      >
        <div class="xsql-scroll max-h-48 overflow-auto whitespace-pre-wrap break-words">{tooltip.content}</div>
      </div>
    {/if}
  {/if}
</div>
