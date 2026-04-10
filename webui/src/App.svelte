<script>
  import { onMount } from 'svelte';

  import QueryEditor from './lib/components/QueryEditor.svelte';
  import ResultsTable from './lib/components/ResultsTable.svelte';
  import Sidebar from './lib/components/Sidebar.svelte';
  import StructureTable from './lib/components/StructureTable.svelte';
  import WorkspaceTabs from './lib/components/WorkspaceTabs.svelte';
  import { WebUIController } from './lib/web-ui.svelte.js';

  const ui = new WebUIController();

  onMount(() => {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    ui.setSystemPrefersDark(mediaQuery.matches);

    const handleThemeChange = (event) => {
      ui.setSystemPrefersDark(event.matches);
    };

    mediaQuery.addEventListener('change', handleThemeChange);
    void ui.initialize();

    return () => {
      mediaQuery.removeEventListener('change', handleThemeChange);
    };
  });
</script>

<svelte:head>
  <title>xsql web</title>
</svelte:head>

<div
  class={[
    'h-screen overflow-hidden bg-[var(--app-bg)] p-2 text-[var(--text)]',
    ui.resolvedTheme === 'black' ? 'theme-black' : 'theme-white'
  ]}
>
  <div class="grid h-full min-h-0 grid-cols-[17rem_minmax(0,1fr)] gap-2 overflow-hidden max-md:grid-cols-1 max-md:grid-rows-[minmax(18rem,40vh)_minmax(0,1fr)]">
    <Sidebar
      selectedProfile={ui.selectedProfile}
      selectedProfileMeta={ui.selectedProfileMeta}
      themeMode={ui.themeMode}
      profiles={ui.profiles}
      authRequired={ui.authRequired}
      authToken={ui.authToken}
      pageLoading={ui.pageLoading}
      configPath={ui.configPath}
      schemaLoading={ui.schemaLoading}
      tableCount={ui.tableCount}
      schemaTables={ui.schemaTables}
      selectedTable={ui.selectedTable}
      onThemeChange={(mode) => ui.setThemeMode(mode)}
      onProfileChange={(profileName) => ui.selectProfile(profileName)}
      onTokenChange={(token) => ui.setAuthToken(token)}
      onSelectTable={(table) => ui.previewTable(table)}
    />

    <main class="grid min-h-0 min-w-0 grid-rows-[minmax(10rem,15rem)_auto_minmax(0,1fr)] gap-2 overflow-hidden">
      <QueryEditor
        sql={ui.sql}
        queryLoading={ui.queryLoading}
        selectedProfile={ui.selectedProfile}
        sqlDialect={ui.sqlDialect}
        completionCatalog={ui.completionCatalog}
        onEnsureTableDetail={(schemaName, tableName) => ui.ensureCompletionTableDetail(schemaName, tableName)}
        onFormat={() => ui.formatSQL()}
        onGetTableDetail={(schemaName, tableName) => ui.getCompletionTableDetail(schemaName, tableName)}
        onRun={() => ui.runQuery()}
        onSqlChange={(sql) => ui.setSQL(sql)}
      />

      {#if ui.errorMessage}
        <section class="xsql-panel flex items-start gap-3 px-4 py-3 text-sm text-[var(--error-text)]">
          <strong class="shrink-0">Error</strong>
          <span class="min-w-0">{ui.errorMessage}</span>
        </section>
      {/if}

      <WorkspaceTabs activeTab={ui.activeTab} onTabChange={(tab) => ui.setActiveTab(tab)}>
        {#snippet results()}
          <ResultsTable
            pageLoading={ui.pageLoading}
            columns={ui.columns}
            rows={ui.rows}
            rowCount={ui.rowCount}
          />
        {/snippet}

        {#snippet structure()}
          <StructureTable
            selectedTable={ui.selectedTable}
            selectedTableDetail={ui.selectedTableDetail}
            selectedTableName={ui.selectedTableName}
            structureLoading={ui.structureLoading}
          />
        {/snippet}
      </WorkspaceTabs>
    </main>
  </div>
</div>
