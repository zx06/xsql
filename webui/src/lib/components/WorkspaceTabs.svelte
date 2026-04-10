<script>
  let {
    activeTab = 'results',
    onTabChange,
    results,
    structure
  } = $props();
</script>

<section class="xsql-panel flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden p-4">
  <div class="mb-4 flex items-center gap-2">
    <button
      class={['xsql-tab', activeTab === 'results' && 'xsql-tab-active']}
      onclick={() => onTabChange?.('results')}
    >
      Results
    </button>
    <button
      class={['xsql-tab', activeTab === 'structure' && 'xsql-tab-active']}
      onclick={() => onTabChange?.('structure')}
    >
      Structure
    </button>
  </div>

  {#if activeTab === 'results'}
    {@render results?.()}
  {:else}
    {@render structure?.()}
  {/if}
</section>
