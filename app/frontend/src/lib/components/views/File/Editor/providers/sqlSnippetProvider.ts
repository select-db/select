import { get } from 'svelte/store';
import * as monaco from 'monaco-editor';
import { editorSnippetsStore } from '$lib/stores/keybindingsStore';
import { contextAt, textBeforeCursor } from './completionContext';

/**
 * Provides editor snippets from .config (merged defaults + user) for the SQL editor.
 */
export function createSqlSnippetProvider(): monaco.languages.CompletionItemProvider {
	return {
		triggerCharacters: ['.', ' '],
		provideCompletionItems: (model, position) => {
			// A snippet writes a statement, so it belongs where one could start.
			// `.` is a trigger character above, and after one the word below is
			// empty: the prefix filter then matches every snippet and sortText
			// pins them over the columns the SQL provider is offering.
			if (contextAt(textBeforeCursor(model, position)) !== 'open') {
				return { suggestions: [] };
			}

			const snippets = get(editorSnippetsStore);
			const wordUntil = model.getWordUntilPosition(position);
			const word = wordUntil.word.toLowerCase();

			const suggestions: monaco.languages.CompletionItem[] = [];

			for (const snippet of snippets) {
				const prefixLower = snippet.prefix.toLowerCase();
				if (word && !prefixLower.startsWith(word)) continue;

				const range: monaco.IRange = {
					startLineNumber: position.lineNumber,
					endLineNumber: position.lineNumber,
					startColumn: wordUntil.startColumn,
					endColumn: wordUntil.endColumn
				};

				suggestions.push({
					label: snippet.prefix,
					kind: monaco.languages.CompletionItemKind.Snippet,
					detail: snippet.description ?? undefined,
					insertText: snippet.body,
					insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
					range,
					sortText: '0_' + snippet.prefix
				});
			}

			return { suggestions };
		}
	};
}
