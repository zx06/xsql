<script>
  import { highlightJson, tryParseJson } from '../result-grid.js';

  let {
    data = null,
    onClose
  } = $props();

  let copied = $state(false);

  function formattedJson() {
    if (!data) return '';
    const parsed = tryParseJson(data);
    if (parsed !== null) {
      return JSON.stringify(parsed, null, 2);
    }
    if (typeof data === 'object') {
      return JSON.stringify(data, null, 2);
    }
    return String(data);
  }

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(formattedJson());
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
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-fade-in">
    <div
      role="button"
      tabindex="-1"
      aria-label="Close modal overlay"
      class="absolute inset-0"
      onclick={onClose}
      onkeydown={(e) => e.key === 'Escape' && onClose()}
    ></div>
    <div class="relative z-10 flex max-h-[85vh] w-full max-w-3xl flex-col rounded-xl border border-[var(--panel-border)] bg-[var(--panel-bg)] shadow-2xl overflow-hidden">
      <!-- Modal Header -->
      <div class="flex items-center justify-between border-b border-[var(--panel-border)] px-4 py-2.5 bg-[var(--panel-inner)]">
        <div class="flex items-center gap-2">
          <svg class="h-4 w-4 text-[var(--accent)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>
          </svg>
          <strong class="text-xs font-semibold text-[var(--text)]">JSON / Detail Inspector</strong>
        </div>
        <div class="flex items-center gap-2">
          <button
            class="xsql-button px-2.5 py-1 text-xs text-[var(--text)]"
            onclick={handleCopy}
          >
            {copied ? '✓ Copied' : 'Copy'}
          </button>
          <button
            class="rounded-lg p-1 text-[var(--muted)] hover:bg-[var(--accent-soft)] hover:text-[var(--text)]"
            onclick={onClose}
          >
            ✕
          </button>
        </div>
      </div>

      <!-- Content -->
      <div class="xsql-scroll flex-1 overflow-auto p-4 bg-[var(--editor-bg)]">
        <pre class="m-0 font-mono text-xs text-[var(--text)] whitespace-pre-wrap break-all leading-5">{@html highlightJson(formattedJson())}</pre>
      </div>
    </div>
  </div>
{/if}
