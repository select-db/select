import * as monaco from 'monaco-editor';
import { tryCatch } from '$lib/utils/tryCatch';
import * as graphApi from '$lib/wailsjs/go/graph/Graph';
import type { graph } from '$lib/wailsjs/go/models';

export type VariableCompletionOptions = {
	/** When true, suggest SQL files from the same folder as $name. Only enable in SQL file context. */
	allowSqlFileRefs?: boolean;
};

/**
 * Provides variable completions. Suggests env variables first,
 * then (if allowSqlFileRefs) SQL file refs.
 */
export function createVariableCompletionProvider(
	getFile: () => graph.FileNode | null,
	options: VariableCompletionOptions = {}
): monaco.languages.CompletionItemProvider {
	const { allowSqlFileRefs = false } = options;

	return {
		triggerCharacters: ['$'],
		provideCompletionItems: async (model, position) => {
			const file = getFile();
			if (!file) {
				return { suggestions: [] };
			}

			const textBeforeCursor = model.getValueInRange({
				startLineNumber: position.lineNumber,
				startColumn: Math.max(1, position.column - 20),
				endLineNumber: position.lineNumber,
				endColumn: position.column
			});

			const isVariableContext = textBeforeCursor.match(/\$[A-Za-z0-9_]*$/);

			if (isVariableContext) {
				try {
					const varsPromise = tryCatch(graphApi.GetUriVariables, file.uri);
					const filesPromise =
						allowSqlFileRefs && typeof graphApi.GetUriSqlFileRefs === 'function'
							? tryCatch(graphApi.GetUriSqlFileRefs, file.uri)
							: Promise.resolve([[], null] as const);
					const [varsResult, filesResult] = await Promise.all([varsPromise, filesPromise]);
					const variables = Array.isArray(varsResult?.[0]) ? varsResult[0] : [];
					const sqlFiles = Array.isArray(filesResult?.[0]) ? filesResult[0] : [];
					const wordBeforeCursor = model.getWordUntilPosition(position);
					const range = {
						startLineNumber: position.lineNumber,
						endLineNumber: position.lineNumber,
						startColumn: wordBeforeCursor.startColumn,
						endColumn: wordBeforeCursor.endColumn
					};

					// Variables first, then SQL file refs (only when allowSqlFileRefs)
					const suggestions: monaco.languages.CompletionItem[] = [
						...variables.map((variable: { name: string; value: string; source: string }) => ({
							label: `$${variable.name}`,
							kind: monaco.languages.CompletionItemKind.Variable,
							detail:
								variable.value.length > 50
									? `${variable.value.substring(0, 50)}...`
									: variable.value,
							documentation: `Source: ${variable.source}`,
							insertText: variable.name,
							range
						})),
						...sqlFiles.map((f: { name: string; preview: string; path: string }) => ({
							label: `$${f.name}`,
							kind: monaco.languages.CompletionItemKind.File,
							detail: f.preview ?? f.path ?? '',
							documentation: f.path ? `SQL file: ${f.path}` : undefined,
							insertText: f.name,
							range
						}))
					];

					return { suggestions };
				} catch (error) {
					console.error('Variable completion error:', error);
					return { suggestions: [] };
				}
			}

			return { suggestions: [] };
		}
	};
}
