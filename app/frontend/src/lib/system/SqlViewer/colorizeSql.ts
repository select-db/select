import * as monaco from 'monaco-editor';

import { sqlLanguage } from '$lib/components/views/File/Editor/config/sqlLanguage';
import { getEditorTheme } from '$lib/components/views/File/Editor/config/editorTheme';

const COLORIZE_THEME = 'sql-schema-theme';
let registered = false;

function ensureRegistered(): void {
	if (registered) return;
	registered = true;
	monaco.languages.register({ id: 'sql' });
	monaco.languages.setMonarchTokensProvider('sql', sqlLanguage as monaco.languages.IMonarchLanguage);
	monaco.editor.defineTheme(COLORIZE_THEME, getEditorTheme() as monaco.editor.IStandaloneThemeData);
}

// Static SQL highlighting for read-only previews, without a full editor instance.
// Returns themed HTML for use with {@html}; input is trusted app SQL.
export async function colorizeSql(sql: string): Promise<string> {
	ensureRegistered();
	monaco.editor.setTheme(COLORIZE_THEME);
	return monaco.editor.colorize(sql, 'sql', { tabSize: 2 });
}
