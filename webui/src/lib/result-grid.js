const PREVIEW_LENGTH = 96;
const TOOLTIP_LENGTH = 1200;

function safeJSONStringify(value, indent = 0) {
  try {
    return JSON.stringify(value, null, indent);
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
    return collapseWhitespace(safeJSONStringify(value));
  }
  return String(value);
}

function buildFullText(value, kind) {
  if (kind === 'null') {
    return 'NULL';
  }
  if (kind === 'json') {
    return safeJSONStringify(value, 2);
  }
  return String(value);
}

function detectValueKind(value) {
  if (value === null || value === undefined) {
    return 'null';
  }
  if (Array.isArray(value)) {
    return 'json';
  }
  if (typeof value === 'string') {
    // Try parsing string as JSON
    const trimmed = value.trim();
    if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
      try {
        JSON.parse(trimmed);
        return 'json';
      } catch {
        // regular string
      }
    }
    return 'string';
  }
  if (typeof value === 'number') {
    return 'number';
  }
  if (typeof value === 'boolean') {
    return 'boolean';
  }
  if (typeof value === 'object') {
    return 'json';
  }
  return 'other';
}

function escapeHTML(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

export function highlightJSON(input) {
  if (input === null || input === undefined) {
    return '<span class="text-[var(--muted)] italic">null</span>';
  }

  let formatted = '';
  if (typeof input !== 'string') {
    formatted = JSON.stringify(input, null, 2);
  } else {
    try {
      formatted = JSON.stringify(JSON.parse(input), null, 2);
    } catch {
      return escapeHTML(input);
    }
  }

  const regex = /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g;

  return formatted.replace(regex, (match) => {
    if (/^"/.test(match)) {
      if (/:$/.test(match)) {
        const key = match.slice(0, -1);
        return `<span class="text-[var(--accent)] font-semibold">${escapeHTML(key)}</span>:`;
      }
      return `<span class="text-emerald-600 dark:text-emerald-400">${escapeHTML(match)}</span>`;
    }
    if (/true|false/.test(match)) {
      return `<span class="text-pink-600 dark:text-pink-400 font-semibold">${escapeHTML(match)}</span>`;
    }
    if (/null/.test(match)) {
      return `<span class="text-[var(--muted)] italic font-semibold">${escapeHTML(match)}</span>`;
    }
    return `<span class="text-amber-600 dark:text-amber-400">${escapeHTML(match)}</span>`;
  });
}

export function isCellTruncated(element) {
  if (!element) {
    return false;
  }
  return element.scrollWidth > element.clientWidth;
}

export function formatResultCellValue(value) {
  const kind = detectValueKind(value);
  const previewText = buildPreviewText(value, kind);
  const fullText = buildFullText(value, kind);

  return {
    kind,
    raw: value,
    previewText,
    previewDisplay: previewText.length > PREVIEW_LENGTH ? `${previewText.slice(0, PREVIEW_LENGTH - 1)}…` : previewText,
    fullText,
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
  document.execCommand('copy');
  document.body.removeChild(element);
}
