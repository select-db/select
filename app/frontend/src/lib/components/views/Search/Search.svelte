<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import { AlertType } from '$lib/system/Alert/types';

	import { notify, notifyError } from '$lib/system/Notifications/notificationsStore';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import { debounce } from '$lib/utils/debounce';

	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { searchResultsStore, performSearch, performReplace } from './searchStore';
	import { get } from 'svelte/store';

	import FileItems from '$lib/components/views/FileSystem/Files/FileItems.svelte';
	import { expandItem, expandedItemIdsStore } from '$lib/components/views/shared/sharedStore';
	import {
		buildVisibilityIndex,
		updateScrollWindow
	} from '$lib/components/views/FileSystem/Files/helpers/visibilityStore';
	import { throttle } from '$lib/utils/throttle';

	// Search parameters
	let searchPattern = $state('');
	let replacePattern = $state('');
	let useRegex = $state(false);
	let caseSensitive = $state(false);
	let wholeWord = $state(false);
	let includePattern = $state('');
	let excludePattern = $state('');

	// UI state
	let isSearching = $state(false);
	let showReplace = $state(false);
	let showFilters = $state(false);

	const searchResults = $derived($searchResultsStore);

	// Result folder comes directly from the backend now
	const searchResultsFolder = $derived(
		searchResults?.resultFolder ? [searchResults.resultFolder] : []
	);

	const search = async () => {
		const workspaceId = get(workspaceGraphStore)?.id;
		if (!workspaceId || !searchPattern) return;

		isSearching = true;
		try {
			await performSearch({
				workspaceId,
				pattern: searchPattern,
				useRegex,
				caseSensitive,
				wholeWord,
				includePattern,
				excludePattern
			});
		} catch (error: unknown) {
			const message = error instanceof Error ? error.message : String(error);
			notifyError(`Search failed: ${message}`);
		} finally {
			isSearching = false;
		}
	};

	const replace = async () => {
		const workspaceId = get(workspaceGraphStore)?.id;
		if (!workspaceId || !searchPattern) return;

		const result = await must(
			tryCatch(performReplace, {
				workspaceId,
				pattern: searchPattern,
				replacement: replacePattern,
				useRegex,
				caseSensitive,
				wholeWord,
				includePattern,
				excludePattern,
				filePath: '',
				dryRun: false
			})
		);

		notify({
			type: AlertType.Success,
			message: `Replaced ${result.totalReplacements} occurrences in ${result.filesModified} files`
		});

		// Re-run search to update results
		await search();
	};

	// Ensure search folder is expanded by default
	$effect(() => {
		if (searchResultsFolder.length > 0) {
			expandItem('search::results');
		}
	});

	// Debounced auto-search when inputs change
	const debouncedSearch = debounce(() => search(), 200);

	$effect(() => {
		// Track reactive dependencies
		void searchPattern;
		void includePattern;
		void excludePattern;
		void useRegex;
		void caseSensitive;
		void wholeWord;

		if (!searchPattern) return;
		debouncedSearch();
	});

	// Virtual scrolling
	let scrollContainer: HTMLDivElement = $state()!;

	// Rebuild visibility index when search results or expanded state changes
	$effect(() => {
		const folders = searchResultsFolder;
		const expandedIds = $expandedItemIdsStore;
		buildVisibilityIndex('search', folders, [], [], [], expandedIds);
		// Update scroll window immediately after building index
		if (scrollContainer) {
			updateScrollWindow('search', scrollContainer.scrollTop, scrollContainer.clientHeight);
		}
	});

	const handleScroll = throttle(() => {
		if (scrollContainer) {
			updateScrollWindow('search', scrollContainer.scrollTop, scrollContainer.clientHeight);
		}
	}, 16);
</script>

<div class="search-panel">
	<div class="section space x y">
		<div class="options">
			<Button
				emphasis={caseSensitive ? 'high' : 'low'}
				size="sm"
				active={caseSensitive}
				onclick={() => {
					caseSensitive = !caseSensitive;
				}}
				leftIcon="text"
				iconSize={18}
				label="Match Case"
			/>
			<Button
				emphasis={wholeWord ? 'high' : 'low'}
				size="sm"
				active={wholeWord}
				onclick={() => {
					wholeWord = !wholeWord;
				}}
				leftIcon="whole-word"
				iconSize={18}
				label="Match Whole Word"
			/>
			<Button
				emphasis={useRegex ? 'high' : 'low'}
				size="sm"
				active={useRegex}
				onclick={() => {
					useRegex = !useRegex;
				}}
				leftIcon="regex"
				iconSize={18}
				label="Use Regular Expression"
			/>
			<div class="options appart">
				<Button
					emphasis={showReplace ? 'high' : 'low'}
					size="sm"
					active={showReplace}
					onclick={() => {
						showReplace = !showReplace;
					}}
					leftIcon="replace"
					iconSize={18}
					label="Toggle Replace"
				/>
				<Button
					emphasis={showFilters ? 'high' : 'low'}
					size="sm"
					active={showFilters}
					onclick={() => {
						showFilters = !showFilters;
					}}
					leftIcon="files"
					iconSize={18}
					label="Toggle File Filters"
				/>
			</div>
		</div>
		<div class="field">
			<Input bind:value={searchPattern} placeholder="Search" autofocus />
			{#if showReplace}
				<div class="field">
					<Input bind:value={replacePattern} placeholder="Replace" />
				</div>
			{/if}
		</div>

		{#if showFilters}
			<div class="filters">
				<div class="divider"></div>
				<div class="field">
					<p class="muted">Files to include</p>
					<Input bind:value={includePattern} placeholder="e.g., *.ts" />
				</div>
				<div class="field">
					<p class="muted">Files to exclude</p>
					<Input bind:value={excludePattern} placeholder="e.g., *.test.ts" />
				</div>
			</div>
		{/if}

		{#if showReplace}
			<div class="actions">
				<Button
					content="Replace All"
					size="sm"
					onclick={replace}
					emphasis="low"
					disabled={!searchPattern || !searchResults || searchResults.totalMatches === 0}
					iconSize={14}
				/>
			</div>
		{/if}
	</div>

	{#if isSearching}
		<div class="section space x y">
			<Loader />
		</div>
	{:else if searchResults}
		<div
			class="section no-scrollbar overflow-x-only"
			style="border-top: var(--border); flex-grow: 1"
			bind:this={scrollContainer}
			onscroll={handleScroll}
		>
			<div>
				<FileItems
					files={[]}
					folders={searchResultsFolder}
					databases={[]}
					databaseItems={[]}
					depth={0}
					parentIds={[]}
					ctx="search"
				/>
			</div>
		</div>
	{/if}
</div>

<style>
	.search-panel {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
	}

	.overflow-x-only {
		overflow-x: hidden;
		overflow-y: auto;
		overscroll-behavior-y: none;
	}

	.section {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
	}

	.divider {
		margin-top: var(--space-xxs);
	}

	.space.x {
		padding-left: var(--space-sm-md);
		padding-right: var(--space-sm-md);
	}
	.space.y {
		padding-top: var(--space-sm);
		padding-bottom: var(--space-sm-md);
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
	}

	.muted {
		color: var(--gray-800);
	}

	.options {
		display: flex;
		gap: var(--space-xs);
		align-items: center;
	}

	.options.appart {
		margin-left: auto;
	}

	.filters {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
	}

	.actions {
		margin-top: var(--space-xs);
		display: flex;
		gap: var(--space-xs);
	}
</style>
