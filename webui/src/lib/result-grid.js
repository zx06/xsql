const PREVIEW_LENGTH = 96;
const TOOLTIP_LENGTH = 1200;

export function tryParseJson(value) {
  if (value === null || value === undefined) return null;
  if (typeof value === 'object') return value;
  if (typeof value !== 'string') return null;
  const trimmed = value.trim();
  if (
    (!trimmed.startsWith('{') || !trimmed.endsWith('}')) &&
    (!trimmed.startsWith('[') || !trimmed.endsWith(']'))
  ) {
    return null;
  }
  try {
    let parsed = JSON.parse(trimmed);
    // Handle double-escaped JSON string
    if (typeof parsed === 'string' && (
      (parsed.startsWith('{') && parsed.endsWith('}')) ||
      (parsed.startsWith('[') && parsed.endsWith(']'))
    )) {
      try {
        parsed = JSON.parse(parsed);
      } catch {
        // keep single parsed
      }
    }
    return parsed;
  } catch {
    return null;
  }
}

function safeJSONStringify(value, indent = 0) {
  try {
    const parsed = tryParseJson(value);
    if (parsed !== null) {
      return JSON.stringify(parsed, null, indent);
    }
    return typeof value === 'object' ? JSON.stringify(value, null, indent) : String(value);
  } catch {
    return String(value);
  }
}

function collapseWhitespace(value) {
  return String(value).replace(/\s+/g, ' ').trim();
}

function buildPreviewText(value, kind) {
  if (kind === 'null') {
    return 'NULL';
  }
  if (kind === 'string') {
    return value === '' ? "''" : collapseWhitespace(value);
  }
  if (kind === 'json') {
    const parsed = tryParseJson(value);
    if (parsed !== null) {
      return collapseWhitespace(JSON.stringify(parsed));
    }
    return collapseWhitespace(safeJSONStringify(value));
  }
  return String(value);
}

function buildFullText(value, kind) {
  if (kind === 'null') {
    return 'NULL';
  }
  if (kind === 'json') {
    const parsed = tryParseJson(value);
    if (parsed !== null) {
      return JSON.stringify(parsed, null, 2);
    }
    return safeJSONStringify(value, 2);
  }
  return String(value);
}

export function detectValueKind(value) {
  if (value === null || value === undefined) {
    return 'null';
  }
  if (Array.isArray(value)) {
    return 'json';
  }
  if (typeof value === 'boolean') {
    return 'boolean';
  }
  if (typeof value === 'number') {
    return 'number';
  }
  if (typeof value === 'object') {
    return 'json';
  }
  if (typeof value === 'string') {
    // Check if valid JSON string
    const parsed = tryParseJson(value);
    if (parsed !== null && typeof parsed === 'object') {
      return 'json';
    }
    // Check if ISO Date/Time
    if (/^\d{4}-\d{2}-\d{2}(T|\s)\d{2}:\d{2}/.test(value)) {
      return 'datetime';
    }
    if (/^\d{4}-\d{2}-\d{2}$/.test(value)) {
      return 'date';
    }
    return 'string';
  }
  return 'other';
}

export function highlightJson(jsonStr) {
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

/**
 * Infer the predominant data type of a column based on its values.
 */
export function inferColumnType(columnName, rows = []) {
  const name = columnName.toLowerCase();
  if (name === 'id' || name.endsWith('_id') || name.endsWith('id')) return 'id';
  if (name.includes('created') || name.includes('updated') || name.includes('time') || name.includes('date')) return 'datetime';
  if (name.includes('is_') || name.includes('has_') || name.includes('enabled') || name.includes('active')) return 'boolean';

  if (!rows || rows.length === 0) return 'string';

  let numCount = 0;
  let boolCount = 0;
  let jsonCount = 0;
  let dateCount = 0;
  let nonNullCount = 0;

  for (const row of rows.slice(0, 30)) {
    const val = row[columnName];
    if (val === null || val === undefined) continue;
    nonNullCount++;
    const kind = detectValueKind(val);
    if (kind === 'number') numCount++;
    else if (kind === 'boolean') boolCount++;
    else if (kind === 'json') jsonCount++;
    else if (kind === 'datetime' || kind === 'date') dateCount++;
  }

  if (nonNullCount === 0) return 'string';
  if (numCount / nonNullCount > 0.8) return 'number';
  if (boolCount / nonNullCount > 0.8) return 'boolean';
  if (jsonCount / nonNullCount > 0.4) return 'json';
  if (dateCount / nonNullCount > 0.8) return 'datetime';

  return 'string';
}

export function formatResultCellValue(value) {
  const kind = detectValueKind(value);
  const previewText = buildPreviewText(value, kind);
  const fullText = buildFullText(value, kind);
  const highlightedHtml = kind === 'json' ? highlightJson(fullText) : '';

  return {
    kind,
    raw: value,
    previewText,
    previewDisplay: previewText.length > PREVIEW_LENGTH ? `${previewText.slice(0, PREVIEW_LENGTH - 1)}…` : previewText,
    fullText,
    highlightedHtml,
    tooltipText: fullText.length > TOOLTIP_LENGTH ? `${fullText.slice(0, TOOLTIP_LENGTH - 1)}…` : fullText,
    isEmptyString: kind === 'string' && value === '',
    isLong: previewText.length > PREVIEW_LENGTH || /\n/.test(fullText)
  };
}

export function buildSelectedResultCell({ rowIndex, columnName, value }) {
  const formatted = formatResultCellValue(value);
  return {
    rowIndex,
    columnName,
    ...formatted
  };
}

export async function copyText(value) {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }

  if (typeof document === 'undefined') {
    throw new Error('Clipboard is unavailable');
  }

  const element = document.createElement('textarea');
  element.value = value;
  element.setAttribute('readonly', 'true');
  element.style.position = 'absolute';
  element.style.left = '-9999px';
  document.body.appendChild(element);
  element.select();

  const copied = document.execCommand('copy');
  document.body.removeChild(element);
  if (!copied) {
    throw new Error('Clipboard copy failed');
  }
}

export function isCellTruncated(element) {
  if (!element) {
    return false;
  }
  return element.scrollWidth > element.clientWidth || element.scrollHeight > element.clientHeight;
}

/**
 * Format rows as Markdown Table string
 */
export function exportToMarkdownTable(columns, rows) {
  if (!columns.length || !rows.length) return '';
  const header = `| ${columns.join(' | ')} |`;
  const separator = `| ${columns.map(() => '---').join(' | ')} |`;
  const body = rows
    .map((row) => `| ${columns.map((col) => {
      const val = row[col];
      if (val === null || val === undefined) return 'NULL';
      return String(val)
        .replace(/\\/g, '\\\\')
        .replace(/\|/g, '\\|')
        .replace(/\r?\n/g, ' ');
    }).join(' | ')} |`)
    .join('\n');

  return `${header}\n${separator}\n${body}`;
}

/**
 * Format rows as SQL INSERT statements
 */
export function exportToInsertSQL(tableName, columns, rows) {
  if (!columns.length || !rows.length) return '';
  const table = tableName || 'table_name';
  const formatVal = (v) => {
    if (v === null || v === undefined) return 'NULL';
    if (typeof v === 'number') return String(v);
    if (typeof v === 'boolean') return v ? 'TRUE' : 'FALSE';
    if (typeof v === 'object') {
      const jsonStr = JSON.stringify(v).replace(/\\/g, '\\\\').replace(/'/g, "''");
      return `'${jsonStr}'`;
    }
    const escaped = String(v).replace(/\\/g, '\\\\').replace(/'/g, "''");
    return `'${escaped}'`;
  };

  const valuesLines = rows.map((row) => `  (${columns.map((c) => formatVal(row[c])).join(', ')})`).join(',\n');
  return `INSERT INTO ${table} (${columns.join(', ')})\nVALUES\n${valuesLines};`;
}
