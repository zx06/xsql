<script>
  let {
    isOpen = false,
    history = [],
    onClose,
    onSelectSQL,
    onClear
  } = $props();

  function formatTime(timestamp) {
    return timestamp || '';
  }
</script>

{#if isOpen}
  <div class="fixed inset-0 z-40 flex justify-end bg-black/40 backdrop-blur-xs transition-opacity">
    <div
      role="button"
      tabindex="-1"
      aria-label="Close modal overlay"
      class="absolute inset-0"
      onclick={onClose}
      onkeydown={(e) => e.key === 'Escape' && onClose()}
    ></div>
    <aside class="relative z-50 flex h-full w-96 max-w-full flex-col border-l border-[var(--panel-border)] bg-[var(--panel-bg)] p-4 shadow-2xl">
      <div class="mb-4 flex items-center justify-between border-b border-[var(--panel-border)] pb-3">
        <div class="flex items-center gap-2">
          <strong class="text-base text-[var(--text)]">Query History</strong>
          <span class="rounded-full bg-[var(--pill-bg)] px-2 py-0.5 text-xs text-[var(--pill-text)]">
            {history.length}
          </span>
        </div>
        <div class="flex items-center gap-2">
          {#if history.length > 0}
            <button
              class="text-xs text-[var(--muted)] hover:text-[var(--error-text)]"
              onclick={onClear}
            >
              Clear
            </button>
          {/if}
          <button
            class="rounded-lg p-1 text-[var(--muted)] hover:bg-[var(--accent-soft)] hover:text-[var(--text)]"
            onclick={onClose}
          >
            ✕
          </button>
        </div>
      </div>

      {#if history.length === 0}
        <div class="flex flex-1 items-center justify-center text-sm text-[var(--muted)]">
          No query history recorded yet.
        </div>
      {:else}
        <div class="xsql-scroll flex-1 overflow-y-auto pr-1">
          <div class="grid gap-2.5">
            {#each history as item (item.id)}
              <div
                role="button"
                tabindex="0"
                class="group flex flex-col gap-1.5 rounded-lg border border-[var(--panel-border)] bg-[var(--panel-inner)] p-3 transition hover:border-[var(--accent-border)] hover:shadow-sm"
                onclick={() => {
                  onSelectSQL?.(item.sql);
                  onClose?.();
                }}
                onkeydown={(e) => {
                  if (e.key === 'Enter') {
                    onSelectSQL?.(item.sql);
                    onClose?.();
                  }
                }}
              >
                <div class="flex items-center justify-between gap-2 text-xs">
                  <span class="rounded bg-[var(--pill-bg)] px-1.5 py-0.5 font-medium text-[var(--pill-text)]">
                    {item.profile}
                  </span>
                  <span class="text-[var(--muted)]">{formatTime(item.timestamp)}</span>
                </div>

                <pre class="m-0 max-h-20 overflow-hidden font-mono text-xs text-[var(--text)] whitespace-pre-wrap break-all">{item.sql}</pre>

                <div class="mt-1 flex items-center justify-between border-t border-[var(--table-border)] pt-1.5 text-[11px]">
                  {#if item.error}
                    <span class="text-[var(--error-text)] truncate max-w-[200px]">Error: {item.error}</span>
                  {:else}
                    <span class="text-[var(--muted)]">{item.rowCount} rows</span>
                  {/if}
                  <span class="font-mono text-[var(--muted)]">{item.durationMs}ms</span>
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </aside>
  </div>
{/if}
