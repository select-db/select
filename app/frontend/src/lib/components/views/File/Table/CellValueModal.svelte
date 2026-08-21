<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import * as monaco from 'monaco-editor';
	import '../Editor/config/monacoWorkers';
	import { refreshTheme } from '../Editor/config/languageSetup';
	import { EDITOR_THEME_NAME } from '../Editor/config/editorTheme';
	import Button from '$lib/system/Button/Button.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';

	type Props = {
		value: string;
		dataType?: string;
		columnName?: string;
		/** Viewer mode: the value can be read and formatted, but not written back. */
		readOnly?: boolean;
		/** Why writing is off, shown next to the read-only chip. */
		readOnlyReason?: string;
		onSave?: (value: string) => void;
		onClose: () => void;
	};

	let {
		value,
		dataType,
		columnName,
		readOnly = false,
		readOnlyReason,
		onSave,
		onClose
	}: Props = $props();

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
		if (readOnly) return;
		onSave?.(editor?.getValue() ?? value);
		onClose();
	}

	function setNull() {
		if (readOnly) return;
		onSave?.('NULL');
		onClose();
	}

	async function handleSave() {
		save();
	}

	async function handleSetNull() {
		setNull();
	}

	async function handleCancel() {
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
			fixedOverflowWidgets: true,
			readOnly,
			domReadOnly: readOnly
		});
		if (!readOnly) editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, save);
		model.onDidChangeContent(() => validate(model!.getValue()));
		validate(value);
		editor.focus();
	});

	onDestroy(() => {
		editor?.dispose();
		model?.dispose();
	});
</script>

<div class="toolbar">
	<span class="title-wrapper">
		<span class="type-chip">{dataType || 'text'}</span>
		<span class="field-chip">{columnName}</span>
	</span>
	<div class="toolbar-right">
		{#if readOnly}
			<span class="readonly-chip">Read only</span>
			{#if readOnlyReason}
				<span class="readonly-reason">{readOnlyReason}</span>
			{/if}
		{/if}
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
	<p class="hint">Invalid JSON: {jsonError}{readOnly ? '' : ', you can still save.'}</p>
{/if}

{#if readOnly}
	<ModalFooter mainAction={{ label: 'Close', action: handleCancel }} />
{:else}
	<ModalFooter
		leftAction={{ label: 'Set NULL', action: handleSetNull }}
		secondaryAction={{ label: 'Cancel', action: handleCancel }}
		mainAction={{ label: 'Save', action: handleSave }}
	/>
{/if}

<style>
	.toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-sm);
		border-bottom: var(--border);
		background-color: var(--gray-200);
	}

	.toolbar-right {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
	}

	.readonly-chip {
		font-size: 11px;
		color: var(--gray-800);
		border: var(--border);
		border-radius: var(--br-xs);
		padding: 2px var(--space-sm);
	}

	.readonly-reason {
		font-size: var(--fs-xs);
		color: var(--gray-700);
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
		background-color: var(--gray-0);
	}
</style>
