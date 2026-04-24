<script>
  let { label, meta = '' } = $props();
</script>

<div class="mb-3 flex items-center justify-between gap-3 border-b border-[var(--panel-border)] pb-2">
  <span class="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--muted)]">{label}</span>
  {#if meta}
    <span class="min-w-0 truncate rounded-full bg-[var(--panel-inner)] px-2 py-0.5 text-[10px] tracking-[0.08em] text-[var(--muted)]">
      {meta}
    </span>
  {/if}
</div>
