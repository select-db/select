import type * as Monaco from 'monaco-editor';

import { sqlLanguage } from '$lib/components/views/File/Editor/config/sqlLanguage';
import { getEditorTheme } from '$lib/components/views/File/Editor/config/editorTheme';

const COLORIZE_THEME = 'sql-schema-theme';

/**
 * Monaco, loaded the first time something asks for colour and not before: this
 * runs from panels that are mounted long before any SQL is shown, and pulling
 * 3.7MB in for them delays the app's first paint.
 */
let loading: Promise<typeof Monaco> | null = null;

function monaco(): Promise<typeof Monaco> {
	loading ??= (async () => {
		await import('$lib/components/views/File/Editor/config/monacoWorkers');
		const m = await import('monaco-editor');
		m.languages.register({ id: 'sql' });
		m.languages.setMonarchTokensProvider('sql', sqlLanguage as Monaco.languages.IMonarchLanguage);
		m.editor.defineTheme(COLORIZE_THEME, getEditorTheme() as Monaco.editor.IStandaloneThemeData);
		return m;
	})();
	return loading;
}

// Static SQL highlighting for read-only previews, without a full editor instance.
// Returns themed HTML for use with {@html}; input is trusted app SQL.
export async function colorizeSql(sql: string): Promise<string> {
	const m = await monaco();
	m.editor.setTheme(COLORIZE_THEME);
	return m.editor.colorize(sql, 'sql', { tabSize: 2 });
}
