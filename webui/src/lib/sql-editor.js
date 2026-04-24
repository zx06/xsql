import { autocompletion } from '@codemirror/autocomplete';
import { EditorState } from '@codemirror/state';
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { EditorView } from '@codemirror/view';
import { MySQL, PostgreSQL, StandardSQL, keywordCompletionSource, sql } from '@codemirror/lang-sql';
import { minimalSetup } from 'codemirror';
import { tags } from '@lezer/highlight';
import { format as formatSQL } from 'sql-formatter';

const identifierPattern = /^[A-Za-z_][A-Za-z0-9_$]*$/;
const validIdentifierChars = /[A-Za-z0-9_$.]/;
const completionFilterPattern = /^[A-Za-z0-9_$]*$/;

const sqlHighlightStyle = HighlightStyle.define([
  { tag: [tags.keyword, tags.operatorKeyword], color: 'var(--accent)', fontWeight: '600' },
  { tag: tags.string, color: 'var(--accent-border)' },
  { tag: [tags.number, tags.bool, tags.null], color: 'var(--tag-text)' },
  { tag: [tags.comment, tags.lineComment, tags.blockComment], color: 'var(--muted)', fontStyle: 'italic' },
  { tag: tags.typeName, color: 'var(--pill-text)' },
  { tag: [tags.name, tags.propertyName], color: 'var(--text)' }
]);

export const sqlEditorBaseExtensions = [
  minimalSetup,
  EditorState.tabSize.of(2),
  EditorView.lineWrapping,
  EditorView.theme({
    '&': {
      height: '100%',
      borderRadius: '0.75rem',
      border: '1px solid var(--input-border)',
      backgroundColor: 'var(--input-bg)',
      color: 'var(--text)'
    },
    '&.cm-focused': {
      outline: 'none',
      borderColor: 'var(--accent)',
      boxShadow: '0 0 0 2px var(--accent-soft)'
    },
    '.cm-scroller': {
      overflow: 'auto',
      fontFamily: 'ui-monospace, SFMono-Regular, "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", monospace'
    },
    '.cm-content': {
      minHeight: '7rem',
      padding: '0.75rem 0.9rem',
      caretColor: 'var(--text)',
      fontSize: '13px',
      lineHeight: '1.65'
    },
    '.cm-cursor, .cm-dropCursor': {
      borderLeftColor: 'var(--accent)'
    },
    '.cm-selectionBackground, &.cm-focused .cm-selectionBackground, ::selection': {
      backgroundColor: 'var(--accent-soft)'
    },
    '.cm-activeLine': {
      backgroundColor: 'transparent'
    },
    '.cm-placeholder': {
      color: 'var(--muted)'
    },
    '.cm-tooltip': {
      border: '1px solid var(--panel-border)',
      backgroundColor: 'var(--panel-bg)',
      color: 'var(--text)',
      boxShadow: '0 10px 24px var(--panel-shadow)',
      backdropFilter: 'blur(10px)'
    },
    '.cm-tooltip-autocomplete > ul': {
      maxHeight: '15rem',
      overflow: 'auto'
    },
    '.cm-tooltip-autocomplete ul li': {
      borderRadius: '0.5rem',
      margin: '0.1rem 0.2rem',
      padding: '0.3rem 0.45rem'
    },
    '.cm-tooltip-autocomplete ul li[aria-selected]': {
      backgroundColor: 'var(--accent-soft)',
      color: 'var(--text)'
    }
  }),
  syntaxHighlighting(sqlHighlightStyle)
];

function dialectForName(dialectName) {
  if (dialectName === 'mysql') {
    return MySQL;
  }
  if (dialectName === 'postgresql') {
    return PostgreSQL;
  }
  return StandardSQL;
}

function buildTableInfo(table) {
  return [table.schema, table.comment].filter(Boolean).join(' · ');
}

function buildColumnInfo(column) {
  const flags = [];
  if (column.primary_key) {
    flags.push('PK');
  }
  if (!column.nullable) {
    flags.push('NOT NULL');
  }
  if (column.default != null && column.default !== '') {
    flags.push(`DEFAULT ${column.default}`);
  }
  if (column.comment) {
    flags.push(column.comment);
  }
  return flags.join(' · ');
}

function buildTopLevelTableOptions(catalog) {
  return (catalog.tables || []).map((table) => ({
    label: table.name,
    type: 'class',
    detail: table.schema,
    info: buildTableInfo(table),
    apply: `${table.schema}.${table.name}`
  }));
}

function buildSchemaTableOptions(catalog, schemaName) {
  return (catalog.tablesBySchema?.[schemaName] || []).map((table) => ({
    label: table.name,
    type: 'class',
    detail: 'table',
    info: buildTableInfo(table)
  }));
}

function buildColumnOptions(detail) {
  return (detail?.columns || []).map((column) => ({
    label: column.name,
    type: 'property',
    detail: column.type || '',
    info: buildColumnInfo(column)
  }));
}

function readCompletionChain(doc, pos) {
  let from = pos;
  while (from > 0) {
    const char = doc.sliceString(from - 1, from);
    if (!validIdentifierChars.test(char)) {
      break;
    }
    from -= 1;
  }
  return {
    from,
    text: doc.sliceString(from, pos)
  };
}

function parseCompletionTarget(context) {
  const { from, text } = readCompletionChain(context.state.doc, context.pos);
  if (!text) {
    return context.explicit
      ? {
          kind: 'top-level',
          from: context.pos,
          to: context.pos,
          prefix: ''
        }
      : null;
  }

  const trailingDot = text.endsWith('.');
  const raw = trailingDot ? text.slice(0, -1) : text;
  const parts = raw ? raw.split('.') : [];
  if (parts.some((part) => !identifierPattern.test(part))) {
    return null;
  }

  if (parts.length === 1) {
    if (trailingDot) {
      return {
        kind: 'schema-or-table',
        from: context.pos,
        to: context.pos,
        identifier: parts[0]
      };
    }
    return {
      kind: 'top-level',
      from,
      to: context.pos,
      prefix: parts[0]
    };
  }

  if (parts.length === 2) {
    if (trailingDot) {
      return {
        kind: 'qualified-table',
        from: context.pos,
        to: context.pos,
        first: parts[0],
        second: parts[1]
      };
    }
    return {
      kind: 'member',
      from: context.pos - parts[1].length,
      to: context.pos,
      first: parts[0],
      prefix: parts[1]
    };
  }

  if (parts.length === 3 && !trailingDot) {
    return {
      kind: 'qualified-member',
      from: context.pos - parts[2].length,
      to: context.pos,
      schema: parts[0],
      table: parts[1],
      prefix: parts[2]
    };
  }

  return null;
}

function resolveUnqualifiedTable(catalog, tableName) {
  const candidateSchemas = catalog.tableSchemas?.[tableName] || [];
  if (candidateSchemas.length === 0) {
    return null;
  }
  if (catalog.activeSchema && candidateSchemas.includes(catalog.activeSchema)) {
    return { schema: catalog.activeSchema, name: tableName };
  }
  if (catalog.defaultSchema && candidateSchemas.includes(catalog.defaultSchema)) {
    return { schema: catalog.defaultSchema, name: tableName };
  }
  if (candidateSchemas.length === 1) {
    return { schema: candidateSchemas[0], name: tableName };
  }
  return null;
}

function buildCompletionResult(from, to, options) {
  if (!options || options.length === 0) {
    return null;
  }
  return {
    from,
    to,
    options,
    validFor: completionFilterPattern
  };
}

function createSchemaCompletionSource({ getCatalog, getTableDetail, ensureTableDetail }) {
  return async (context) => {
    const catalog = getCatalog();
    if (!catalog || catalog.profile === '') {
      return null;
    }

    const target = parseCompletionTarget(context);
    if (!target) {
      return null;
    }

    if (target.kind === 'top-level') {
      return buildCompletionResult(target.from, target.to, buildTopLevelTableOptions(catalog));
    }

    if (target.kind === 'schema-or-table') {
      if (catalog.tablesBySchema?.[target.identifier]) {
        return buildCompletionResult(target.from, target.to, buildSchemaTableOptions(catalog, target.identifier));
      }

      const tableRef = resolveUnqualifiedTable(catalog, target.identifier);
      if (!tableRef) {
        return null;
      }

      const detail =
        getTableDetail(tableRef.schema, tableRef.name) ||
        (await ensureTableDetail(tableRef.schema, tableRef.name));
      return buildCompletionResult(target.from, target.to, buildColumnOptions(detail));
    }

    if (target.kind === 'member') {
      if (catalog.tablesBySchema?.[target.first]) {
        return buildCompletionResult(target.from, target.to, buildSchemaTableOptions(catalog, target.first));
      }

      const tableRef = resolveUnqualifiedTable(catalog, target.first);
      if (!tableRef) {
        return null;
      }

      const detail =
        getTableDetail(tableRef.schema, tableRef.name) ||
        (await ensureTableDetail(tableRef.schema, tableRef.name));
      return buildCompletionResult(target.from, target.to, buildColumnOptions(detail));
    }

    if (target.kind === 'qualified-table') {
      const detail =
        getTableDetail(target.first, target.second) ||
        (await ensureTableDetail(target.first, target.second));
      return buildCompletionResult(target.from, target.to, buildColumnOptions(detail));
    }

    if (target.kind === 'qualified-member') {
      const detail =
        getTableDetail(target.schema, target.table) ||
        (await ensureTableDetail(target.schema, target.table));
      return buildCompletionResult(target.from, target.to, buildColumnOptions(detail));
    }

    return null;
  };
}

export function resolveSQLDialectName(dbName) {
  const normalized = String(dbName || '').trim().toLowerCase();
  if (normalized === 'mysql') {
    return 'mysql';
  }
  if (normalized === 'postgres' || normalized === 'postgresql') {
    return 'postgresql';
  }
  return 'sql';
}

export function createSQLLanguageSupport(dialectName) {
  return sql({ dialect: dialectForName(dialectName) });
}

export function createSQLAutocompletion({ dialectName, getCatalog, getTableDetail, ensureTableDetail }) {
  const dialect = dialectForName(dialectName);
  return autocompletion({
    activateOnTyping: true,
    closeOnBlur: true,
    override: [
      createSchemaCompletionSource({ getCatalog, getTableDetail, ensureTableDetail }),
      keywordCompletionSource(dialect, true)
    ]
  });
}

export function formatSQLQuery(sqlText, dialectName) {
  return formatSQL(sqlText, {
    language: dialectName === 'postgresql' ? 'postgresql' : dialectName === 'mysql' ? 'mysql' : 'sql',
    tabWidth: 2,
    linesBetweenQueries: 1
  });
}
