import * as monaco from 'monaco-editor';
import { Complete } from '$lib/wailsjs/go/sqllang/SqlLang';
import type { graph, sqllang } from '$lib/wailsjs/go/models';

export function createSqlCompletionProvider(
	getFile: () => graph.FileNode | null
): monaco.languages.CompletionItemProvider {
	return {
		triggerCharacters: ['.', ' ', '"'],
		provideCompletionItems: async (model, position) => {
			const file = getFile();
			if (!file) {
				return { suggestions: [] };
			}

			const dbId = file.databases?.[0]?.id;
			if (!dbId) return { suggestions: [] };

			try {
				const params: sqllang.PositionParams = {
					DbInstanceID: dbId,
					FileID: file.id,
					SQL: model.getValue(),
					Line: position.lineNumber,
					Column: position.column - 1
				};

				const result = await Complete(params);

				if (result.errors && result.errors.length > 0) {
					console.warn('Completion errors:', result.errors);
				}

				if (!result.candidates) return { suggestions: [] };

				const wordUntil = model.getWordUntilPosition(position);
				let replaceEndColumn = wordUntil.endColumn;
				const charAfterWord = model.getValueInRange({
					startLineNumber: position.lineNumber,
					endLineNumber: position.lineNumber,
					startColumn: wordUntil.endColumn,
					endColumn: wordUntil.endColumn + 1
				});

				// Swallow trailing " for partial identifiers, not empty word
				if (charAfterWord === '"' && wordUntil.word.length > 0) {
					replaceEndColumn = wordUntil.endColumn + 1;
				}

				const insideDoubleQuotes = charAfterWord === '"';
				const insideSingleQuotes = charAfterWord === "'";

				// Inside "..." with empty word: eat leading spaces
				let replaceStartColumn = wordUntil.startColumn;
				if (insideDoubleQuotes && wordUntil.word.length === 0 && replaceStartColumn > 1) {
					while (replaceStartColumn > 1) {
						const ch = model.getValueInRange({
							startLineNumber: position.lineNumber,
							endLineNumber: position.lineNumber,
							startColumn: replaceStartColumn - 1,
							endColumn: replaceStartColumn
						});
						if (ch !== ' ') break;
						replaceStartColumn--;
					}
				}

				const suggestions = result.candidates.map((candidate) => {
					const hasSnippet = candidate.InsertText && candidate.InsertText !== candidate.Text;
					let insertText = candidate.InsertText || candidate.Text;
					if (
						!hasSnippet &&
						insideDoubleQuotes &&
						insertText.length >= 2 &&
						insertText.startsWith('"') &&
						insertText.endsWith('"')
					) {
						insertText = insertText.slice(1, -1);
					}
					if (
						!hasSnippet &&
						insideSingleQuotes &&
						insertText.length >= 2 &&
						insertText.startsWith("'") &&
						insertText.endsWith("'")
					) {
						insertText = insertText.slice(1, -1);
					}

					let detail = candidate.Comment || '';
					if (candidate.Definition) {
						detail = detail ? `${detail} (${candidate.Definition})` : candidate.Definition;
					}

					return {
						label: candidate.Text,
						kind: candidate.kind,
						detail: detail,
						insertText,
						insertTextRules: hasSnippet
							? monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet
							: undefined,
						range: {
							startLineNumber: position.lineNumber,
							endLineNumber: position.lineNumber,
							startColumn: replaceStartColumn,
							endColumn: replaceEndColumn
						}
					};
				});

				return { suggestions };
			} catch (error) {
				console.error('Completion error:', error);
				return { suggestions: [] };
			}
		}
	};
}
