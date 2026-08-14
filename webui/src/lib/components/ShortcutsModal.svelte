<script>
  import { onMount } from 'svelte';

  let { isOpen = false, onClose } = $props();

  let isMac = $state(true);

  onMount(() => {
    if (typeof navigator !== 'undefined') {
      isMac = /Mac|iPhone|iPod|iPad/i.test(navigator.userAgent || navigator.platform || '');
    }
  });

  const macShortcuts = [
    { key: '⌘ + Enter', desc: 'Execute current SQL query' },
    { key: '⌥ + ⇧ + F', desc: 'Format SQL query with dialect rules' },
    { key: '⌘ + ⇧ + F', desc: 'Alternative format SQL query' },
    { key: 'Double Click Cell', desc: 'Open formatted JSON / detail viewer' },
    { key: 'Esc', desc: 'Close open dialogs / Deselect cell' }
  ];

  const winShortcuts = [
    { key: 'Ctrl + Enter', desc: 'Execute current SQL query' },
    { key: 'Shift + Alt + F', desc: 'Format SQL query with dialect rules' },
    { key: 'Ctrl + Shift + F', desc: 'Alternative format SQL query' },
    { key: 'Double Click Cell', desc: 'Open formatted JSON / detail viewer' },
    { key: 'Esc', desc: 'Close open dialogs / Deselect cell' }
  ];

  let displayShortcuts = $derived(isMac ? macShortcuts : winShortcuts);
</script>

{#if isOpen}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-fade-in">
    <div
      role="button"
      tabindex="-1"
      aria-label="Close modal"
      class="absolute inset-0"
      onclick={onClose}
      onkeydown={(e) => e.key === 'Escape' && onClose()}
    ></div>
    <div class="relative z-10 w-full max-w-md rounded-xl border border-[var(--panel-border)] bg-[var(--panel-bg)] p-5 shadow-2xl">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-[var(--panel-border)] pb-3">
        <div class="flex items-center gap-2">
          <svg class="h-4 w-4 text-[var(--accent)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect width="18" height="12" x="3" y="6" rx="2"/><path d="M7 10h.01M12 10h.01M17 10h.01M7 14h.01M12 14h4"/>
          </svg>
          <strong class="text-sm font-semibold text-[var(--text)]">Keyboard Shortcuts</strong>
        </div>
        <button
          class="rounded-lg p-1 text-[var(--muted)] hover:bg-[var(--accent-soft)] hover:text-[var(--text)]"
          onclick={onClose}
        >
          ✕
        </button>
      </div>

      <!-- Shortcuts list -->
      <div class="mt-4 flex flex-col gap-2.5">
        {#each displayShortcuts as item}
          <div class="flex items-center justify-between gap-3 text-xs">
            <span class="text-[var(--text-secondary)]">{item.desc}</span>
            <kbd class="rounded border border-[var(--panel-border)] bg-[var(--panel-inner)] px-2 py-1 font-mono text-[11px] font-semibold text-[var(--text)] shadow-xs">
              {item.key}
            </kbd>
          </div>
        {/each}
      </div>

      <div class="mt-5 border-t border-[var(--panel-border)] pt-3 text-right">
        <button
          class="xsql-button px-3 py-1 text-xs"
          onclick={onClose}
        >
          Got it
        </button>
      </div>
    </div>
  </div>
{/if}
