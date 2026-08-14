import { formatSQLQuery, resolveSQLDialectName } from './sql-editor.js';
import { copyText, exportToInsertSQL, exportToMarkdownTable } from './result-grid.js';
import { renderTemplateById } from './templates.js';

function readSessionValue(key) {
  if (typeof sessionStorage === 'undefined') {
    return '';
  }
  return sessionStorage.getItem(key) || '';
}

function readThemeMode() {
  if (typeof localStorage === 'undefined') {
    return 'auto';
  }
  const storedTheme = localStorage.getItem('xsql-web-theme');
  if (storedTheme === 'auto' || storedTheme === 'white' || storedTheme === 'black') {
    return storedTheme;
  }
  return 'auto';
}

function readNumberStorage(key, defaultValue) {
  if (typeof localStorage === 'undefined') {
    return defaultValue;
  }
  const val = Number(localStorage.getItem(key));
  return Number.isFinite(val) && val > 0 ? val : defaultValue;
}

function readBoolStorage(key, defaultValue = false) {
  if (typeof localStorage === 'undefined') {
    return defaultValue;
  }
  return localStorage.getItem(key) === 'true';
}

function readHistoryStorage() {
  if (typeof localStorage === 'undefined') {
    return [];
  }
  try {
    const raw = localStorage.getItem('xsql-web-query-history');
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function saveHistoryStorage(history) {
  if (typeof localStorage === 'undefined') {
    return;
  }
  try {
    localStorage.setItem('xsql-web-query-history', JSON.stringify(history.slice(0, 50)));
  } catch {
    // ignore
  }
}

function formatTableName(table) {
  return `${table.schema}.${table.name}`;
}

function buildPreviewSQL(table) {
  return `SELECT * FROM ${formatTableName(table)} LIMIT 20;`;
}

function createEmptyCompletionCatalog(profile = '') {
  return {
    profile,
    defaultSchema: '',
    activeSchema: '',
    tables: [],
    tablesBySchema: {},
    tableSchemas: {}
  };
}

function buildCompletionCatalog(profile, tables, activeSchema = '') {
  const sortedTables = [...tables].sort((left, right) => formatTableName(left).localeCompare(formatTableName(right)));
  const tablesBySchema = {};
  const tableSchemas = {};

  for (const table of sortedTables) {
    if (!tablesBySchema[table.schema]) {
      tablesBySchema[table.schema] = [];
    }
    tablesBySchema[table.schema].push(table);

    if (!tableSchemas[table.name]) {
      tableSchemas[table.name] = [];
    }
    tableSchemas[table.name].push(table.schema);
  }

  const schemaNames = Object.keys(tablesBySchema).sort((left, right) => left.localeCompare(right));
  const defaultSchema = schemaNames.length === 1 ? schemaNames[0] : '';

  return {
    profile,
    defaultSchema,
    activeSchema: activeSchema || defaultSchema,
    tables: sortedTables,
    tablesBySchema,
    tableSchemas
  };
}

export class WebUIController {
  authRequired = $state(false);
  authToken = $state(readSessionValue('xsql-web-auth-token'));
  profiles = $state.raw([]);
  selectedProfile = $state('');
  schemaTables = $state.raw([]);
  selectedTable = $state(null);
  selectedTableDetail = $state(null);
  schemaLoading = $state(false);
  structureLoading = $state(false);
  queryLoading = $state(false);
  pageLoading = $state(true);
  errorMessage = $state('');
  sql = $state('SELECT 1;');
  columns = $state.raw([]);
  rows = $state.raw([]);
  configPath = $state('');
  activeTab = $state('results'); // 'results' | 'structure' | 'ddl'
  themeMode = $state(readThemeMode());
  systemPrefersDark = $state(false);
  completionCatalog = $state.raw(createEmptyCompletionCatalog());

  // Layout & Resizing
  sidebarWidth = $state(readNumberStorage('xsql-web-sidebar-width', 260));
  editorHeight = $state(readNumberStorage('xsql-web-editor-height', 200));
  sidebarCollapsed = $state(readBoolStorage('xsql-web-sidebar-collapsed', false));
  editorMaximized = $state(false);

  // Query History & Timing
  queryHistory = $state(readHistoryStorage());
  queryHistoryOpen = $state(false);
  lastExecutionMs = $state(null);

  // Modals & Drawers
  configModalOpen = $state(false);
  configLoading = $state(false);
  fullConfig = $state(null);
  shortcutsModalOpen = $state(false);
  selectedJsonModalData = $state(null);

  // Toasts Notification System
  toasts = $state([]);

  // Results Grid enhancements: Filter & Sort
  resultFilter = $state('');
  sortColumn = $state('');
  sortDirection = $state('asc'); // 'asc' | 'desc'

  rowCount = $derived(this.rows.length);
  tableCount = $derived(this.schemaTables.length);
  selectedProfileMeta = $derived(this.profiles.find((profile) => profile.name === this.selectedProfile) ?? null);
  selectedTableName = $derived(this.selectedTable ? formatTableName(this.selectedTable) : '');
  resolvedTheme = $derived(this.themeMode === 'auto' ? (this.systemPrefersDark ? 'black' : 'white') : this.themeMode);
  sqlDialect = $derived(
    resolveSQLDialectName(
      this.selectedProfileMeta?.db,
      this.selectedProfile,
      this.schemaTables[0]?.schema
    )
  );

  // Processed Rows (Filtered & Sorted)
  processedRows = $derived.by(() => {
    let result = this.rows;
    if (this.resultFilter.trim()) {
      const q = this.resultFilter.trim().toLowerCase();
      result = result.filter((row) =>
        Object.values(row).some((val) => String(val ?? '').toLowerCase().includes(q))
      );
    }
    if (this.sortColumn && this.columns.includes(this.sortColumn)) {
      const col = this.sortColumn;
      const dir = this.sortDirection === 'desc' ? -1 : 1;
      result = [...result].sort((a, b) => {
        const valA = a[col];
        const valB = b[col];
        if (valA === valB) return 0;
        if (valA === null || valA === undefined) return 1;
        if (valB === null || valB === undefined) return -1;
        if (typeof valA === 'number' && typeof valB === 'number') {
          return (valA - valB) * dir;
        }
        return String(valA).localeCompare(String(valB)) * dir;
      });
    }
    return result;
  });

  #schemaRequestSeq = 0;
  #structureRequestSeq = 0;
  #tableDetailCache = new Map();
  #tableDetailRequestCache = new Map();

  #toastCounter = 0;

  showToast(message, type = 'info', duration = 3000) {
    this.#toastCounter += 1;
    const id = `toast-${this.#toastCounter}-${Date.now()}`;
    this.toasts = [...this.toasts, { id, message, type }];
    if (duration > 0) {
      setTimeout(() => {
        this.dismissToast(id);
      }, duration);
    }
  }

  dismissToast(id) {
    this.toasts = this.toasts.filter((t) => t.id !== id);
  }

  authHeaders() {
    const headers = { 'Content-Type': 'application/json' };
    const token = this.authToken.trim();
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }
    return headers;
  }

  async api(path, init = {}) {
    const response = await fetch(path, {
      ...init,
      headers: {
        ...this.authHeaders(),
        ...(init.headers || {})
      }
    });

    const payload = await response.json().catch(() => null);
    if (!response.ok || !payload?.ok) {
      const code = payload?.error?.code ? ` [${payload.error.code}]` : '';
      throw new Error(`${payload?.error?.message || 'Request failed'}${code}`);
    }
    return payload.data ?? payload;
  }

  setThemeMode(mode) {
    this.themeMode = mode;
    localStorage.setItem('xsql-web-theme', mode);
  }

  setSystemPrefersDark(matches) {
    this.systemPrefersDark = matches;
  }

  setAuthToken(token) {
    this.authToken = token;
    sessionStorage.setItem('xsql-web-auth-token', token.trim());
  }

  setSidebarWidth(width) {
    const nextWidth = Math.max(180, Math.min(500, width));
    this.sidebarWidth = nextWidth;
    localStorage.setItem('xsql-web-sidebar-width', String(nextWidth));
  }

  setEditorHeight(height) {
    const nextHeight = Math.max(100, Math.min(600, height));
    this.editorHeight = nextHeight;
    localStorage.setItem('xsql-web-editor-height', String(nextHeight));
  }

  toggleSidebarCollapsed() {
    this.sidebarCollapsed = !this.sidebarCollapsed;
    localStorage.setItem('xsql-web-sidebar-collapsed', String(this.sidebarCollapsed));
  }

  toggleEditorMaximized() {
    this.editorMaximized = !this.editorMaximized;
  }

  toggleQueryHistory() {
    this.queryHistoryOpen = !this.queryHistoryOpen;
  }

  openShortcutsModal() {
    this.shortcutsModalOpen = true;
  }

  closeShortcutsModal() {
    this.shortcutsModalOpen = false;
  }

  setSQL(sql) {
    this.sql = sql;
  }

  clearSQL() {
    this.sql = '';
  }

  setActiveTab(tab) {
    this.activeTab = tab;
  }

  toggleSortColumn(columnName) {
    if (this.sortColumn === columnName) {
      if (this.sortDirection === 'asc') {
        this.sortDirection = 'desc';
      } else {
        this.sortColumn = '';
        this.sortDirection = 'asc';
      }
    } else {
      this.sortColumn = columnName;
      this.sortDirection = 'asc';
    }
  }

  setResultFilter(text) {
    this.resultFilter = text;
  }

  openJsonModal(data) {
    this.selectedJsonModalData = data;
  }

  closeJsonModal() {
    this.selectedJsonModalData = null;
  }

  formatSQL() {
    const input = this.sql.trim();
    if (!input) {
      return;
    }

    try {
      this.sql = formatSQLQuery(input, this.sqlDialect);
      this.errorMessage = '';
      this.showToast('SQL Formatted', 'success', 1500);
    } catch (error) {
      this.errorMessage = error instanceof Error ? `Format failed: ${error.message}` : 'Format failed';
      this.showToast(`Format failed: ${error.message}`, 'error', 3000);
    }
  }

  // --- SQL Snippets Library ---
  insertSnippet(type) {
    const rawTableName = this.selectedTable?.name || 'table_name';
    const schema = this.selectedTable?.schema || '';
    const table = this.selectedTable ? formatTableName(this.selectedTable) : '';

    const snippet = renderTemplateById(type, this.sqlDialect, {
      table,
      rawTableName,
      schema
    });

    this.sql = snippet;
    this.showToast('Template inserted into editor', 'info', 1500);
  }

  // --- Graphical Config Management ---
  async openConfigModal() {
    this.configModalOpen = true;
    await this.loadConfig();
  }

  closeConfigModal() {
    this.configModalOpen = false;
  }

  async loadConfig() {
    this.configLoading = true;
    try {
      const data = await this.api('/api/v1/config');
      this.fullConfig = data;
      this.configPath = data.config_path || this.configPath;
    } catch (error) {
      this.errorMessage = `Failed to load config: ${error.message}`;
      this.showToast(`Failed to load config: ${error.message}`, 'error');
    } finally {
      this.configLoading = false;
    }
  }

  async saveProfileConfig(name, profileData) {
    this.errorMessage = '';
    try {
      await this.api('/api/v1/config/profiles', {
        method: 'POST',
        body: JSON.stringify({ name, profile: profileData })
      });
      await this.loadConfig();
      const profileList = await this.api('/api/v1/profiles');
      this.profiles = profileList.profiles || [];
      this.showToast(`Profile "${name}" saved`, 'success');
    } catch (error) {
      this.errorMessage = `Failed to save profile: ${error.message}`;
      this.showToast(this.errorMessage, 'error');
      throw error;
    }
  }

  async deleteProfileConfig(name) {
    this.errorMessage = '';
    try {
      await this.api(`/api/v1/config/profiles/${encodeURIComponent(name)}`, {
        method: 'DELETE'
      });
      await this.loadConfig();
      const profileList = await this.api('/api/v1/profiles');
      this.profiles = profileList.profiles || [];
      if (this.selectedProfile === name) {
        this.selectedProfile = this.profiles[0]?.name || '';
        await this.loadTables();
      }
      this.showToast(`Profile "${name}" deleted`, 'info');
    } catch (error) {
      this.errorMessage = `Failed to delete profile: ${error.message}`;
      this.showToast(this.errorMessage, 'error');
      throw error;
    }
  }

  async saveSSHProxyConfig(name, proxyData) {
    this.errorMessage = '';
    try {
      await this.api('/api/v1/config/ssh-proxies', {
        method: 'POST',
        body: JSON.stringify({ name, ssh_proxy: proxyData })
      });
      await this.loadConfig();
      this.showToast(`SSH Proxy "${name}" saved`, 'success');
    } catch (error) {
      this.errorMessage = `Failed to save SSH proxy: ${error.message}`;
      this.showToast(this.errorMessage, 'error');
      throw error;
    }
  }

  async deleteSSHProxyConfig(name) {
    this.errorMessage = '';
    try {
      await this.api(`/api/v1/config/ssh-proxies/${encodeURIComponent(name)}`, {
        method: 'DELETE'
      });
      await this.loadConfig();
      this.showToast(`SSH Proxy "${name}" deleted`, 'info');
    } catch (error) {
      this.errorMessage = `Failed to delete SSH proxy: ${error.message}`;
      this.showToast(this.errorMessage, 'error');
      throw error;
    }
  }

  async saveAIConfig(aiData) {
    this.errorMessage = '';
    try {
      await this.api('/api/v1/config/ai', {
        method: 'POST',
        body: JSON.stringify({ ai: aiData })
      });
      await this.loadConfig();
      this.showToast('AI Settings saved successfully', 'success');
    } catch (error) {
      this.errorMessage = `Failed to save AI config: ${error.message}`;
      this.showToast(this.errorMessage, 'error');
      throw error;
    }
  }

  async testProfileConnection(name, profileData) {
    return this.api('/api/v1/config/test/profile', {
      method: 'POST',
      body: JSON.stringify({ name, profile: profileData })
    });
  }

  async testSSHProxyConnection(name, proxyData) {
    return this.api('/api/v1/config/test/ssh-proxy', {
      method: 'POST',
      body: JSON.stringify({ name, ssh_proxy: proxyData })
    });
  }

  async testAIConnection(aiData) {
    return this.api('/api/v1/config/test/ai', {
      method: 'POST',
      body: JSON.stringify({ ai: aiData })
    });
  }

  // --- Copy & Export ---
  async copyAsMarkdown() {
    if (!this.columns.length || !this.rows.length) {
      this.showToast('No data to copy', 'error');
      return;
    }
    const md = exportToMarkdownTable(this.columns, this.processedRows);
    await copyText(md);
    this.showToast(`Copied ${this.processedRows.length} rows as Markdown table!`, 'success');
  }

  async copyAsInsertSQL() {
    if (!this.columns.length || !this.rows.length) {
      this.showToast('No data to copy', 'error');
      return;
    }
    const sql = exportToInsertSQL(this.selectedTableName || 'query_result', this.columns, this.processedRows);
    await copyText(sql);
    this.showToast(`Copied ${this.processedRows.length} rows as SQL INSERT!`, 'success');
  }

  exportResults(type = 'csv') {
    if (!this.columns.length || !this.rows.length) {
      return;
    }

    let content = '';
    let mimeType = 'text/plain';
    let ext = 'txt';

    const rowsToExport = this.processedRows;

    if (type === 'csv') {
      mimeType = 'text/csv;charset=utf-8;';
      ext = 'csv';
      const escapeCSV = (val) => {
        if (val === null || val === undefined) return '""';
        const str = String(val).replace(/"/g, '""');
        return `"${str}"`;
      };
      const header = this.columns.map(escapeCSV).join(',');
      const body = rowsToExport.map((row) => this.columns.map((col) => escapeCSV(row[col])).join(',')).join('\n');
      content = `${header}\n${body}`;
    } else if (type === 'tsv') {
      mimeType = 'text/tab-separated-values;charset=utf-8;';
      ext = 'tsv';
      const escapeTSV = (val) => (val === null || val === undefined ? '' : String(val).replace(/\t/g, ' '));
      const header = this.columns.map(escapeTSV).join('\t');
      const body = rowsToExport.map((row) => this.columns.map((col) => escapeTSV(row[col])).join('\t')).join('\n');
      content = `${header}\n${body}`;
    } else if (type === 'json') {
      mimeType = 'application/json;charset=utf-8;';
      ext = 'json';
      content = JSON.stringify(rowsToExport, null, 2);
    }

    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `xsql_result_${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '_')}.${ext}`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    this.showToast(`Exported ${rowsToExport.length} rows to ${ext.toUpperCase()}`, 'success');
  }

  // Generate DDL for current table detail
  generateDDL(tableDetail) {
    if (!tableDetail || !tableDetail.columns || !tableDetail.columns.length) {
      return '-- No table columns available to generate DDL';
    }

    const isPG = this.sqlDialect === 'postgresql';
    const quoteIdent = (name) => (isPG ? `"${name}"` : `\`${name}\``);
    
    let tableName = quoteIdent(tableDetail.name);
    if (tableDetail.schema) {
      tableName = `${quoteIdent(tableDetail.schema)}.${quoteIdent(tableDetail.name)}`;
    }

    const comments = [];
    const cols = tableDetail.columns.map((col) => {
      const colIdent = quoteIdent(col.name);
      let def = `  ${colIdent} ${col.type}`;
      if (!col.nullable) def += ' NOT NULL';
      if (col.default != null && col.default !== '') def += ` DEFAULT ${col.default}`;
      if (!isPG && col.comment) {
        def += ` COMMENT '${col.comment.replaceAll("'", "''")}'`;
      }
      if (isPG && col.comment) {
        comments.push(`COMMENT ON COLUMN ${tableName}.${quoteIdent(col.name)} IS '${col.comment.replaceAll("'", "''")}';`);
      }
      return def;
    });

    const pkCols = tableDetail.columns.filter((c) => c.primary_key).map((c) => quoteIdent(c.name));
    if (pkCols.length > 0) {
      cols.push(`  PRIMARY KEY (${pkCols.join(', ')})`);
    }

    let ddl = `CREATE TABLE ${tableName} (\n${cols.join(',\n')}\n)`;
    if (!isPG) {
      if (tableDetail.comment) {
        ddl += ` COMMENT='${tableDetail.comment.replaceAll("'", "''")}'`;
      }
      ddl += ';';
    } else {
      ddl += ';';
      if (tableDetail.comment) {
        comments.unshift(`COMMENT ON TABLE ${tableName} IS '${tableDetail.comment.replaceAll("'", "''")}';`);
      }
      if (comments.length > 0) {
        ddl += `\n\n${comments.join('\n')}`;
      }
    }
    return ddl;
  }

  #tableDetailCacheKey(profileName, schemaName, tableName) {
    return `${profileName}:${schemaName}.${tableName}`;
  }

  #resetCompletionState(profileName) {
    this.completionCatalog = createEmptyCompletionCatalog(profileName);
    this.#tableDetailCache.clear();
    this.#tableDetailRequestCache.clear();
  }

  #setCompletionCatalog(tables, activeSchema = '') {
    this.completionCatalog = buildCompletionCatalog(this.selectedProfile, tables, activeSchema);
  }

  #setCompletionActiveSchema(schemaName = '') {
    const nextActiveSchema = schemaName || this.completionCatalog.defaultSchema;
    if (this.completionCatalog.activeSchema === nextActiveSchema) {
      return;
    }
    this.completionCatalog = {
      ...this.completionCatalog,
      activeSchema: nextActiveSchema
    };
  }

  #cacheTableDetail(profileName, table, detail) {
    const key = this.#tableDetailCacheKey(profileName, table.schema, table.name);
    this.#tableDetailCache.set(key, detail);
    return detail;
  }

  getCompletionTableDetail(schemaName, tableName) {
    if (!this.selectedProfile) {
      return null;
    }
    const key = this.#tableDetailCacheKey(this.selectedProfile, schemaName, tableName);
    return this.#tableDetailCache.get(key) || null;
  }

  async #fetchTableDetail(profileName, table) {
    const cacheKey = this.#tableDetailCacheKey(profileName, table.schema, table.name);
    const cachedDetail = this.#tableDetailCache.get(cacheKey);
    if (cachedDetail) {
      return cachedDetail;
    }

    const pendingRequest = this.#tableDetailRequestCache.get(cacheKey);
    if (pendingRequest) {
      return pendingRequest;
    }

    const request = this.api(
      `/api/v1/schema/tables/${encodeURIComponent(table.schema)}/${encodeURIComponent(table.name)}?profile=${encodeURIComponent(profileName)}`
    )
      .then((detail) => this.#cacheTableDetail(profileName, table, detail))
      .finally(() => {
        this.#tableDetailRequestCache.delete(cacheKey);
      });

    this.#tableDetailRequestCache.set(cacheKey, request);
    return request;
  }

  async ensureCompletionTableDetail(schemaName, tableName) {
    if (!this.selectedProfile || !schemaName || !tableName) {
      return null;
    }

    try {
      return await this.#fetchTableDetail(this.selectedProfile, { schema: schemaName, name: tableName });
    } catch {
      return null;
    }
  }

  async initialize() {
    this.pageLoading = true;
    this.errorMessage = '';
    try {
      const [health, profileData] = await Promise.all([
        this.api('/api/v1/health'),
        this.api('/api/v1/profiles')
      ]);
      this.authRequired = Boolean(health.auth_required);
      if (!this.selectedProfile && typeof health.initial_profile === 'string') {
        this.selectedProfile = health.initial_profile;
      }

      this.profiles = profileData.profiles || [];
      this.configPath = profileData.config_path || '';
      if (!this.selectedProfile && this.profiles.length > 0) {
        this.selectedProfile = this.profiles[0].name;
      }
    } catch (error) {
      this.errorMessage = error.message;
      this.showToast(error.message, 'error', 5000);
    } finally {
      this.pageLoading = false;
    }

    await this.loadTables();
  }

  async loadTables() {
    const requestSeq = ++this.#schemaRequestSeq;
    if (!this.selectedProfile) {
      this.schemaTables = [];
      this.selectedTable = null;
      this.selectedTableDetail = null;
      this.#resetCompletionState('');
      return;
    }

    this.schemaLoading = true;
    this.errorMessage = '';
    this.#resetCompletionState(this.selectedProfile);
    try {
      const data = await this.api(`/api/v1/schema/tables?profile=${encodeURIComponent(this.selectedProfile)}`);
      if (requestSeq !== this.#schemaRequestSeq) {
        return;
      }
      this.schemaTables = data.tables || [];
      this.selectedTable = this.schemaTables[0] || null;
      this.selectedTableDetail = null;
      this.activeTab = 'results';
      this.#setCompletionCatalog(this.schemaTables, this.selectedTable?.schema || '');

      if (this.selectedTable) {
        await this.loadTableDetail(this.selectedTable);
      }
    } catch (error) {
      if (requestSeq !== this.#schemaRequestSeq) {
        return;
      }
      this.errorMessage = error.message;
      this.schemaTables = [];
      this.selectedTable = null;
      this.selectedTableDetail = null;
      this.#resetCompletionState(this.selectedProfile);
    } finally {
      this.schemaLoading = false;
    }
  }

  async loadTableDetail(table) {
    const requestSeq = ++this.#structureRequestSeq;
    if (!this.selectedProfile || !table) {
      this.selectedTableDetail = null;
      return;
    }

    this.structureLoading = true;
    this.errorMessage = '';
    const profileName = this.selectedProfile;
    try {
      const data = await this.#fetchTableDetail(profileName, table);
      if (requestSeq !== this.#structureRequestSeq || profileName !== this.selectedProfile) {
        return;
      }
      this.selectedTableDetail = data;
    } catch (error) {
      if (requestSeq !== this.#structureRequestSeq || profileName !== this.selectedProfile) {
        return;
      }
      this.selectedTableDetail = null;
      this.errorMessage = error.message;
    } finally {
      this.structureLoading = false;
    }
  }

  async runQuery() {
    if (!this.selectedProfile || !this.sql.trim()) {
      return;
    }

    this.queryLoading = true;
    this.errorMessage = '';
    this.resultFilter = '';
    this.sortColumn = '';
    this.sortDirection = 'asc';
    const startTime = performance.now();

    let queryError = null;
    let returnedRows = [];
    let returnedColumns = [];

    try {
      const data = await this.api('/api/v1/query', {
        method: 'POST',
        body: JSON.stringify({ profile: this.selectedProfile, sql: this.sql })
      });
      returnedColumns = data.columns || [];
      returnedRows = data.rows || [];
      this.columns = returnedColumns;
      this.rows = returnedRows;
      this.activeTab = 'results';
    } catch (error) {
      queryError = error.message;
      this.columns = [];
      this.rows = [];
      this.errorMessage = error.message;
      this.showToast(`Query error: ${error.message}`, 'error', 4000);
    } finally {
      const durationMs = Math.round(performance.now() - startTime);
      this.lastExecutionMs = durationMs;
      this.queryLoading = false;

      // Add to Query History
      const historyItem = {
        id: String(Date.now()),
        sql: this.sql,
        profile: this.selectedProfile,
        durationMs,
        rowCount: returnedRows.length,
        error: queryError,
        timestamp: new Date().toLocaleTimeString()
      };
      this.queryHistory = [historyItem, ...this.queryHistory.slice(0, 49)];
      saveHistoryStorage(this.queryHistory);
    }
  }

  clearQueryHistory(filterProfile) {
    if (!filterProfile || filterProfile === '__all__') {
      this.queryHistory = [];
      saveHistoryStorage([]);
      this.showToast('All query history cleared', 'info');
    } else {
      this.queryHistory = this.queryHistory.filter((item) => item.profile !== filterProfile);
      saveHistoryStorage(this.queryHistory);
      this.showToast(`History cleared for profile ${filterProfile}`, 'info');
    }
  }

  async selectHistoryItem(item) {
    if (!item) return;
    if (item.profile && item.profile !== this.selectedProfile) {
      try {
        await this.selectProfile(item.profile);
      } catch (e) {
        console.error('Failed to switch profile:', e);
      }
    }
    if (item.sql) {
      this.setSQL(item.sql);
    }
    this.queryHistoryOpen = false;
  }

  async selectProfile(profileName) {
    this.selectedProfile = profileName;
    this.columns = [];
    this.rows = [];
    this.selectedTable = null;
    this.selectedTableDetail = null;
    this.activeTab = 'results';
    this.#resetCompletionState(profileName);
    await this.loadTables();
  }

  async previewTable(table) {
    this.selectedTable = table;
    this.activeTab = 'results';
    this.#setCompletionActiveSchema(table.schema);
    void this.loadTableDetail(table);
    this.sql = buildPreviewSQL(table);
    await this.runQuery();
  }
}
