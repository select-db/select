<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import * as monaco from 'monaco-editor';
	import '../Editor/config/monacoWorkers';
	import { refreshTheme } from '../Editor/config/languageSetup';
	import { EDITOR_THEME_NAME } from '../Editor/config/editorTheme';
	import Button from '$lib/system/Button/Button.svelte';

	type Props = {
		value: string;
		dataType?: string;
		columnName?: string;
		onSave: (value: string) => void;
		onClose: () => void;
	};

	let { value, dataType, columnName, onSave, onClose }: Props = $props();

	const language = languageFor(dataType);
	let jsonError = $state<string | null>(null);

	let container: HTMLDivElement;
	let editor: monaco.editor.IStandaloneCodeEditor | null = null;
	let model: monaco.editor.ITextModel | null = null;

	function languageFor(type: string | undefined): string {
		const t = (type ?? '').toLowerCase();
		if (t.includes('json')) return 'json';
		if (t.includes('xml')) return 'xml';
		return 'plaintext';
	}

	function validate(text: string) {
		if (language !== 'json' || text.trim() === '' || text.trim() === 'NULL') {
			jsonError = null;
			return;
		}
		try {
			JSON.parse(text);
			jsonError = null;
		} catch (e) {
			jsonError = e instanceof Error ? e.message : 'Invalid JSON';
		}
	}

	function save() {
		onSave(editor?.getValue() ?? value);
		onClose();
	}

	function setNull() {
		onSave('NULL');
		onClose();
	}

	function formatJson() {
		if (!editor || language !== 'json') return;
		try {
			editor.setValue(JSON.stringify(JSON.parse(editor.getValue()), null, 2));
		} catch {
			// invalid JSON: leave the text untouched, the hint already explains why
		}
	}

	onMount(() => {
		refreshTheme();
		model = monaco.editor.createModel(value, language);
		editor = monaco.editor.create(container, {
			model,
			theme: EDITOR_THEME_NAME,
			automaticLayout: true,
			minimap: { enabled: false },
			wordWrap: 'on',
			fontFamily: 'JetBrains Mono',
			fontSize: 12,
			fontWeight: '300',
			lineNumbersMinChars: 3,
			scrollBeyondLastLine: false,
			scrollbar: { verticalScrollbarSize: 6, horizontalScrollbarSize: 6 },
			fixedOverflowWidgets: true
		});
		editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, save);
		model.onDidChangeContent(() => validate(model!.getValue()));
		validate(value);
		editor.focus();
	});

	onDestroy(() => {
		editor?.dispose();
		model?.dispose();
	});
</script>

<div class="cell-modal-body">
	<div class="toolbar">
		<span class="title-wrapper">
			<span class="type-chip">{dataType || 'text'}</span>
			<span class="field-chip">{columnName}</span>
		</span>
		<div class="toolbar-right">
			{#if language === 'json'}
				<Button
					content="Format"
					leftIcon="code-bracket"
					emphasis="low"
					size="sm"
					onclick={formatJson}
				/>
			{/if}
		</div>
	</div>

	<div class="editor" bind:this={container}></div>

	{#if jsonError}
		<p class="hint">Invalid JSON: {jsonError}, you can still save.</p>
	{/if}
</div>

<div class="cell-modal-footer">
	<Button content="Set NULL" emphasis="low" onclick={setNull} />
	<div class="footer-main">
		<Button content="Cancel" emphasis="low" onclick={onClose} />
		<Button content="Save" emphasis="high" onclick={save} />
	</div>
</div>

<style>
	.cell-modal-body {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-height: 0;
		background-color: var(--gray-0);
	}

	.toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-sm);
		border-bottom: var(--border);
	}

	.title-wrapper {
		display: flex;
		gap: var(--space-xs-sm);
		align-items: center;
	}

	.type-chip {
		font-family: 'JetBrains Mono', monospace;
		font-size: 11px;
		color: var(--gray-800);
		background-color: var(--gray-200);
		padding: 2px var(--space-sm);
		border-radius: var(--br-xs);
	}

	.field-chip {
		font-family: 'JetBrains Mono', monospace;
		font-size: var(--fs-sm);
		color: var(--gray-1000);
		font-weight: 200;
	}

	.editor {
		flex: 1;
		min-height: 0;
		width: 100%;
	}

	.hint {
		margin: 0;
		padding: var(--space-sm);
		font-size: 12px;
		color: var(--orange, var(--gray-800));
		border-top: var(--border);
	}

	.cell-modal-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-sm);
		background-color: var(--gray-0);
		padding: var(--space-sm);
		border-top: var(--border);
	}

	.footer-main {
		display: flex;
		gap: var(--space-sm);
	}
</style>
