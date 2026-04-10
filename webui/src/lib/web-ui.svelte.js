import { formatSQLQuery, resolveSQLDialectName } from './sql-editor.js';

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

function formatTableName(table) {
  return `${table.schema}.${table.name}`;
}

function buildPreviewSQL(table) {
  return `SELECT * FROM ${formatTableName(table)} LIMIT 10`;
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
  sql = $state('SELECT 1');
  columns = $state.raw([]);
  rows = $state.raw([]);
  configPath = $state('');
  activeTab = $state('structure');
  themeMode = $state(readThemeMode());
  systemPrefersDark = $state(false);
  completionCatalog = $state.raw(createEmptyCompletionCatalog());

  rowCount = $derived(this.rows.length);
  tableCount = $derived(this.schemaTables.length);
  selectedProfileMeta = $derived(this.profiles.find((profile) => profile.name === this.selectedProfile) ?? null);
  selectedTableName = $derived(this.selectedTable ? formatTableName(this.selectedTable) : '');
  resolvedTheme = $derived(this.themeMode === 'auto' ? (this.systemPrefersDark ? 'black' : 'white') : this.themeMode);
  sqlDialect = $derived(resolveSQLDialectName(this.selectedProfileMeta?.db));

  #schemaRequestSeq = 0;
  #structureRequestSeq = 0;
  #tableDetailCache = new Map();
  #tableDetailRequestCache = new Map();

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
    return payload.data;
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

  setSQL(sql) {
    this.sql = sql;
  }

  setActiveTab(tab) {
    this.activeTab = tab;
  }

  formatSQL() {
    const input = this.sql.trim();
    if (!input) {
      return;
    }

    try {
      this.sql = formatSQLQuery(input, this.sqlDialect);
      this.errorMessage = '';
    } catch (error) {
      this.errorMessage = error instanceof Error ? `Format failed: ${error.message}` : 'Format failed';
    }
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
      this.activeTab = 'structure';
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
    try {
      const data = await this.api('/api/v1/query', {
        method: 'POST',
        body: JSON.stringify({ profile: this.selectedProfile, sql: this.sql })
      });
      this.columns = data.columns || [];
      this.rows = data.rows || [];
      this.activeTab = 'results';
    } catch (error) {
      this.columns = [];
      this.rows = [];
      this.errorMessage = error.message;
    } finally {
      this.queryLoading = false;
    }
  }

  async selectProfile(profileName) {
    this.selectedProfile = profileName;
    this.columns = [];
    this.rows = [];
    this.selectedTable = null;
    this.selectedTableDetail = null;
    this.activeTab = 'structure';
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
