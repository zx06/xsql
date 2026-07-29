<script>
  import { highlightJSON } from '../result-grid.js';

  let {
    data = null,
    onClose
  } = $props();

  let copied = $state(false);

  function rawText() {
    if (!data) return '';
    try {
      const parsed = typeof data === 'string' ? JSON.parse(data) : data;
      return JSON.stringify(parsed, null, 2);
    } catch {
      return String(data);
    }
  }

  let highlightedHtml = $derived(data ? highlightJSON(data) : '');

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(rawText());
      copied = true;
      setTimeout(() => {
        copied = false;
      }, 1500);
    } catch {
      // ignore
    }
  }
</script>

{#if data !== null}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4">
    <div
      role="button"
      tabindex="-1"
      aria-label="Close modal overlay"
      class="absolute inset-0"
      onclick={onClose}
      onkeydown={(e) => e.key === 'Escape' && onClose()}
    ></div>
    <div class="relative z-10 flex max-h-[85vh] w-full max-w-2xl flex-col rounded-xl border border-[var(--panel-border)] bg-[var(--panel-bg)] shadow-2xl overflow-hidden">
      <div class="flex items-center justify-between border-b border-[var(--panel-border)] px-4 py-3">
        <div class="flex items-center gap-2">
          <strong class="text-sm text-[var(--text)]">Formatted JSON Viewer</strong>
          <span class="rounded bg-[var(--pill-bg)] px-2 py-0.5 text-[10px] uppercase font-mono font-bold text-[var(--pill-text)]">
            JSON Highlighting
          </span>
        </div>
        <div class="flex items-center gap-2">
          <button
            class="xsql-button border-[var(--input-border)] bg-[var(--panel-inner)] px-2.5 py-1 text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
            onclick={handleCopy}
          >
            {copied ? 'Copied' : 'Copy JSON'}
          </button>
          <button
            class="rounded-lg p-1 text-[var(--muted)] hover:bg-[var(--accent-soft)] hover:text-[var(--text)]"
            onclick={onClose}
          >
            ✕
          </button>
        </div>
      </div>
      <div class="xsql-scroll flex-1 overflow-auto p-4 bg-[var(--editor-bg)]">
        <pre class="m-0 font-mono text-xs text-[var(--text)] whitespace-pre-wrap break-all">{@html highlightedHtml}</pre>
      </div>
    </div>
  </div>
{/if}
