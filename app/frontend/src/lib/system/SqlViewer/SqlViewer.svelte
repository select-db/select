<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import * as monaco from 'monaco-editor';
	import { format as formatSQL } from 'sql-formatter';

	import { sqlLanguage } from '$lib/components/views/File/Editor/config/sqlLanguage';
	import { getEditorTheme } from '$lib/components/views/File/Editor/config/editorTheme';
	import { editorConfig } from '$lib/components/views/File/Editor/config/editorConfig';
	import { tryCatch } from '$lib/utils/tryCatch';

	let {
		sql = '',
		className = 'surface',
		format = false
	}: {
		sql: string;
		className?: string;
		format?: boolean;
	} = $props();

	function formatSqlString(input: string): string {
		if (!input) return input;
		const [formatted, err] = tryCatch(formatSQL, input, {
			language: 'postgresql',
			keywordCase: 'upper',
			tabWidth: 2
		});
		return err ? input : formatted;
	}

	const displaySql = $derived(format ? formatSqlString(sql) : sql);

	let container: HTMLElement | null = null;
	let editor: monaco.editor.IStandaloneCodeEditor | null = null;
	let model: monaco.editor.ITextModel | null = null;

	onMount(() => {
		if (!container) return;

		monaco.languages.register({ id: 'sql' });
		monaco.languages.setMonarchTokensProvider(
			'sql',
			sqlLanguage as monaco.languages.IMonarchLanguage
		);
		monaco.editor.defineTheme(
			'sql-schema-theme',
			getEditorTheme() as monaco.editor.IStandaloneThemeData
		);

		model = monaco.editor.createModel(displaySql, 'sql');

		editor = monaco.editor.create(container, {
			...editorConfig,
			model,
			theme: 'sql-schema-theme',
			useShadowDOM: false,
			readOnly: true,
			lineNumbers: 'off',
			glyphMargin: false,
			lineDecorationsWidth: 0,
			lineNumbersMinChars: 0,
			guides: {
				indentation: false
			},
			padding: {
				top: 10
			}
		});
	});

	$effect(() => {
		if (!editor || !model || !displaySql) return;
		model.setValue(displaySql);
	});

	export function scrollToTop() {
		editor?.revealLine(1);
	}

	onDestroy(() => {
		model?.dispose();
		editor?.dispose();
	});
</script>

<div class={`editorContainer ${className}`} bind:this={container}></div>

<style>
	.editorContainer {
		height: 100%;
		width: 100%;
	}

	/* surface */
	:global(.editorContainer.surface .monaco-editor) {
		--vscode-editorStickyScroll-background: var(--gray-300);
		--vscode-editor-background: var(--gray-300);
	}
	:global(.editorContainer.surface .monaco-editor .margin) {
		background-color: var(--gray-300);
	}

	/* surface-light */
	:global(.editorContainer.surface-light .monaco-editor) {
		--vscode-editorStickyScroll-background: var(--gray-300);
		--vscode-editor-background: var(--gray-300);
	}
	:global(.editorContainer.surface-light .monaco-editor .margin) {
		background-color: var(--gray-300);
	}
</style>
