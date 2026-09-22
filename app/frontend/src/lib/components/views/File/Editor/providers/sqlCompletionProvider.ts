import * as monaco from 'monaco-editor';
import { Complete } from '$lib/bindings/selectDb/internal/sqllang/sqllang';
import type * as graph from '$lib/wails/graph';
import type * as sqllang from '$lib/bindings/selectDb/internal/sqllang/models';

/**
 * How far up the list a kind of candidate belongs. Monaco sorts by sortText and
 * falls back to the label, so without this the list is one alphabetical run and
 * a column of the table being read sits below two hundred function names.
 *
 * The numbers are the Monaco kinds `candidateTypeToMonacoKind` produces. Ranks
 * start at 1 because the snippet provider claims 0.
 */
const RANK_BY_KIND = new Map<number, number>([
	[0, 1], // Text: an operator, offered only where one is the answer
	[16, 1], // EnumMember: a value of the column being compared
	[5, 2], // Field: a column
	[7, 3], // Class: a table, a foreign table, a type
	[11, 3], // Interface: a view, a materialized view
	[2, 4], // Module: a schema
	[14, 5], // Keyword
	[1, 6] // Function
]);

function sortTextFor(kind: number, label: string): string {
	return `${RANK_BY_KIND.get(kind) ?? 5}_${label.toLowerCase()}`;
}

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
						sortText: sortTextFor(candidate.kind, candidate.Text),
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
