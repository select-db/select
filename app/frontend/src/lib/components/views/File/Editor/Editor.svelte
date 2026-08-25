<script lang="ts">
	import './config/monacoWorkers';
	import { onMount, onDestroy } from 'svelte';
	import * as monaco from 'monaco-editor';

	import { EventsOn } from '$lib/wails/events';
	import { GetOSPathFromURI } from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';

	import {
		updateTab,
		getAllGroups,
		activeGroupStore,
		type Tab
	} from '$lib/components/Layout/layoutStore';
	import { tryCatch } from '$lib/utils/tryCatch';
	import { syncCurrentFileFromEditor } from '$lib/components/views/Chat/utils/currentFile';
	import { setContext } from '$lib/stores/keybindingsContextStore';
	import { registerCommand, unregisterCommand } from '$lib/stores/commandRegistry';
	import { debounce } from '$lib/utils/debounce';
	import { format as formatSQL } from './utils/format';
	import { editorConfig } from './config/editorConfig';
	import {
		initializeLanguages,
		refreshTheme,
		setActiveFileGetter,
		clearActiveFileGetter
	} from './config/languageSetup';
	import { attachSqlFunctionHighlighter } from './config/sqlSemanticTokens';
	import {
		attachSqlSchemaPointerNavigation,
		attachSqlVariablePointerNavigation
	} from './config/sqlPointerNavigation';
	import { attachSqlLintProvider, LINT_SOURCE } from './providers/sqlLintProvider';
	import { getEffectiveSelectedDbId } from '../views/tableViewState';

	import Path from '../Header/Path.svelte';

	const DIFF_PANEL_MODIFIED = 1;

	const editorCommandMap: Record<string, string> = {
		'editor.undo': 'undo',
		'editor.redo': 'redo',
		'editor.selectAll': 'editor.action.selectAll',
		'editor.find': 'actions.find',
		'editor.replace': 'editor.action.startFindReplaceAction',
		'editor.findNext': 'editor.action.nextMatchFindAction',
		'editor.findPrevious': 'editor.action.previousMatchFindAction',
		'editor.addSelectionToNextMatch': 'editor.action.addSelectionToNextFindMatch',
		'editor.selectAllOccurrences': 'editor.action.selectHighlights',
		'editor.selectLine': 'expandLineSelection',
		'editor.toggleLineComment': 'editor.action.commentLine',
		'editor.outdent': 'outdent',
		'editor.indent': 'indent',
		'editor.cursorUndo': 'cursorUndo',
		'editor.deleteLine': 'editor.action.deleteLines',
		'editor.insertLineAfter': 'editor.action.insertLineAfter',
		'editor.insertLineBefore': 'editor.action.insertLineBefore',
		'editor.moveLineUp': 'editor.action.moveLinesUpAction',
		'editor.moveLineDown': 'editor.action.moveLinesDownAction',
		'editor.copyLineUp': 'editor.action.copyLinesUpAction',
		'editor.copyLineDown': 'editor.action.copyLinesDownAction',
		'editor.fold': 'editor.fold',
		'editor.unfold': 'editor.unfold',
		'editor.formatSelection': 'editor.action.formatSelection',
		'editor.goToSymbol': 'editor.action.quickOutline',
		'editor.insertCursorAtEndOfEachLine': 'editor.action.insertCursorAtEndOfEachLineSelected',
		'editor.addCursorAbove': 'editor.action.insertCursorAbove',
		'editor.addCursorBelow': 'editor.action.insertCursorBelow',
		'editor.deleteWordLeft': 'deleteWordLeft',
		'editor.deleteAllLeft': 'deleteAllLeft',
		'editor.deleteWordRight': 'deleteWordRight',
		'editor.deleteAllRight': 'deleteAllRight',
		'editor.goToTop': 'cursorTop',
		'editor.goToBottom': 'cursorBottom',
		'editor.goToLineStart': 'cursorHome',
		'editor.goToLineEnd': 'cursorEnd',
		'editor.selectToTop': 'cursorTopSelect',
		'editor.selectToBottom': 'cursorBottomSelect',
		'editor.selectToLineStart': 'cursorHomeSelect',
		'editor.selectToLineEnd': 'cursorEndSelect',
		'editor.cursorWordLeft': 'cursorWordStartLeft',
		'editor.cursorWordRight': 'cursorWordEndRight',
		'editor.cursorWordLeftSelect': 'cursorWordStartLeftSelect',
		'editor.cursorWordRightSelect': 'cursorWordEndRightSelect',
		'editor.blockComment': 'editor.action.blockComment',
		'editor.scrollLineUp': 'scrollLineUp',
		'editor.scrollLineDown': 'scrollLineDown',
		'editor.triggerParameterHints': 'editor.action.triggerParameterHints',
		'editor.foldAll': 'editor.foldAll',
		'editor.unfoldAll': 'editor.unfoldAll',
		'editor.toggleFold': 'editor.toggleFold',
		'editor.duplicateSelection': 'editor.action.duplicateSelection',
		'editor.jumpToBracket': 'editor.action.jumpToBracket',
		'editor.quickFix': 'editor.action.quickFix'
	};

	type Props = {
		tab: Tab;
		content?: string;
		language?: string;
		onContentChange?: (content: string, panelIndex?: number) => void;

		errorPosition?: number | null;
		errorMessage?: string | null;

		standalone?: boolean;
		onStateChange?: (viewState: unknown) => void;
	};

	let {
		tab,
		content = '',
		language = 'plaintext',
		onContentChange,
		errorPosition = null,
		errorMessage = null,
		standalone = false,
		onStateChange
	}: Props = $props();

	const effectiveDbId = $derived(getEffectiveSelectedDbId(tab.file?.node, tab));

	let container: HTMLElement;
	let editor = $state<monaco.editor.IStandaloneCodeEditor | null>(null);
	let diffEditor = $state<monaco.editor.IStandaloneDiffEditor | null>(null);
	let currentTabId: string;
	let currentLanguage: string = language;
	let changeListenerDisposable: monaco.IDisposable | null = null;
	let diffChangeDisposable: monaco.IDisposable | null = null;
	let editorDirty = false;
	let cursorDisposable: monaco.IDisposable | null = null;
	let selectionDisposable: monaco.IDisposable | null = null;
	let scrollDisposable: monaco.IDisposable | null = null;
	let editorTextFocused = $state(false);

	const persistViewState = debounce(() => {
		if (editor && onStateChange) onStateChange(editor.saveViewState() ?? undefined);
	}, 300);

	const isDiffMode = $derived(!!tab.diff);

	export function format() {
		if (currentLanguage !== 'sql-custom') return;
		formatSQL(editor, 'postgresql', (c) => onContentChange?.(c));
	}

	export function focus() {
		getActiveEditor()?.focus();
	}

	const fileUri = $derived(tab.file?.node.uri ?? '');
	const diffPath = $derived(
		tab.diff?.context === 'git' ? ((tab.diff?.meta?.path as string | undefined) ?? '') : ''
	);
	const displayPath = $derived(fileUri || diffPath);

	const getFile = () => tab.file?.node || null;

	function getActiveEditor(): monaco.editor.ICodeEditor | null {
		return editor ?? diffEditor?.getModifiedEditor?.() ?? null;
	}

	const QUERY_ERROR_SOURCE = 'query-error';

	$effect(() => {
		const ed = getActiveEditor();
		const model = ed?.getModel();
		const pos = errorPosition ?? null;
		const msg = errorMessage ?? null;
		if (!model) return;
		if (pos == null || !msg) {
			monaco.editor.setModelMarkers(model, QUERY_ERROR_SOURCE, []);
			return;
		}
		const offset = Math.max(0, pos - 1);
		const fullLen = model.getValueLength();
		if (offset >= fullLen) {
			monaco.editor.setModelMarkers(model, QUERY_ERROR_SOURCE, []);
			return;
		}
		const start = model.getPositionAt(offset);
		const end = model.getPositionAt(Math.min(offset + 1, fullLen));
		monaco.editor.setModelMarkers(model, QUERY_ERROR_SOURCE, [
			{
				startLineNumber: start.lineNumber,
				startColumn: start.column,
				endLineNumber: end.lineNumber,
				endColumn: end.column,
				message: msg,
				severity: monaco.MarkerSeverity.Error
			}
		]);
	});

	function findOldTab(): Tab | undefined {
		if (!currentTabId) return undefined;
		return getAllGroups()
			.flatMap((g) => g.tabs)
			.find((t) => t.id === currentTabId);
	}

	const handleChange = debounce(() => {
		const ed = getActiveEditor();
		if (!ed) return;

		const model = ed.getModel();
		if (!model) return;

		editorDirty = false;

		if (!onContentChange) return;
		onContentChange(model.getValue(), isDiffMode ? DIFF_PANEL_MODIFIED : undefined);
	}, 200);

	async function buildModelUri(uri: string, tabId: string): Promise<monaco.Uri> {
		if (uri.startsWith('selectdb://')) {
			const [absPath, err] = await tryCatch(GetOSPathFromURI, uri);
			if (!err && absPath) {
				return monaco.Uri.file(absPath);
			}
		}
		return monaco.Uri.parse(`inmemory://${tabId}`);
	}

	function scrollToLine(data: { uri: string; line: number; column?: number }) {
		const file = tab.file?.node;
		const ed = getActiveEditor();
		if (!ed || !file || file.uri !== data.uri) return;

		const position: monaco.IPosition = { lineNumber: data.line, column: data.column ?? 1 };
		ed.setSelection(
			new monaco.Selection(
				position.lineNumber,
				position.column,
				position.lineNumber,
				position.column
			)
		);
		ed.revealPositionInCenter(position);
		ed.focus();
	}

	async function mountContent(newContent: string, newLanguage: string, tabId: string) {
		const file = tab.file?.node;
		const uri = file?.uri ?? '';
		const isLanguageChange = newLanguage !== currentLanguage;
		const modelUri = await buildModelUri(uri, tabId);
		const existingModel = monaco.editor.getModel(modelUri);

		if (currentTabId === tabId && existingModel && !isLanguageChange) {
			if (existingModel.getValue() !== newContent) {
				const state = editor?.saveViewState?.();
				existingModel.setValue(newContent);
				if (state && editor) editor.restoreViewState(state);
			}
			return;
		}

		if (currentTabId && editor && currentTabId !== tabId) {
			const viewState = editor.saveViewState();
			const oldTab = findOldTab();
			if (oldTab?.file) {
				updateTab({
					...oldTab,
					file: {
						...oldTab.file,
						editor: {
							viewState: viewState ?? undefined,
							lintMarkers: oldTab.file.editor?.lintMarkers
						}
					}
				});
			}
		}

		currentTabId = tabId;
		currentLanguage = newLanguage;

		const model = existingModel || monaco.editor.createModel(newContent, newLanguage, modelUri);
		if (existingModel && existingModel.getValue() !== newContent) {
			existingModel.setValue(newContent);
		}

		changeListenerDisposable?.dispose();
		changeListenerDisposable = model.onDidChangeContent(() => {
			editorDirty = true;
			handleChange();
		});
		editor?.setModel(model);
		requestAnimationFrame(() => {
			const ed = editor;

			if (tab.file?.isTemp) ed?.focus();

			const savedState = tab.file?.editor?.viewState;
			if (savedState && ed) {
				ed.restoreViewState(savedState as monaco.editor.ICodeEditorViewState);
			}

			const savedMarkers = tab.file?.editor?.lintMarkers;
			if (savedMarkers && ed) {
				const m = ed.getModel();
				if (!m) return;
				monaco.editor.setModelMarkers(m, LINT_SOURCE, savedMarkers as monaco.editor.IMarkerData[]);
			}

			if (ed) syncCurrentFileFromEditor(ed, tab);
		});
	}

	function mountDiffContent() {
		if (!tab.diff || tab.diff.panels.length < 2) return;

		const d = tab.diff;
		const tabId = tab.id;
		const baseUri = `inmemory://${d.tabId}`;
		const originalUri = monaco.Uri.parse(`${baseUri}/original`);
		const modifiedUri = monaco.Uri.parse(`${baseUri}/modified`);

		if (currentTabId && diffEditor && currentTabId !== tabId) {
			const oldTab = findOldTab();
			if (oldTab?.diff && oldTab.diff.panels.length >= 2) {
				const origState = diffEditor.getOriginalEditor().saveViewState();
				const modState = diffEditor.getModifiedEditor().saveViewState();
				updateTab({
					...oldTab,
					diff: {
						...oldTab.diff,
						panels: [
							{ ...oldTab.diff.panels[0], editor: { viewState: origState ?? undefined } },
							{ ...oldTab.diff.panels[1], editor: { viewState: modState ?? undefined } }
						]
					}
				});
			}
		}

		currentTabId = tabId;
		currentLanguage = d.language;

		const originalModel =
			monaco.editor.getModel(originalUri) ??
			monaco.editor.createModel(d.panels[0].content, d.language, originalUri);
		const modifiedModel =
			monaco.editor.getModel(modifiedUri) ??
			monaco.editor.createModel(d.panels[1].content, d.language, modifiedUri);

		if (originalModel.getValue() !== d.panels[0].content)
			originalModel.setValue(d.panels[0].content);
		if (modifiedModel.getValue() !== d.panels[1].content)
			modifiedModel.setValue(d.panels[1].content);

		diffChangeDisposable?.dispose();
		diffChangeDisposable = modifiedModel.onDidChangeContent(() => handleChange());

		diffEditor?.setModel({ original: originalModel, modified: modifiedModel });

		requestAnimationFrame(() => {
			diffEditor?.updateOptions({
				renderSideBySide: d.viewMode !== 'inline'
			});
			const origEd = diffEditor!.getOriginalEditor();
			const modEd = diffEditor!.getModifiedEditor();
			origEd.updateOptions({ ...editorConfig, readOnly: true });
			modEd.updateOptions({ ...editorConfig, readOnly: d.readOnly ?? true });

			const savedOriginal = d.panels[0]?.editor?.viewState;
			const savedModified = d.panels[1]?.editor?.viewState;
			if (savedOriginal)
				origEd.restoreViewState(savedOriginal as monaco.editor.ICodeEditorViewState);
			if (savedModified)
				modEd.restoreViewState(savedModified as monaco.editor.ICodeEditorViewState);
			modEd.focus();
		});
	}

	let scrollToLineUnsubscribe: (() => void) | undefined;

	onMount(async () => {
		initializeLanguages();
		refreshTheme();

		if (isDiffMode) {
			diffEditor = monaco.editor.createDiffEditor(container, {
				...editorConfig,
				renderSideBySide: tab.diff?.viewMode !== 'inline',
				readOnly: tab.diff?.readOnly ?? true,
				renderMarginRevertIcon: !(tab.diff?.readOnly ?? true)
			});
			setActiveFileGetter(() => null);
			mountDiffContent();
		} else {
			const ed = monaco.editor.create(container, {
				...editorConfig,
				useShadowDOM: false,
				acceptSuggestionOnEnter: 'smart',
				tabCompletion: 'on',
				wordBasedSuggestions: 'off'
			});
			editor = ed;
			setActiveFileGetter(getFile);

			ed.onDidFocusEditorText(() => {
				setActiveFileGetter(getFile);
				setContext('editorFocus', true);
				editorTextFocused = true;
			});
			ed.onDidBlurEditorText(() => {
				setContext('editorFocus', false);
				editorTextFocused = false;
			});
			syncCurrentFileFromEditor(ed, tab);
			cursorDisposable = ed.onDidChangeCursorPosition(() => {
				syncCurrentFileFromEditor(ed, tab);
				persistViewState();
			});
			selectionDisposable = ed.onDidChangeCursorSelection(() => {
				syncCurrentFileFromEditor(ed, tab);
				persistViewState();
			});
			scrollDisposable = ed.onDidScrollChange(() => persistViewState());

			scrollToLineUnsubscribe = EventsOn('editor:scrollToLine', scrollToLine);
			await mountContent(content, language, tab.id);
		}
	});

	const isFocused = $derived($activeGroupStore?.activeTabId === tab.id);
	// Detached editors (Settings theme/config) are never the active tab of a
	// layout group, so fall back to live editor focus to own keybinding commands.
	const commandsActive = $derived(standalone ? editorTextFocused : isFocused);
	$effect(() => {
		const ed = editor;
		if (!ed || isDiffMode || !commandsActive) return;

		const handlers: Array<[string, () => void]> = [];
		for (const [command, action] of Object.entries(editorCommandMap)) {
			const handler = () => ed.trigger('keyboard', action, null);
			handlers.push([command, handler]);
			registerCommand(command, handler);
		}

		return () => {
			for (const [command, handler] of handlers) unregisterCommand(command, handler);
		};
	});

	$effect(() => {
		// Read tab.id early so it's always tracked.
		const tabId = tab.id;
		if (isDiffMode) {
			if (!diffEditor) return;
			if (!tab.diff) return;
			mountDiffContent();
		} else {
			if (!editor) return;
			if (editorDirty && tabId === currentTabId) return;
			editorDirty = false;
			mountContent(content, language, tabId);
		}
	});

	$effect(() => {
		const ed = editor;
		if (isDiffMode || !ed || !effectiveDbId || language !== 'sql-custom') {
			return;
		}
		const d = attachSqlSchemaPointerNavigation(
			ed,
			() => effectiveDbId,
			() => tab.file?.node?.id
		);
		return () => d.dispose();
	});

	$effect(() => {
		const ed = editor;
		if (isDiffMode || !ed || language !== 'sql-custom') {
			return;
		}
		const d = attachSqlVariablePointerNavigation(ed, getFile);
		return () => d.dispose();
	});

	$effect(() => {
		if (isDiffMode || !editor || language !== 'sql-custom') return;
		const ed = editor;
		const dbId = effectiveDbId;
		const d = attachSqlFunctionHighlighter(ed, () => dbId);
		return () => d.dispose();
	});

	$effect(() => {
		if (isDiffMode || !editor || !effectiveDbId || language !== 'sql-custom') return;
		const d = attachSqlLintProvider(editor, getFile, (markers) => {
			const currentFile = tab.file;
			if (!currentFile) return;
			updateTab({
				...tab,
				file: { ...currentFile, editor: { ...currentFile.editor, lintMarkers: markers } }
			});
		});
		return () => d.dispose();
	});

	onDestroy(() => {
		if (editor && onStateChange) onStateChange(editor.saveViewState() ?? undefined);
		cursorDisposable?.dispose();
		selectionDisposable?.dispose();
		scrollDisposable?.dispose();
		changeListenerDisposable?.dispose();
		diffChangeDisposable?.dispose();
		scrollToLineUnsubscribe?.();
		clearActiveFileGetter(getFile);
		setContext('editorFocus', false);

		editor?.dispose();
		diffEditor?.dispose();
		editor = null;
		diffEditor = null;
	});
</script>

<div class="editorRoot">
	{#if displayPath}
		<div class="uri-wrapper">
			{#if !displayPath.startsWith('temp:')}
				<Path uri={displayPath} />
			{/if}
		</div>
	{/if}
	<div class="editorContainer">
		<div class="editorScale" bind:this={container}></div>
	</div>
</div>

<style>
	:global(.editorContainer .monaco-editor .sql-schema-cmd-link) {
		text-decoration: underline;
		font-weight: 300 !important;
		text-underline-offset: 4px;
	}

	:global(.editorContainer .monaco-editor .sql-db-function) {
		color: var(--blue);
	}

	:global(.editorContainer .monaco-editor .sticky-widget) {
		border-bottom: var(--border);
	}

	:global(.editorContainer .monaco-editor) {
		--vscode-editorStickyScroll-background: var(--gray-200);
		--vscode-editor-background: var(--gray-200);
		--vscode-input-background: var(--gray-100);
	}

	:global(.editorContainer .monaco-editor .margin) {
		background-color: var(--gray-200);
	}

	:global(.editorContainer .monaco-diff-editor) {
		--vscode-editorStickyScroll-background: var(--gray-200);
		--vscode-editor-background: var(--gray-200);
	}

	:global(.editorContainer .monaco-editor) {
		--vscode-focusBorder: transparent;
	}

	:global(.editorContainer .monaco-editor .suggest-widget) {
		box-shadow: 0 6px 8px 0 var(--shadow) !important;
		border: var(--border) !important;
		z-index: 10;
	}

	/* Hover tooltip (incl. SQL schema markdown): shadow + themed markdown tables */
	:global(.monaco-hover) {
		box-shadow: 0 var(--space-xs) var(--space-md) var(--shadow) !important;
		border: var(--border) !important;
		border-radius: var(--space-xs);
		background: var(--gray-100);
	}

	:global(.monaco-hover *) {
		font-size: var(--fs-sm);
	}

	:global(.monaco-hover table) {
		width: 100%;
		border-collapse: collapse;
		line-height: 1.4;
		overflow: hidden;
		border-radius: var(--space-sm);
	}

	:global(.monaco-hover table th) {
		text-align: left;
		font-weight: var(--fw-normal);
		padding: var(--space-sm) var(--space-sm-md);
		text-transform: uppercase;
		color: var(--gray-900);
		background: var(--gray-200);
	}

	:global(.monaco-hover table td) {
		padding: var(--space-sm) var(--space-sm-md);
		vertical-align: top;
		word-break: break-word;
		color: var(--gray-1000);
		background: var(--gray-200);
	}

	:global(.monaco-hover table tbody tr:nth-child(even) td) {
		background: var(--gray-100);
	}

	:global(.monaco-hover table tbody tr:last-child td) {
		border-bottom: none;
	}

	:global(.monaco-hover table td:first-child) {
		font-family: ui-monospace, monospace;
	}

	:global(.monaco-hover code) {
		padding: var(--space-xxs) var(--space-xs);
	}

	:global(.gutter.monaco-editor) {
		outline: none;
	}
	:global(.gutter.monaco-editor::after) {
		content: '';
		position: absolute;
		top: 0;
		right: 0;
		width: 1px;
		height: 100%;
		background: var(--border-color);
	}

	.editorRoot {
		height: 100%;
		display: flex;
		flex-direction: column;
		width: 100%;
	}

	.uri-wrapper {
		padding: var(--space-sm) var(--space-sm-md);
		background-color: var(--gray-200);
	}

	.editorContainer {
		flex: 1;
		width: 100%;
		overflow: hidden;
	}

	.editorScale {
		width: 100%;
		height: 100%;
	}
</style>
