import * as monaco from 'monaco-editor';
import { Lint } from '$lib/wailsjs/go/sqllang/SqlLang';
import type { graph, sqllang } from '$lib/wailsjs/go/models';
import { debounce } from '$lib/utils/debounce';

export const LINT_SOURCE = 'sql-lint';

const DEBOUNCE_MS = 500;

/**
 * Attaches a debounced SQL lint provider to a Monaco editor.
 * After each content change (debounced at 500 ms) it calls the backend Lint()
 * Returns a disposable that clears markers and cancels the listener.
 */
export function attachSqlLintProvider(
	editor: monaco.editor.IStandaloneCodeEditor,
	getFile: () => graph.FileNode | null,
	onMarkersSet: (markers: monaco.editor.IMarkerData[]) => void
): monaco.IDisposable {
	let disposed = false;

	const runLint = debounce(async () => {
		if (disposed) return;

		const m = editor.getModel();
		if (!m || m.isDisposed()) return;

		const file = getFile();
		const dbId = file?.databases?.[0]?.id;
		if (!file || !dbId) {
			monaco.editor.setModelMarkers(m, LINT_SOURCE, []);
			return;
		}

		const params: sqllang.PositionParams = {
			DbInstanceID: dbId,
			FileID: file.id,
			SQL: m.getValue(),
			Line: 0,
			Column: 0
		};

		try {
			const result = await Lint(params);
			if (disposed) return;
			if (!result.diagnostics) {
				monaco.editor.setModelMarkers(m, LINT_SOURCE, []);
				onMarkersSet([]);
				return;
			}

			const markers: monaco.editor.IMarkerData[] = result.diagnostics.map((d) => ({
				startLineNumber: d.startLine,
				startColumn: d.startCol + 1, // Monaco columns are 1-based
				endLineNumber: d.endLine,
				endColumn: d.endCol + 1,
				message: d.message,
				code: d.ruleId,
				severity: d.severity as monaco.MarkerSeverity,
				source: LINT_SOURCE
			}));

			monaco.editor.setModelMarkers(m, LINT_SOURCE, markers);
			onMarkersSet(markers);
		} catch {
			if (disposed || m.isDisposed()) return;
			monaco.editor.setModelMarkers(m, LINT_SOURCE, []);
			onMarkersSet([]);
		}
	}, DEBOUNCE_MS);

	// Run once on attach, then on every content change.
	runLint();
	const disposable = editor.onDidChangeModelContent(() => runLint());

	return {
		dispose: () => {
			disposed = true;
			disposable.dispose();
		}
	};
}
