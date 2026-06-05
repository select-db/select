/**
 * Cmd/Ctrl + hover underline + click navigation for SQL editors.
 * Two concrete behaviours:
 * 		- schema item modal
 * 		- variable file open
 * Single generic infrastructure via attachCmdClickNavigation<T>.
 */
import * as monaco from 'monaco-editor';
import { addTab } from '$lib/components/Layout/layoutStore';
import { modalStore } from '$lib/system/Modal/ModalStore';
import { tryCatch } from '$lib/utils/tryCatch';
import { Resolve } from '$lib/wailsjs/go/sqllang/SqlLang';
import * as graphApi from '$lib/wailsjs/go/graph/Graph';
import { loadDatabase } from '$lib/components/views/Chat/tools/helpers';
import ItemInfoModal from '$lib/components/views/FileSystem/modals/ItemInfoModal.svelte';
import { graph } from '$lib/wailsjs/go/models';

const DEBOUNCE_MS = 50;
const VAR_RE = /\$[A-Za-z_][A-Za-z0-9_]*/g;

// Prevents multiple attachCmdClickNavigation instances on the same editor from
// both activating on a single click. The first to fire consumes the click;
// subsequent ones skip. Reset via rAF after all synchronous handlers have run.
const clickConsumed = new WeakMap<monaco.editor.IStandaloneCodeEditor, boolean>();

function getSqlVariableTokenAtPosition(
	model: monaco.editor.ITextModel,
	position: monaco.Position
): { name: string; range: monaco.Range } | null {
	const line = model.getLineContent(position.lineNumber);
	if (!line) return null;

	const col0 = position.column - 1;
	let m: RegExpExecArray | null;
	VAR_RE.lastIndex = 0;

	while ((m = VAR_RE.exec(line)) !== null) {
		const start0 = m.index;
		const end0 = start0 + m[0].length;
		if (col0 < start0 || col0 > end0) continue;

		return {
			name: m[0].slice(1),
			range: new monaco.Range(position.lineNumber, start0 + 1, position.lineNumber, end0 + 1)
		};
	}
	return null;
}

function makeFileNode(uri: string, name: string): graph.FileNode {
	return new graph.FileNode({
		id: uri,
		uri,
		type: 'file',
		name: name || uri.split('/').pop() || 'file',
		folder_id: '',
		badges: []
	});
}

/**
 * Attaches Cmd/Ctrl+hover underline + click to an editor.
 * resolve()  called on debounced hover; return { range, data } to show underline, null to clear.
 * activate() called on click with the last cached data; no second async resolve needed.
 */
function attachCmdClickNavigation<T>(
	editor: monaco.editor.IStandaloneCodeEditor,
	resolve: (
		pos: monaco.Position,
		model: monaco.editor.ITextModel
	) => Promise<{ range: monaco.Range; data: T } | null>,
	activate: (data: T) => void | Promise<void>
): monaco.IDisposable {
	const decos = editor.createDecorationsCollection([]);
	let timer: ReturnType<typeof setTimeout> | null = null;
	let gen = 0;
	let lastTextPos: monaco.Position | null = null;
	let lastResolved: { pos: monaco.Position; data: T } | null = null;
	let runningHint: boolean = false;

	const clear = () => {
		if (runningHint) return;

		if (timer != null) {
			clearTimeout(timer);
			timer = null;
		}

		gen++;
		lastResolved = null;
		decos.clear();
	};

	const runHint = (pos: monaco.Position) => {
		if (runningHint) return;

		const model = editor.getModel();
		if (!model) return;

		runningHint = true;

		const g = ++gen;
		void (async () => {
			const result = await resolve(pos, model);

			if (g !== gen) return;
			if (!result) {
				lastResolved = null;
				decos.clear();
				return;
			}

			lastResolved = { pos, data: result.data };

			decos.set([
				{
					range: result.range,
					options: {
						inlineClassName: 'sql-schema-cmd-link',
						stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges
					}
				}
			]);
		})().finally(() => {
			runningHint = false;
		});
	};

	const schedule = (pos: monaco.Position) => {
		if (timer != null) clearTimeout(timer);

		timer = setTimeout(() => {
			timer = null;
			runHint(pos);
		}, DEBOUNCE_MS);
	};

	const subMove = editor.onMouseMove((e) => {
		const ev = e.event;
		if (e.target.type === monaco.editor.MouseTargetType.CONTENT_TEXT && e.target.position) {
			lastTextPos = e.target.position;
		}

		if (!(ev.metaKey || ev.ctrlKey)) {
			clear();
			return;
		}

		if (e.target.type !== monaco.editor.MouseTargetType.CONTENT_TEXT || !e.target.position) {
			clear();
			return;
		}

		schedule(e.target.position);
	});

	const subLeave = editor.onMouseLeave(() => clear());
	const subModelChange = editor.onDidChangeModel(() => clear());

	const isModifierOnly = (ke: KeyboardEvent) =>
		ke.code === 'MetaLeft' ||
		ke.code === 'MetaRight' ||
		ke.code === 'ControlLeft' ||
		ke.code === 'ControlRight';

	const onKeyDown = (ke: KeyboardEvent) => {
		if (!isModifierOnly(ke) || !lastTextPos) return;
		schedule(lastTextPos);
	};
	const onKeyUp = (ke: KeyboardEvent) => {
		if (ke.key === 'Meta' || ke.key === 'Control') clear();
	};
	window.addEventListener('keydown', onKeyDown);
	window.addEventListener('keyup', onKeyUp);

	const subDown = editor.onMouseDown((e) => {
		const ev = e.event;
		if (!(ev.metaKey || ev.ctrlKey)) return;

		const pos = e.target.position;
		if (!pos || e.target.type !== monaco.editor.MouseTargetType.CONTENT_TEXT) return;
		if (clickConsumed.get(editor)) return;

		ev.preventDefault();
		ev.stopPropagation();

		const cached = lastResolved;
		if (!cached || cached.pos.lineNumber !== pos.lineNumber || cached.pos.column !== pos.column)
			return;

		clickConsumed.set(editor, true);
		requestAnimationFrame(() => clickConsumed.delete(editor));
		void activate(cached.data);
	});

	return {
		dispose: () => {
			clear();
			subMove.dispose();
			subLeave.dispose();
			subModelChange.dispose();
			subDown.dispose();
			window.removeEventListener('keydown', onKeyDown);
			window.removeEventListener('keyup', onKeyUp);
		}
	};
}

export function attachSqlSchemaPointerNavigation(
	editor: monaco.editor.IStandaloneCodeEditor,
	getDbId: () => string | null | undefined,
	getFileId: () => string | null | undefined
): monaco.IDisposable {
	return attachCmdClickNavigation<graph.DBInstanceItemNode>(
		editor,
		async (pos, model) => {
			const dbId = getDbId();
			if (!dbId) return null;

			// If the cursor is inside a $variable token, let variable navigation handle it.
			if (getSqlVariableTokenAtPosition(model, pos)) return null;

			const w = model.getWordAtPosition(pos);
			if (!w?.word?.trim()) return null;

			const [r, err] = await tryCatch(Resolve, {
				DbInstanceID: dbId,
				FileID: getFileId() ?? '',
				SQL: model.getValue(),
				Line: pos.lineNumber,
				Column: pos.column - 1
			});
			if (err || !r?.found || !r.node) return null;

			const w2 = model.getWordAtPosition(pos);
			if (!w2?.word?.trim()) return null;

			return {
				range: new monaco.Range(pos.lineNumber, w2.startColumn, pos.lineNumber, w2.endColumn),
				data: r.node
			};
		},
		async (node) => {
			const dbId = getDbId();
			if (!dbId) return;

			await loadDatabase(dbId);
			modalStore.set({ content: () => ItemInfoModal, props: { item: node }, width: 600 });
		}
	);
}

type VarNavData = { uri: string; name: string };

export function attachSqlVariablePointerNavigation(
	editor: monaco.editor.IStandaloneCodeEditor,
	getFile: () => graph.FileNode | null
): monaco.IDisposable {
	return attachCmdClickNavigation<VarNavData>(
		editor,
		async (pos, model) => {
			const file = getFile();
			if (!file) return null;

			const token = getSqlVariableTokenAtPosition(model, pos);
			if (!token) return null;

			const [fileResult, envResult] = await Promise.all([
				tryCatch(graphApi.GetUriSqlFileRefs, file.uri),
				tryCatch(graphApi.GetUriVariables, file.uri)
			]);
			const fileRefs = Array.isArray(fileResult?.[0]) ? fileResult[0] : [];
			const envVars = Array.isArray(envResult?.[0]) ? envResult[0] : [];

			const fileRef = fileRefs.find((f) => f.name === token.name);
			if (fileRef?.uri)
				return { range: token.range, data: { uri: fileRef.uri, name: fileRef.name } };

			const envVar = envVars.find((v) => v.name === token.name);
			if (envVar?.source_uri)
				return { range: token.range, data: { uri: envVar.source_uri, name: envVar.source } };

			return null;
		},
		({ uri, name }) => {
			addTab(makeFileNode(uri, name));
		}
	);
}
