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
