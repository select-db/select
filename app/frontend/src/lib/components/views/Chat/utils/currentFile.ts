import { writable } from 'svelte/store';
import type * as monaco from 'monaco-editor';
import type { Tab } from '$lib/components/Layout/layoutStore';

export type CursorPosition = {
	lineNumber: number;
	column: number;
};

export type TextSelection = {
	startLineNumber: number;
	startColumn: number;
	endLineNumber: number;
	endColumn: number;
	selectedText?: string;
};

export type CurrentFile = {
	uri: string;
	id: string;
	folderId?: string;
	cursorPosition?: CursorPosition;
	textSelection?: TextSelection;
};

export const currentFileStore = writable<CurrentFile | null>(null);

export function setCurrentFile(file: CurrentFile | null): void {
	currentFileStore.set(file);
}

export function syncCurrentFileFromEditor(ed: monaco.editor.ICodeEditor, tab: Tab): void {
	const file = tab.file?.node;
	if (!file) return;
	const position = ed.getPosition();
	const selection = ed.getSelection();
	const model = ed.getModel();
	const selectedText = model && selection ? model.getValueInRange(selection) : undefined;
	queueMicrotask(() =>
		setCurrentFile({
			uri: tab.uri,
			id: file.id,
			folderId: file.folder_id || undefined,
			cursorPosition: position
				? { lineNumber: position.lineNumber, column: position.column }
				: undefined,
			textSelection:
				selection && !selection.isEmpty()
					? {
							startLineNumber: selection.startLineNumber,
							startColumn: selection.startColumn,
							endLineNumber: selection.endLineNumber,
							endColumn: selection.endColumn,
							...(selectedText !== undefined && { selectedText })
						}
					: undefined
		})
	);
}
