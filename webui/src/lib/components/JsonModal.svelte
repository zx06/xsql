<script>
  let {
    data = null,
    onClose
  } = $props();

  let copied = $state(false);

  function formattedJson() {
    if (!data) return '';
    try {
      const parsed = typeof data === 'string' ? JSON.parse(data) : data;
      return JSON.stringify(parsed, null, 2);
    } catch {
      return String(data);
    }
  }

  function highlightJson(jsonStr) {
    if (!jsonStr) return '';
    const escaped = String(jsonStr)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');

    const jsonRegex = /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g;

    return escaped.replace(jsonRegex, (match) => {
      if (/^"/.test(match)) {
        if (/:$/.test(match)) {
          const key = match.slice(0, -1);
          return `<span class="json-key">${key}</span>:`;
        }
        return `<span class="json-string">${match}</span>`;
      }
      if (/true|false/.test(match)) {
        return `<span class="json-boolean">${match}</span>`;
      }
      if (/null/.test(match)) {
        return `<span class="json-null">${match}</span>`;
      }
      return `<span class="json-number">${match}</span>`;
    });
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
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-xs p-4">
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
        <strong class="text-sm text-[var(--text)]">Formatted JSON Preview</strong>
        <div class="flex items-center gap-2">
          <button
            class="xsql-button border-[var(--input-border)] bg-[var(--panel-inner)] px-2.5 py-1 text-xs text-[var(--text)] hover:bg-[var(--accent-soft)]"
            onclick={handleCopy}
          >
            {copied ? 'Copied' : 'Copy'}
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
        <pre class="m-0 font-mono text-xs text-[var(--text)] whitespace-pre-wrap break-all">{@html highlightJson(formattedJson())}</pre>
      </div>
    </div>
  </div>
{/if}
