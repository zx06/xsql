<script>
  import { onDestroy, onMount } from 'svelte';

  import { buildSelectedResultCell, copyText, formatResultCellValue, isCellTruncated } from '../result-grid.js';
  import SectionHeader from './SectionHeader.svelte';

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
    onOpenJsonModal
  } = $props();

  let selectedCell = $state(null);
  let hoveredRow = $state(-1);
  let hoveredColumn = $state('');
  let copied = $state(false);
  let tooltip = $state(null);
  let copyResetTimer = null;
  let exportMenuOpen = $state(false);

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
    clearTooltip();
  }

  async function handleCopySelectedCell() {
    if (!selectedCell) {
      return;
    }

    try {
      await copyText(selectedCell.kind === 'null' ? 'NULL' : selectedCell.fullText);
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
      'block w-full overflow-hidden text-ellipsis whitespace-nowrap rounded-md px-2 py-1 text-left outline-none transition font-mono',
      isSelected && 'bg-[var(--accent-soft)] text-[var(--text)] ring-1 ring-[var(--accent-border)]',
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
      hoveredRow === rowIndex && 'bg-[color:var(--panel-inner)]'
    ];
  }

  function selectedValueClass() {
    if (selectedCell?.kind === 'json') {
      return 'font-mono text-[12px] leading-6 whitespace-pre-wrap break-all';
    }
    return 'whitespace-pre-wrap break-words';
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

<section class="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
  <!-- Controls Bar -->
  <div class="mb-2.5 flex items-center justify-between gap-3">
    <div class="flex items-center gap-3">
      <SectionHeader label="Result" meta={`${rows.length} rows`} />
      {#if lastExecutionMs !== null}
        <span class="text-xs font-mono text-[var(--muted)]">{lastExecutionMs}ms</span>
      {/if}
    </div>

    {#if columns.length > 0}
      <div class="flex items-center gap-2">
        <input
          type="text"
          placeholder="Search in results..."
          class="xsql-input px-2.5 py-1 text-xs w-48"
          value={resultFilter}
          oninput={(e) => onFilterChange?.(e.currentTarget.value)}
        />

        <div class="relative">
          <button
            class="xsql-button border-[var(--input-border)] bg-[var(--panel-inner)] px-2.5 py-1 text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
            onclick={() => (exportMenuOpen = !exportMenuOpen)}
          >
            📥 Export ▾
          </button>

          {#if exportMenuOpen}
            <div
              role="menu"
              tabindex="0"
              class="absolute right-0 top-full z-30 mt-1 flex w-36 flex-col rounded-lg border border-[var(--panel-border)] bg-[var(--panel-bg)] p-1 shadow-lg backdrop-blur-md"
              onmouseleave={() => (exportMenuOpen = false)}
            >
              <button
                class="rounded px-3 py-1.5 text-left text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
                onclick={() => {
                  onExport?.('csv');
                  exportMenuOpen = false;
                }}
              >
                Export CSV (.csv)
              </button>
              <button
                class="rounded px-3 py-1.5 text-left text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
                onclick={() => {
                  onExport?.('json');
                  exportMenuOpen = false;
                }}
              >
                Export JSON (.json)
              </button>
              <button
                class="rounded px-3 py-1.5 text-left text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
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

  {#if pageLoading}
    <p class="text-sm text-[var(--muted)] py-4">Loading UI state…</p>
  {:else if columns.length === 0}
    <p class="text-sm text-[var(--muted)] py-4">Run a query or click a table to preview data.</p>
  {:else}
    <div class={['grid min-h-0 min-w-0 flex-1 gap-3', selectedCell ? 'grid-rows-[minmax(0,1fr)_12rem]' : 'grid-rows-[minmax(0,1fr)]']}>
      <div class="xsql-scroll min-h-0 overflow-auto rounded-lg border border-[var(--table-border)] bg-[var(--table-bg)]">
        <table class="xsql-table xsql-table-compact">
          <thead>
            <tr>
              {#each columns as column (column)}
                <th class={headerClass(column)} onclick={() => onToggleSortColumn?.(column)}>
                  <div class="flex items-center justify-between gap-1">
                    <span class="truncate">{column}</span>
                    {#if sortColumn === column}
                      <span class="text-[10px]">{sortDirection === 'asc' ? '▲' : '▼'}</span>
                    {/if}
                  </div>
                </th>
              {/each}
            </tr>
          </thead>
          <tbody>
            {#each rows as row, rowIndex (rowIndex)}
              <tr class={rowClass(rowIndex)}>
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
                      {formatted.previewDisplay}
                    </button>
                  </td>
                {/each}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      {#if selectedCell}
        <section class="grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden rounded-lg border border-[var(--table-border)] bg-[color:var(--panel-inner)]">
          <div class="flex items-center justify-between gap-3 border-b border-[var(--table-border)] px-4 py-2.5">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <strong class="text-xs text-[var(--text)] font-mono">{selectedCell.columnName}</strong>
                <span class="rounded-full bg-[var(--pill-bg)] px-2 py-0.5 text-[10px] uppercase font-mono tracking-wider text-[var(--pill-text)]">
                  {selectedCell.kind}
                </span>
                <span class="text-xs text-[var(--muted)]">Row {selectedCell.rowIndex + 1}</span>
              </div>
            </div>
            <div class="flex items-center gap-2">
              {#if selectedCell.kind === 'json' || selectedCell.fullText.length > 50}
                <button
                  class="xsql-button shrink-0 border-[var(--input-border)] bg-[var(--panel-bg)] px-2.5 py-1 text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
                  onclick={() => onOpenJsonModal?.(selectedCell.fullText)}
                >
                  🔍 Format View
                </button>
              {/if}
              <button
                class="xsql-button shrink-0 border-[var(--input-border)] bg-[var(--panel-bg)] px-2.5 py-1 text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
                onclick={handleCopySelectedCell}
              >
                {copied ? 'Copied' : 'Copy'}
              </button>
              <button
                class="xsql-button shrink-0 border-[var(--input-border)] bg-[var(--panel-bg)] px-2.5 py-1 text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
                onclick={clearSelection}
              >
                Close
              </button>
            </div>
          </div>
          <div class="xsql-scroll min-h-0 overflow-auto px-4 py-3 text-xs text-[var(--text)]">
            {#if selectedCell.isEmptyString}
              <p class="italic text-[var(--muted)]">Empty string</p>
            {:else}
              <pre class={['m-0', selectedValueClass()]}>{selectedCell.fullText}</pre>
            {/if}
          </div>
        </section>
      {/if}
    </div>

    {#if tooltip}
      <div
        class="pointer-events-none fixed z-50 max-h-52 overflow-hidden rounded-lg border border-[var(--panel-border)] bg-[var(--panel-bg)] px-3 py-2 text-xs leading-5 text-[var(--text)] shadow-xl backdrop-blur-sm font-mono"
        style={`top:${tooltip.top}px;left:${tooltip.left}px;max-width:${tooltip.maxWidth}px;`}
      >
        <div class="xsql-scroll max-h-48 overflow-auto whitespace-pre-wrap break-words">{tooltip.content}</div>
      </div>
    {/if}
  {/if}
</section>
