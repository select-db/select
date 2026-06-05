import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker';
import cssWorker from 'monaco-editor/esm/vs/language/css/css.worker?worker';
import jsonWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker';
import htmlWorker from 'monaco-editor/esm/vs/language/html/html.worker?worker';
import tsWorker from 'monaco-editor/esm/vs/language/typescript/ts.worker?worker';

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
