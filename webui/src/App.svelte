<script>
  import { onMount } from 'svelte';

  import ConfigModal from './lib/components/ConfigModal.svelte';
  import JsonModal from './lib/components/JsonModal.svelte';
  import Navbar from './lib/components/Navbar.svelte';
  import QueryEditor from './lib/components/QueryEditor.svelte';
  import QueryHistoryDrawer from './lib/components/QueryHistoryDrawer.svelte';
  import ResizableHandle from './lib/components/ResizableHandle.svelte';
  import ResultsTable from './lib/components/ResultsTable.svelte';
  import ShortcutsModal from './lib/components/ShortcutsModal.svelte';
  import Sidebar from './lib/components/Sidebar.svelte';
  import StructureTable from './lib/components/StructureTable.svelte';
  import Toast from './lib/components/Toast.svelte';
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
    'flex h-screen flex-col overflow-hidden bg-[var(--app-bg)] p-2.5 text-[var(--text)] transition-colors',
    ui.resolvedTheme === 'black' ? 'theme-black' : 'theme-white'
  ]}
>
  <!-- Top Navigation Bar -->
  <Navbar
    profiles={ui.profiles}
    selectedProfile={ui.selectedProfile}
    selectedProfileMeta={ui.selectedProfileMeta}
    themeMode={ui.themeMode}
    authRequired={ui.authRequired}
    authToken={ui.authToken}
    tableCount={ui.tableCount}
    schemaLoading={ui.schemaLoading}
    onProfileChange={(profileName) => ui.selectProfile(profileName)}
    onThemeChange={(mode) => ui.setThemeMode(mode)}
    onOpenConfig={() => ui.openConfigModal()}
    onOpenShortcuts={() => ui.openShortcutsModal()}
  />

  <!-- Main Workspace Area -->
  <div class="flex min-h-0 flex-1 overflow-hidden" style={`column-gap: 4px;`}>
    <!-- Sidebar Container -->
    <div style={`width: ${ui.sidebarCollapsed ? '3rem' : `${ui.sidebarWidth}px`}; shrink: 0; min-width: 0;`}>
      <Sidebar
        selectedProfile={ui.selectedProfile}
        selectedProfileMeta={ui.selectedProfileMeta}
        schemaLoading={ui.schemaLoading}
        tableCount={ui.tableCount}
        schemaTables={ui.schemaTables}
        selectedTable={ui.selectedTable}
        collapsed={ui.sidebarCollapsed}
        onSelectTable={(table) => ui.previewTable(table)}
        onRefreshTables={() => ui.loadTables()}
        onToggleCollapse={() => ui.toggleSidebarCollapsed()}
      />
    </div>

    {#if !ui.sidebarCollapsed}
      <ResizableHandle
        direction="horizontal"
        onResize={(delta) => ui.setSidebarWidth(ui.sidebarWidth + delta)}
      />
    {/if}

    <!-- Main Workspace (Editor + Results/Structure) -->
    <main class="flex min-h-0 flex-1 flex-col gap-2 overflow-hidden min-w-0">
      <!-- Top SQL Editor Section -->
      {#if !ui.editorMaximized}
        <div class="relative z-20" style={`height: ${ui.editorHeight}px; shrink: 0; min-height: 80px; max-height: 70vh; display: flex; flex-direction: column;`}>
          <QueryEditor
            sql={ui.sql}
            queryLoading={ui.queryLoading}
            selectedProfile={ui.selectedProfile}
            sqlDialect={ui.sqlDialect}
            completionCatalog={ui.completionCatalog}
            lastExecutionMs={ui.lastExecutionMs}
            queryHistoryCount={ui.queryHistory.length}
            isMaximized={ui.editorMaximized}
            onEnsureTableDetail={(schemaName, tableName) => ui.ensureCompletionTableDetail(schemaName, tableName)}
            onFormat={() => ui.formatSQL()}
            onGetTableDetail={(schemaName, tableName) => ui.getCompletionTableDetail(schemaName, tableName)}
            onRun={() => ui.runQuery()}
            onSqlChange={(sql) => ui.setSQL(sql)}
            onClear={() => ui.clearSQL()}
            onInsertSnippet={(type) => ui.insertSnippet(type)}
            onToggleHistory={() => ui.toggleQueryHistory()}
            onToggleMaximize={() => ui.toggleEditorMaximized()}
          />
        </div>

        <ResizableHandle
          direction="vertical"
          onResize={(delta) => ui.setEditorHeight(ui.editorHeight + delta)}
        />
      {:else}
        <!-- Fullscreen Editor View -->
        <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
          <QueryEditor
            sql={ui.sql}
            queryLoading={ui.queryLoading}
            selectedProfile={ui.selectedProfile}
            sqlDialect={ui.sqlDialect}
            completionCatalog={ui.completionCatalog}
            lastExecutionMs={ui.lastExecutionMs}
            queryHistoryCount={ui.queryHistory.length}
            isMaximized={ui.editorMaximized}
            onEnsureTableDetail={(schemaName, tableName) => ui.ensureCompletionTableDetail(schemaName, tableName)}
            onFormat={() => ui.formatSQL()}
            onGetTableDetail={(schemaName, tableName) => ui.getCompletionTableDetail(schemaName, tableName)}
            onRun={() => ui.runQuery()}
            onSqlChange={(sql) => ui.setSQL(sql)}
            onClear={() => ui.clearSQL()}
            onInsertSnippet={(type) => ui.insertSnippet(type)}
            onToggleHistory={() => ui.toggleQueryHistory()}
            onToggleMaximize={() => ui.toggleEditorMaximized()}
          />
        </div>
      {/if}

      <!-- Bottom Workspace Tabs & Results -->
      {#if !ui.editorMaximized}
        <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
          <WorkspaceTabs
            activeTab={ui.activeTab}
            rowCount={ui.rowCount}
            columnCount={ui.columns.length}
            selectedTableName={ui.selectedTableName}
            onTabChange={(tab) => ui.setActiveTab(tab)}
          >
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
                onCopyMarkdown={() => ui.copyAsMarkdown()}
                onCopyInsertSQL={() => ui.copyAsInsertSQL()}
                onOpenJsonModal={(data) => ui.openJsonModal(data)}
              />
            {/snippet}

            {#snippet structure()}
              <StructureTable
                selectedTable={ui.selectedTable}
                selectedTableDetail={ui.selectedTableDetail}
                selectedTableName={ui.selectedTableName}
                sqlDialect={ui.sqlDialect}
                structureLoading={ui.structureLoading}
                onGenerateQuery={(sql) => {
                  ui.setSQL(sql);
                  ui.runQuery();
                }}
                onGenerateDDL={(detail) => ui.generateDDL(detail)}
              />
            {/snippet}
          </WorkspaceTabs>
        </div>
      {/if}
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

  <ShortcutsModal
    isOpen={ui.shortcutsModalOpen}
    onClose={() => ui.closeShortcutsModal()}
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
    onSaveAI={(data) => ui.saveAIConfig(data)}
    onTestProfile={(name, data) => ui.testProfileConnection(name, data)}
    onTestSSHProxy={(name, data) => ui.testSSHProxyConnection(name, data)}
    onTestAI={(data) => ui.testAIConnection(data)}
  />

  <!-- Toast Container -->
  <Toast toasts={ui.toasts} onDismiss={(id) => ui.dismissToast(id)} />
</div>
