<script lang="ts">
	import { untrack } from 'svelte';
	import type { Snippet } from 'svelte';

	import * as fs from '$lib/wailsjs/go/fs_provider/FSProvider';
	import { updateTab, removeTab, type Tab } from '$lib/components/Layout/layoutStore';
	import { notifyError } from '$lib/system/Notifications/notificationsStore';
	import { tryCatch, must } from '$lib/utils/tryCatch';
	import { debounce } from '$lib/utils/debounce';

	import Editor from '../Editor/Editor.svelte';

	type Props = {
		tab: Tab;
		language: string;
		manualSave?: boolean;
		defaultContent?: string;

		/** Forwarded to Editor for synthetic tabs detached from any layout group. */
		standalone?: boolean;
		/** Forwarded to Editor; fires with Monaco view state on cursor/scroll change. */
		onStateChange?: (viewState: unknown) => void;
		header?: Snippet<
			[
				{
					hasUnsavedChanges: boolean;
					isModifiedFromDefault: boolean;
					saveFile: () => Promise<void>;
				}
			]
		>;
	};

	let {
		tab,
		language,
		manualSave = false,
		defaultContent,
		standalone = false,
		onStateChange,
		header
	}: Props = $props();

	const file = $derived(tab.file?.node);
	const isTemp = $derived(tab.file?.isTemp ?? false);
	let content = $state<string>('');
	let contentLoaded = $state(false);
	let hasUnsavedChanges = $state(false);
	const isModifiedFromDefault = $derived(
		contentLoaded && defaultContent !== undefined && content !== defaultContent
	);

	let currentLoadingTabId: string | undefined;
	$effect(() => {
		const tabId = tab.id;
		const currentFile = tab.file;

		const alreadyHandled = untrack(() => currentLoadingTabId === tabId);
		if (alreadyHandled) return;

		currentLoadingTabId = tabId;

		if (!currentFile?.node) {
			content = '';
			contentLoaded = false;
			return;
		}

		if (currentFile.isTemp) {
			content = currentFile.content ?? '';
			contentLoaded = true;
			return;
		}

		content = '';
		contentLoaded = false;
		hasUnsavedChanges = false;
		readFileContent(currentFile.node.uri);
	});

	async function readFileContent(uri: string) {
		const [fileContent, err] = await tryCatch(fs.ReadFile, { uri });
		if (err) {
			notifyError(`Failed to read file: ${uri}`);
			removeTab(tab.id);
			return;
		}

		content = fileContent ?? '';
		contentLoaded = true;
	}

	const writeToFile = debounce(async (uri: string, fileContent: string) => {
		await must(tryCatch(fs.Write, { uri, content: fileContent }));
	}, 200);

	const handleContentChange = (newContent: string) => {
		content = newContent;

		if (isTemp) {
			updateTab({
				...tab,
				file: {
					...tab.file!,
					content
				}
			});
			return;
		}

		if (file) {
			if (manualSave) {
				hasUnsavedChanges = true;
				return;
			}
			writeToFile(file.uri, content);
		}
	};

	const saveFile = async () => {
		if (!file) return;
		await must(tryCatch(fs.Write, { uri: file.uri, content }));
		hasUnsavedChanges = false;
	};
</script>

<div class="file-view">
	{#if header}
		{@render header({ hasUnsavedChanges, isModifiedFromDefault, saveFile })}
	{/if}

	<div class="page">
		{#if contentLoaded}
			<Editor
				{tab}
				{content}
				{language}
				{standalone}
				{onStateChange}
				onContentChange={handleContentChange}
			/>
		{/if}
	</div>
</div>

<style>
	.file-view {
		height: 100%;
		overflow: hidden;
		display: flex;
		flex-direction: column;
	}

	.page {
		display: flex;
		overflow: hidden;
		flex: 1 1 0;
		min-height: 0;
	}

	.page :global(Editor) {
		flex: 1 1 0;
		min-width: 0;
		min-height: 0;
	}
</style>
