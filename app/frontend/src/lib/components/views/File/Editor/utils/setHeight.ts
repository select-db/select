import * as monaco from 'monaco-editor';

export const setHeight = (
	container: HTMLElement | null,
	editor: monaco.editor.IStandaloneCodeEditor | null
) => {
	if (!editor || !container) return;
	const contentHeight = editor.getContentHeight();
	container.style.height = `${contentHeight}px`;
	editor.layout({ width: container.clientWidth, height: contentHeight });
};
