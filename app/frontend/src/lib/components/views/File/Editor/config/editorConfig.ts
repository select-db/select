import type * as monaco from 'monaco-editor';
import { EDITOR_THEME_NAME } from './editorTheme';

export const editorConfig: monaco.editor.IStandaloneEditorConstructionOptions = {
	theme: EDITOR_THEME_NAME,
	language: 'sql',
	'semanticHighlighting.enabled': true,
	lineNumbersMinChars: 4,
	automaticLayout: true,
	minimap: {
		enabled: false
	},
	fontFamily: 'JetBrains Mono',
	fontSize: 12,
	fontWeight: '200',
	scrollbar: {
		verticalScrollbarSize: 5,
		horizontalScrollbarSize: 5,
		alwaysConsumeMouseWheel: false
	},
	fixedOverflowWidgets: true,
	quickSuggestions: {
		other: true,
		comments: false,
		strings: false
	},
	parameterHints: {
		enabled: false
	},
	autoSurround: 'languageDefined',
	autoClosingBrackets: 'languageDefined',
	autoClosingQuotes: 'languageDefined'
};
