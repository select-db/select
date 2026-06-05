import * as monaco from 'monaco-editor';
import { get } from 'svelte/store';
import type { graph } from '$lib/wailsjs/go/models';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';

function addNamesFromFunctionsFolder(folder: graph.DBInstanceItemNode, names: Set<string>): void {
	for (const leaf of folder.children ?? []) {
		if (leaf.type !== 'function') continue;
		const meta = leaf.metadata as Record<string, unknown> | undefined | null;
		let raw =
			meta && typeof meta.name === 'string'
				? meta.name
				: (String(leaf.name ?? '').split('(')[0] ?? '').trim();
		raw = raw.trim();
		if (raw) names.add(raw.toLowerCase());
	}
}

function collectSqlFunctionNamesForDbInstance(
	workspace: graph.WorkspaceNode | undefined,
	dbInstanceId: string
): Set<string> {
	const names = new Set<string>();
	if (!workspace || !dbInstanceId) return names;
	const db = (workspace.db_instances ?? []).find((d) => d.id === dbInstanceId);
	if (!db?.children?.length) return names;
	for (const top of db.children) {
		if (top.type === 'schema') {
			for (const child of top.children ?? []) {
				if (child.type === 'functions') {
					addNamesFromFunctionsFolder(child, names);
				}
			}
		} else if (top.type === 'system_catalog') {
			for (const section of top.children ?? []) {
				if (section.type === 'functions') addNamesFromFunctionsFolder(section, names);
			}
		}
	}
	return names;
}

interface LineScanState {
	blockComment: boolean;
}

/**
 * Find `identifier` or `a.b.c` immediately before `(` in code (skips strings, --, block comments).
 * Returns 1-based column start and length for the final identifier only.
 */
function collectCallSitesOnLine(
	line: string,
	state: LineScanState
): { startCol: number; len: number }[] {
	const matches: { startCol: number; len: number }[] = [];
	const n = line.length;
	let i = 0;

	while (i < n) {
		if (state.blockComment) {
			const end = line.indexOf('*/', i);
			if (end === -1) return matches;
			state.blockComment = false;
			i = end + 2;
			continue;
		}

		const c = line[i];
		if (c === '-' && line[i + 1] === '-') break;
		if (c === '/' && line[i + 1] === '*') {
			state.blockComment = true;
			i += 2;
			continue;
		}
		if (c === "'") {
			i++;
			while (i < n) {
				if (line[i] === "'" && line[i + 1] === "'") {
					i += 2;
					continue;
				}
				if (line[i] === "'") {
					i++;
					break;
				}
				i++;
			}
			continue;
		}
		if (c === '"') {
			i++;
			while (i < n) {
				if (line[i] === '"' && line[i + 1] === '"') {
					i += 2;
					continue;
				}
				if (line[i] === '"') {
					i++;
					break;
				}
				i++;
			}
			continue;
		}
		if (/[a-zA-Z_]/.test(c)) {
			const start = i++;
			while (i < n && /[a-zA-Z0-9_]/.test(line[i])) i++;
			const name = line.slice(start, i);
			let j = i;
			while (j < n && /\s/.test(line[j])) j++;
			if (j < n && line[j] === '.') {
				i = j + 1;
				continue;
			}
			if (j < n && line[j] === '(') matches.push({ startCol: start + 1, len: name.length });
			i = j;
			continue;
		}
		i++;
	}

	return matches;
}

function buildHighlightDecorations(
	model: monaco.editor.ITextModel,
	names: Set<string>
): monaco.editor.IModelDeltaDecoration[] {
	const decorations: monaco.editor.IModelDeltaDecoration[] = [];
	const state: LineScanState = { blockComment: false };
	for (let line = 1; line <= model.getLineCount(); line++) {
		const content = model.getLineContent(line);
		for (const s of collectCallSitesOnLine(content, state)) {
			const raw = content.slice(s.startCol - 1, s.startCol - 1 + s.len);
			if (names.has(raw.toLowerCase())) {
				decorations.push({
					range: new monaco.Range(line, s.startCol, line, s.startCol + s.len),
					options: {
						inlineClassName: 'sql-db-function',
						stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges
					}
				});
			}
		}
	}
	return decorations;
}

/**
 * Attaches DB-function highlighting to an editor instance.
 * Highlights SQL function call sites that match known functions from the workspace graph.
 * Returns a disposable, call dispose() when the editor is destroyed or the language changes.
 */
export function attachSqlFunctionHighlighter(
	editor: monaco.editor.IStandaloneCodeEditor,
	getDbId: () => string | null | undefined
): monaco.IDisposable {
	const decos = editor.createDecorationsCollection([]);

	const refresh = () => {
		const model = editor.getModel();
		const dbId = getDbId();
		if (!model || !dbId || model.isDisposed()) {
			decos.clear();
			return;
		}
		const names = collectSqlFunctionNamesForDbInstance(get(workspaceGraphStore), dbId);
		decos.set(buildHighlightDecorations(model, names));
	};

	const d1 = editor.onDidChangeModel(() => refresh());
	const d2 = editor.onDidChangeModelContent(() => refresh());
	const unsubGraph = workspaceGraphStore.subscribe(() => refresh());

	refresh();

	return {
		dispose: () => {
			d1.dispose();
			d2.dispose();
			unsubGraph();
			decos.clear();
		}
	};
}
