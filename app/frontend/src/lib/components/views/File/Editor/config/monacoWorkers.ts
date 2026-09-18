// Monaco 0.56 added an `exports` map ("./*" -> "./esm/vs/*.js"), so the worker
// subpaths must be written without the `esm/vs/` prefix (it's now baked into the
// mapping). The old `monaco-editor/esm/vs/.../*.worker` paths resolve to a
// doubled `esm/vs/esm/vs/...` and fail under Vite 8's Rolldown bundler.
import editorWorker from 'monaco-editor/editor/editor.worker?worker';
import cssWorker from 'monaco-editor/language/css/css.worker?worker';
import jsonWorker from 'monaco-editor/language/json/json.worker?worker';
import htmlWorker from 'monaco-editor/language/html/html.worker?worker';
import tsWorker from 'monaco-editor/language/typescript/ts.worker?worker';

// Must be set before monaco-editor is imported so Monaco uses these
// workers instead of falling back to the broken AMD URL resolution.
self.MonacoEnvironment = {
	getWorker(_: string, label: string) {
		switch (label) {
			case 'css':
			case 'scss':
			case 'less':
				return new cssWorker();
			case 'json':
				return new jsonWorker();
			case 'html':
			case 'handlebars':
			case 'razor':
				return new htmlWorker();
			case 'typescript':
			case 'javascript':
				return new tsWorker();
			default:
				return new editorWorker();
		}
	}
};
