<script lang="ts">
	import { onMount } from 'svelte';
	import { get } from 'svelte/store';
	import Button from '$lib/system/Button/Button.svelte';
	import InputMultiline from '$lib/system/InputMultiline/InputMultiline.svelte';
	import Select from '$lib/system/Select/Select.svelte';
	import DatabasePicker from '$lib/components/views/File/Header/DatabasePicker.svelte';
	import FileBadge from './FileBadge.svelte';
	import FilePicker from './FilePicker.svelte';
	import { AI_MODEL_OPTION_GROUPS } from '$lib/components/views/Chat/core/aiConnections';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import {
		updateTab,
		dbInstanceToContextDb,
		getAllGroups
	} from '$lib/components/Layout/layoutStore';
	import { findItemById } from '$lib/components/views/FileSystem/Files/helpers/dragHelpers';
	import type { Tab, ChatContextFile } from '$lib/components/Layout/layoutStore';
	import type { ResourceMenuOption } from '$lib/components/ResourceMenu/types';
	import type * as graph from '$lib/wails/graph';

	type Props = {
		inputText: string;
		selectedModel: string;
		sending: boolean;
		tab: Tab;
		onSend: () => void;
		onStop: () => void;
	};

	let {
		inputText = $bindable(),
		selectedModel = $bindable(),
		sending,
		tab,
		onSend,
		onStop
	}: Props = $props();

	let inputFocused = $state(false);
	let inputActive = $state(false);
	let inputRef: { focus: () => void } | undefined;

	const contextFiles = $derived((tab.chat?.files ?? []));
	const selectedFileIds = $derived(contextFiles.map((f) => f.id));

	function focusInputOnFooterClick(e: MouseEvent) {
		const t = e.target as HTMLElement;
		if (t.closest('button') || t.closest('[data-select-trigger]')) return;
		inputRef?.focus();
	}

	function toggleContextFile(option: ResourceMenuOption) {
		const existingFiles = (tab.chat?.files ?? []);
		const existingIndex = existingFiles.findIndex((f) => f.id === option.id);

		let files: ChatContextFile[];
		if (existingIndex >= 0) {
			files = existingFiles.filter((f) => f.id !== option.id);
		} else {
			files = [...existingFiles, { id: option.id, uri: option.uri, folderId: option.folderId }];
		}

		updateTab({
			...tab,
			chat: {
				sessionId: tab.chat!.sessionId,
				...tab.chat,
				files
			}
		});
	}

	function getFileNode(file: ChatContextFile): graph.FileNode | null {
		const ws = get(workspaceGraphStore);
		if (ws) {
			const found = findItemById(file.id, [], ws.folders, ws.db_instances);
			if (found && found.type === 'file') return found as graph.FileNode;
		}
		// Temp files are not in the workspace graph, find the node from the open tab.
		// Match Tab.id (current chat context) or virtual node id / uri (older persisted sessions).
		for (const group of getAllGroups()) {
			for (const t of group.tabs) {
				const n = t.file?.node;
				if (t.file?.isTemp && n && (t.id === file.id || n.id === file.id || t.uri === file.uri)) {
					return n as graph.FileNode;
				}
			}
		}
		return null;
	}

	function handleDatabaseChange(v: string | string[]) {
		const ids = Array.isArray(v) ? v : v ? [v] : [];
		const ws = get(workspaceGraphStore);
		const dbInstances = (ws?.db_instances ?? []);
		const databases = ids
			.map((id) => dbInstances.find((d) => d.id === id))
			.filter(Boolean)
			.map((d) => dbInstanceToContextDb(d!));

		updateTab({
			...tab,
			chat: {
				sessionId: tab.chat!.sessionId,
				...tab.chat,
				databases
			}
		});
	}

	onMount(() => setTimeout(() => inputRef?.focus(), 100));
</script>

<div
	class="chat-footer"
	class:focused={inputFocused}
	class:active={inputActive}
	role="button"
	tabindex="0"
	aria-label="Focus chat input"
	onclick={focusInputOnFooterClick}
	onkeydown={(e) => {
		if (document.activeElement !== e.currentTarget) return;
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			inputRef?.focus();
		}
	}}
>
	<div class="input-row">
		<InputMultiline
			bind:this={inputRef}
			size="lg"
			bind:value={inputText}
			bind:focused={inputFocused}
			bind:active={inputActive}
			placeholder="Type a message..."
			onsubmit={onSend}
			noBorder
			autofocus
		/>
	</div>

	{#if contextFiles.length > 0}
		<div class="file-row">
			{#each contextFiles as file (file.id)}
				{@const node = getFileNode(file)}
				{#if node}
					<FileBadge
						{node}
						onRemove={() =>
							toggleContextFile({
								id: file.id,
								uri: file.uri,
								label: node.name,
								type: 'file',
								node,
								folderId: file.folderId
							})}
					/>
				{/if}
			{/each}
		</div>
	{/if}

	<div class="model-row">
		<DatabasePicker
			multiple={true}
			value={tab.chat?.databases?.map((d) => d.id) ?? []}
			onchange={handleDatabaseChange}
		/>
		<div class="left-wrapper">
			<Select
				optionGroups={AI_MODEL_OPTION_GROUPS}
				bind:value={selectedModel}
				placeholder="Model"
				emphasis="low"
				menuWidth={275}
				searchEnabled
				searchPlaceholder="Search models..."
			/>

			<div class="action-wrapper">
				<FilePicker selectedIds={selectedFileIds} onToggle={toggleContextFile} />

				{#if sending}
					<Button
						emphasis="warning"
						size="sm"
						leftIcon="cross"
						iconSize={20}
						onclick={onStop}
						active
					/>
				{:else}
					<Button emphasis="low" size="sm" leftIcon="arrow-up" iconSize={20} onclick={onSend} />
				{/if}
			</div>
		</div>
	</div>
</div>

<style>
	.chat-footer {
		cursor: text;
		background-color: var(--gray-0);
		transition: all 0.2s;
		min-width: 300px;
	}

	.chat-footer:hover,
	.chat-footer.focused,
	.chat-footer.active {
		background-color: var(--gray-100);
	}

	.input-row {
		display: flex;
		border-top: var(--border);
		padding: var(--space-xs) var(--space-xs) 0 var(--space-xs);
	}

	.input-row :global([contenteditable='true']) {
		flex: 1;
	}

	.model-row {
		display: flex;
		align-items: end;
		justify-content: space-between;
		gap: var(--space-sm);
		padding: 0 var(--space-sm) calc(var(--space-sm) + 2px) var(--space-sm);
		margin-top: -2px;
	}

	.model-row :global([data-select-trigger]) {
		min-width: 180px;
	}

	.model-row .left-wrapper {
		display: flex;
		align-items: center;
	}

	.file-row {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-xs);
		padding: var(--space-xs) var(--space-sm);
		margin-top: -8px;
	}

	.action-wrapper {
		display: flex;
		gap: var(--space-xs);
	}
</style>
