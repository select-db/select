import * as monaco from 'monaco-editor';
import { format as formatSQL } from 'sql-formatter';

type Language =
	| 'postgresql'
	| 'bigquery'
	| 'db2'
	| 'db2i'
	| 'hive'
	| 'mariadb'
	| 'mysql'
	| 'n1ql'
	| 'plsql'
	| 'redshift'
	| 'singlestoredb'
	| 'snowflake'
	| 'spark'
	| 'sql'
	| 'sqlite'
	| 'tidb'
	| 'transactsql'
	| 'trino'
	| 'tsql';

const KEYWORDS_AFTER_TRAILING_COMMA = [
	'returning',
	'intersect',
	'except',
	'recursive',
	'materialized',
	'natural',
	'from',
	'where',
	'group',
	'having',
	'order',
	'limit',
	'offset',
	'on',
	'as',
	'into',
	'values',
	'set',
	'union',
	'join',
	'left',
	'right',
	'inner',
	'outer',
	'cross',
	'using',
	'with',
	'lateral',
	'and',
	'or',
	'by'
];

const RE_TRAILING_COMMA_BEFORE_KEYWORD = new RegExp(
	`,\\s*\\b(${KEYWORDS_AFTER_TRAILING_COMMA.join('|')})\\b`,
	'gi'
);

/**
 * Removes commas that are immediately followed by optional whitespace and then
 * an SQL keyword (e.g. "SELECT a, b, FROM t" → "SELECT a, b FROM t").
 */
function removeTrailingCommasBeforeKeywords(sql: string): string {
	return sql.replace(RE_TRAILING_COMMA_BEFORE_KEYWORD, ' $1');
}

/**
 * Pre-processes SQL by replacing $VARIABLE with positional parameters ($1, $2, etc.)
 * Returns the processed SQL and a mapping to restore variable names
 */
function preprocessVariables(sql: string): { processedSQL: string; variables: string[] } {
	const variables: string[] = [];
	const varPattern = /\$([A-Za-z_][A-Za-z0-9_]*)/g;

	const processedSQL = sql.replace(varPattern, (_match, varName) => {
		variables.push(varName);
		return `$${variables.length}`;
	});

	return { processedSQL, variables };
}

/**
 * Post-processes SQL by replacing positional parameters ($1, $2, etc.) back to $VARIABLE
 */
function restoreVariables(sql: string, variables: string[]): string {
	let result = sql;

	// Replace in reverse order to avoid issues with $1 matching in $10, $11, etc.
	for (let i = variables.length; i >= 1; i--) {
		const varName = variables[i - 1];
		// Use word boundary to ensure we don't replace $1 in $10
		const pattern = new RegExp(`\\$${i}\\b`, 'g');
		result = result.replace(pattern, `$${varName}`);
	}

	return result;
}

export const format = (
	editor: monaco.editor.IStandaloneCodeEditor | null,
	language: Language,
	onchange?: (content: string) => void
) => {
	if (!editor) return;

	const model = editor.getModel();
	if (!model) return;

	const originalSQL = model.getValue();

	// Pre-process: Replace $VARIABLE with $1, $2, etc.
	const { processedSQL, variables } = preprocessVariables(originalSQL);

	// Remove trailing commas before keywords (e.g. "a, b, FROM" → "a, b FROM")
	const sqlForFormat = removeTrailingCommasBeforeKeywords(processedSQL);

	// Format the SQL with positional parameters
	const formattedSQL = formatSQL(sqlForFormat, {
		language,
		keywordCase: 'upper',
		tabWidth: 2
	});

	// Post-process: Restore variable names
	const restoredSQL = restoreVariables(formattedSQL, variables);

	// Skip if nothing changed
	if (restoredSQL === originalSQL) return;

	// Use pushEditOperations for proper undo support and reliable updates
	const fullRange = model.getFullModelRange();
	model.pushEditOperations(
		editor.getSelections(),
		[{ range: fullRange, text: restoredSQL }],
		() => null
	);

	// Immediately notify parent to update content prop
	onchange?.(restoredSQL);
};
