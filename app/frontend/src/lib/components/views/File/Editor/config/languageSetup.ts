import * as monaco from 'monaco-editor';
import { sqlLanguage } from './sqlLanguage';
import { getEditorTheme, EDITOR_THEME_NAME } from './editorTheme';
import { dotenvLanguage } from './dotenvLanguage';
import { createSqlCompletionProvider } from '../providers/sqlCompletionProvider';
import { createSqlHoverProvider } from '../providers/sqlHoverProvider';
import { createSqlSnippetProvider } from '../providers/sqlSnippetProvider';
import { createVariableCompletionProvider } from '../providers/variableCompletionProvider';
import { createSqlLintCodeActionsProvider } from '../providers/sqlLintCodeActionsProvider';
import type { graph } from '$lib/wailsjs/go/models';

// Track whether languages have been initialized to prevent duplicate registration
let languagesInitialized = false;

// Global file getter that can be updated by editors
let activeFileGetter: (() => graph.FileNode | null) | null = null;

/**
 * Sets the active file getter function. Called by the focused editor.
 */
export function setActiveFileGetter(getter: () => graph.FileNode | null) {
	activeFileGetter = getter;
}

/**
 * Clears the active file getter. Called when an editor is destroyed.
 */
export function clearActiveFileGetter(getter: () => graph.FileNode | null) {
	// Only clear if this is the current getter (to handle race conditions)
	if (activeFileGetter !== getter) return;
	activeFileGetter = null;
}

/**
 * Re-registers the editor theme. Call this when CSS variables may have changed.
 * Monaco themes are defined globally, so this updates all editors.
 */
export function refreshTheme() {
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	monaco.editor.defineTheme(EDITOR_THEME_NAME, getEditorTheme() as any);
}

// Common language configuration for brackets and auto-surround
const defaultLanguageConfig: monaco.languages.LanguageConfiguration = {
	brackets: [
		['(', ')'],
		['[', ']'],
		['{', '}']
	],
	autoClosingPairs: [
		{ open: '(', close: ')' },
		{ open: '[', close: ']' },
		{ open: '{', close: '}' },
		{ open: '"', close: '"' },
		{ open: "'", close: "'" },
		{ open: '`', close: '`' }
	],
	surroundingPairs: [
		{ open: '(', close: ')' },
		{ open: '[', close: ']' },
		{ open: '{', close: '}' },
		{ open: '"', close: '"' },
		{ open: "'", close: "'" },
		{ open: '`', close: '`' }
	]
};

/**
 * Initializes Monaco language configurations and completion providers.
 * This should be called once when the first editor is created.
 * Subsequent calls are no-ops.
 */
export function initializeLanguages() {
	if (languagesInitialized) return;
	languagesInitialized = true;

	// Register SQL language (use custom ID to avoid conflicts with Monaco's built-in SQL)
	monaco.languages.register({ id: 'sql-custom' });
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	monaco.languages.setMonarchTokensProvider('sql-custom', sqlLanguage as any);
	monaco.languages.setLanguageConfiguration('sql-custom', defaultLanguageConfig);

	// Register dotenv language
	monaco.languages.register({ id: 'dotenv' });
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	monaco.languages.setMonarchTokensProvider('dotenv', dotenvLanguage as any);
	monaco.languages.setLanguageConfiguration('dotenv', defaultLanguageConfig);

	// Register JSON schemas for .lint and .config so Monaco validates and
	// autocompletes their fields automatically.
	monaco.json.jsonDefaults.setDiagnosticsOptions({
		validate: true,
		schemas: [
			{
				uri: 'selectdb://schemas/lint',
				fileMatch: ['.lint'],
				schema: {
					type: 'array',
					items: {
						type: 'object',
						properties: {
							files: {
								type: 'array',
								items: { type: 'string' },
								description: 'Glob patterns. Entry only applies to matching files. Omit for global scope.'
							},
							ignores: {
								type: 'array',
								items: { type: 'string' },
								description: 'Skip these files entirely. Must be the only field in the entry.'
							},
							rules: {
								type: 'object',
								description: 'Override built-in rule severities.',
								additionalProperties: {
									oneOf: [
										{
											type: 'string',
											enum: ['off', 'hint', 'warning', 'error']
										},
										{
											type: 'array',
											prefixItems: [
												{ type: 'string', enum: ['off', 'hint', 'warning', 'error'] }
											],
											minItems: 1
										}
									]
								}
							},
							custom: {
								type: 'array',
								description: 'User-defined regexp rules.',
								items: {
									type: 'object',
									required: ['id', 'severity', 'message', 'pattern'],
									properties: {
										id: {
											type: 'string',
											description: 'Unique rule identifier. Use the C prefix by convention (C001, C002…).'
										},
										severity: {
											type: 'string',
											enum: ['off', 'hint', 'warning', 'error']
										},
										message: {
											type: 'string',
											description: 'Text shown in the editor when the rule matches.'
										},
										pattern: {
											type: 'string',
											description: 'Go-compatible regexp. Use (?i) for case-insensitive matching and [\\s\\S] to match newlines.'
										}
									},
									additionalProperties: false
								}
							}
						},
						additionalProperties: false
					}
				}
			},
			{
				uri: 'selectdb://schemas/config',
				fileMatch: ['.config'],
				schema: {
					type: 'object',
					properties: {
						keybindings: {
							type: 'object',
							description: 'Keybindings grouped by category (workbench, editor, modal, menu).',
							additionalProperties: {
								type: 'array',
								items: {
									type: 'object',
									required: ['key', 'command'],
									properties: {
										key: { type: 'string', description: 'Key combination (e.g. cmd+shift+p).' },
										command: { type: 'string', description: 'Command identifier to trigger.' },
										when: { type: 'string', description: 'Optional condition expression.' }
									},
									additionalProperties: false
								}
							}
						},
						editor_snippets: {
							type: 'array',
							description: 'Custom editor snippets.',
							items: {
								type: 'object',
								required: ['prefix', 'body'],
								properties: {
									prefix: { type: 'string', description: 'Trigger word for the snippet.' },
									body: { type: 'string', description: 'Snippet content.' },
									description: { type: 'string' }
								},
								additionalProperties: false
							}
						}
					},
					additionalProperties: false
				}
			}
		]
	});

	// Apply default config to built-in languages used in the app
	monaco.languages.setLanguageConfiguration('json', defaultLanguageConfig);
	monaco.languages.setLanguageConfiguration('css', defaultLanguageConfig);
	monaco.languages.setLanguageConfiguration('plaintext', defaultLanguageConfig);

	// Register completion providers globally (once)
	// They use the activeFileGetter to get context from the focused editor
	const getActiveFile = () => activeFileGetter?.() ?? null;
	monaco.languages.registerCompletionItemProvider(
		'sql-custom',
		createSqlSnippetProvider()
	);
	monaco.languages.registerCompletionItemProvider(
		'sql-custom',
		createSqlCompletionProvider(getActiveFile)
	);
	monaco.languages.registerCompletionItemProvider(
		'sql-custom',
		createVariableCompletionProvider(getActiveFile, { allowSqlFileRefs: true })
	);
	monaco.languages.registerHoverProvider('sql-custom', createSqlHoverProvider(getActiveFile));
	monaco.languages.registerCodeActionProvider('sql-custom', createSqlLintCodeActionsProvider());

}
