import templatesData from './templates.json';

/**
 * Returns available templates filtered and adapted for the given SQL dialect.
 * @param {string} dialect - 'mysql' | 'postgresql' | 'sql'
 * @returns {Array<{id: string, title: string, description: string, category: string}>}
 */
export function getAvailableTemplates(dialect = 'sql') {
  return templatesData.map((tpl) => ({
    id: tpl.id,
    title: tpl.title,
    description: tpl.description || '',
    category: tpl.category || 'General'
  }));
}

/**
 * Renders a template into executable SQL for the given dialect and table context.
 * @param {string} templateId 
 * @param {string} dialect 
 * @param {{ table?: string, rawTableName?: string, schema?: string }} context 
 * @returns {string}
 */
export function renderTemplateById(templateId, dialect = 'sql', context = {}) {
  const tpl = templatesData.find((t) => t.id === templateId);
  if (!tpl) {
    return context.table ? `SELECT * FROM ${context.table} LIMIT 20;` : 'SELECT 1;';
  }

  const isPG = dialect === 'postgresql';
  const rawTable = context.rawTableName || 'table_name';
  const formattedTable = context.table || (isPG ? '"public"."table_name"' : '`table_name`');
  const schema = context.schema || (isPG ? 'public' : '');

  // Select dialect-specific template with fallbacks
  let sqlPattern = tpl.dialects?.[dialect] || tpl.dialects?.['default'] || '';
  if (!sqlPattern) {
    // Fallback to first available dialect
    const keys = Object.keys(tpl.dialects || {});
    sqlPattern = keys.length > 0 ? tpl.dialects[keys[0]] : '';
  }

  // Replace placeholders
  return sqlPattern
    .replace(/\{table\}/g, formattedTable)
    .replace(/\{rawTableName\}/g, rawTable)
    .replace(/\{schema\}/g, schema);
}
