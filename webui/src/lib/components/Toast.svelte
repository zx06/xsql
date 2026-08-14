<script>
  let { toasts = [], onDismiss } = $props();
</script>

{#if toasts.length > 0}
  <div class="fixed bottom-5 right-5 z-50 flex flex-col gap-2 pointer-events-none max-w-sm w-full">
    {#each toasts as toast (toast.id)}
      <div
        class={[
          'pointer-events-auto flex items-center justify-between gap-3 rounded-xl border px-4 py-3 text-xs shadow-xl backdrop-blur-md animate-fade-in transition-all',
          toast.type === 'error'
            ? 'border-[var(--error-border)] bg-[var(--error-bg)] text-[var(--error-text)]'
            : toast.type === 'success'
            ? 'border-[var(--success-border)] bg-[var(--success-bg)] text-[var(--success-text)]'
            : 'border-[var(--panel-border)] bg-[var(--panel-bg)] text-[var(--text)]'
        ]}
      >
        <div class="flex items-center gap-2 min-w-0">
          {#if toast.type === 'error'}
            <svg class="h-4 w-4 shrink-0 text-red-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
            </svg>
          {:else if toast.type === 'success'}
            <svg class="h-4 w-4 shrink-0 text-emerald-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="20 6 9 17 4 12"/>
            </svg>
          {:else}
            <svg class="h-4 w-4 shrink-0 text-blue-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/>
            </svg>
          {/if}
          <span class="truncate font-medium">{toast.message}</span>
        </div>
        <button
          class="shrink-0 rounded p-1 opacity-60 hover:opacity-100 hover:bg-black/5 dark:hover:bg-white/10"
          onclick={() => onDismiss?.(toast.id)}
          aria-label="Dismiss toast"
        >
          ✕
        </button>
      </div>
    {/each}
  </div>
{/if}
