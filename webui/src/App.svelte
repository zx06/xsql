<script>
  import { onMount } from 'svelte';

  import ConfigModal from './lib/components/ConfigModal.svelte';
  import JsonModal from './lib/components/JsonModal.svelte';
  import QueryEditor from './lib/components/QueryEditor.svelte';
  import QueryHistoryDrawer from './lib/components/QueryHistoryDrawer.svelte';
  import ResizableHandle from './lib/components/ResizableHandle.svelte';
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
  <title>xsql web - AI-first DB Console</title>
</svelte:head>

<div
  class={[
    'h-screen overflow-hidden bg-[var(--app-bg)] p-2 text-[var(--text)] transition-colors',
    ui.resolvedTheme === 'black' ? 'theme-black' : 'theme-white'
  ]}
>
  <div
    class="flex h-full min-h-0 min-w-0 overflow-hidden"
    style={`column-gap: 4px;`}
  >
    <!-- Sidebar Container -->
    <div style={`width: ${ui.sidebarCollapsed ? '3.5rem' : `${ui.sidebarWidth}px`}; shrink: 0; min-width: 0;`}>
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
        collapsed={ui.sidebarCollapsed}
        onThemeChange={(mode) => ui.setThemeMode(mode)}
        onProfileChange={(profileName) => ui.selectProfile(profileName)}
        onTokenChange={(token) => ui.setAuthToken(token)}
        onSelectTable={(table) => ui.previewTable(table)}
        onOpenConfig={() => ui.openConfigModal()}
        onToggleCollapse={() => ui.toggleSidebarCollapsed()}
      />
    </div>

    {#if !ui.sidebarCollapsed}
      <ResizableHandle
        direction="horizontal"
        onResize={(delta) => ui.setSidebarWidth(ui.sidebarWidth + delta)}
      />
    {/if}

    <!-- Main Workspace -->
    <main class="flex min-h-0 flex-1 flex-col gap-2 overflow-hidden min-w-0">
      <!-- Top SQL Editor Section -->
      <div style={`height: ${ui.editorHeight}px; shrink: 0; min-height: 80px; max-height: 60vh; display: flex; flex-direction: column;`}>
        <QueryEditor
          sql={ui.sql}
          queryLoading={ui.queryLoading}
          selectedProfile={ui.selectedProfile}
          sqlDialect={ui.sqlDialect}
          completionCatalog={ui.completionCatalog}
          lastExecutionMs={ui.lastExecutionMs}
          queryHistoryCount={ui.queryHistory.length}
          onEnsureTableDetail={(schemaName, tableName) => ui.ensureCompletionTableDetail(schemaName, tableName)}
          onFormat={() => ui.formatSQL()}
          onGetTableDetail={(schemaName, tableName) => ui.getCompletionTableDetail(schemaName, tableName)}
          onRun={() => ui.runQuery()}
          onSqlChange={(sql) => ui.setSQL(sql)}
          onToggleHistory={() => ui.toggleQueryHistory()}
        />
      </div>

      <ResizableHandle
        direction="vertical"
        onResize={(delta) => ui.setEditorHeight(ui.editorHeight + delta)}
      />

      {#if ui.errorMessage}
        <section class="xsql-panel flex items-center justify-between gap-3 px-4 py-2.5 text-xs text-[var(--error-text)] bg-[var(--error-bg)] shrink-0">
          <div class="flex items-center gap-2 min-w-0">
            <strong class="shrink-0 font-bold">Error:</strong>
            <span class="truncate">{ui.errorMessage}</span>
          </div>
          <button class="shrink-0 text-xs opacity-70 hover:opacity-100" onclick={() => (ui.errorMessage = '')}>✕</button>
        </section>
      {/if}

      <!-- Bottom Workspace Tabs & Results -->
      <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
        <WorkspaceTabs activeTab={ui.activeTab} onTabChange={(tab) => ui.setActiveTab(tab)}>
          {#snippet results()}
            <ResultsTable
              pageLoading={ui.pageLoading}
              columns={ui.columns}
              rows={ui.processedRows}
              rowCount={ui.rowCount}
              sortColumn={ui.sortColumn}
              sortDirection={ui.sortDirection}
              resultFilter={ui.resultFilter}
              lastExecutionMs={ui.lastExecutionMs}
              onToggleSortColumn={(col) => ui.toggleSortColumn(col)}
              onFilterChange={(q) => ui.setResultFilter(q)}
              onExport={(fmt) => ui.exportResults(fmt)}
              onOpenJsonModal={(data) => ui.openJsonModal(data)}
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
      </div>
    </main>
  </div>

  <!-- Drawers & Modals -->
  <QueryHistoryDrawer
    isOpen={ui.queryHistoryOpen}
    history={ui.queryHistory}
    profiles={ui.profiles}
    selectedProfile={ui.selectedProfile}
    onClose={() => (ui.queryHistoryOpen = false)}
    onSelectHistoryItem={(item) => ui.selectHistoryItem(item)}
    onSelectSQL={(sql) => ui.setSQL(sql)}
    onClear={(filter) => ui.clearQueryHistory(filter)}
  />

  <JsonModal
    data={ui.selectedJsonModalData}
    onClose={() => ui.closeJsonModal()}
  />

  <ConfigModal
    isOpen={ui.configModalOpen}
    configPath={ui.configPath}
    fullConfig={ui.fullConfig}
    loading={ui.configLoading}
    onClose={() => ui.closeConfigModal()}
    onSaveProfile={(name, data) => ui.saveProfileConfig(name, data)}
    onDeleteProfile={(name) => ui.deleteProfileConfig(name)}
    onSaveSSHProxy={(name, data) => ui.saveSSHProxyConfig(name, data)}
    onDeleteSSHProxy={(name) => ui.deleteSSHProxyConfig(name)}
  />
</div>
