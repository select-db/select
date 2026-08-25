<script lang="ts">
	import { LookupForeignKey } from '$lib/wails/graph';
	import * as db_client from '$lib/bindings/selectDb/internal/db_client/models';
	import Checkbox from '$lib/system/Checkbox/Checkbox.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import Menu from '$lib/system/Menu/Menu.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';
	import type { MenuOption } from '$lib/system/Menu/Menu.types';
	import { notifyError } from '$lib/system/Notifications/notificationsStore';
	import { debounce } from '$lib/utils/debounce';

	type Props = {
		databaseId: string;
		sourceColumn: string;
		currentValue: string;
		targetSchema: string;
		targetTable: string;
		targetColumn: string;
		availableColumns: string[];
		selectedColumns: string[];
		onSelectColumns: (cols: string[]) => void;
		onSave: (value: string) => void;
		onClose: () => void;
	};

	let {
		databaseId,
		currentValue,
		targetSchema,
		targetTable,
		targetColumn,
		availableColumns,
		selectedColumns,
		onSelectColumns,
		onSave,
		onClose
	}: Props = $props();

	const ROW_LIMIT = 200;
	const NULL_VALUE = 'NULL';
	const isNull = (v: string) => v === '' || v === NULL_VALUE;

	let selected = $state<string[]>([...selectedColumns]);
	let searchQuery = $state('');
	let columnFilter = $state('');
	let rows = $state<unknown[][]>([]);
	let cols = $state<string[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);

	const isCheckedCol = (name: string) => selected.includes(name);
	function toggleCol(name: string) {
		const next = isCheckedCol(name) ? selected.filter((c) => c !== name) : [...selected, name];
		selected = next;
		onSelectColumns(next);
	}

	// Display order always follows the left panel (availableColumns), so toggling
	// columns doesn't shuffle the label parts of menu options.
	const orderedSelected = $derived(availableColumns.filter((c) => selected.includes(c)));

	const filteredColumns = $derived.by(() => {
		const q = columnFilter.trim().toLowerCase();
		if (!q) return availableColumns;
		return availableColumns.filter((c) => c.toLowerCase().includes(q));
	});

	async function fetchRows() {
		if (orderedSelected.length === 0) {
			rows = [];
			cols = [];
			error = 'Select at least one column to display.';
			return;
		}
		loading = true;
		error = null;
		try {
			const params = new db_client.LookupForeignKeyParams({
				databaseId,
				schema: targetSchema,
				table: targetTable,
				fkColumn: targetColumn,
				displayColumns: orderedSelected,
				query: searchQuery,
				currentValue: isNull(currentValue) ? '' : currentValue,
				limit: ROW_LIMIT
			});
			const result = await LookupForeignKey(params);
			if (result.errors && result.errors.length > 0) {
				error = result.errors.join('\n');
				rows = [];
				cols = [];
			} else {
				rows = result.rows ?? [];
				cols = result.columns ?? [];
			}
		} catch (e) {
			const msg = e instanceof Error ? e.message : String(e);
			error = msg;
			notifyError(`Failed to load referenced rows: ${msg}`);
		} finally {
			loading = false;
		}
	}

	const debouncedFetch = debounce(fetchRows, 200);

	// First load: run immediately so the menu populates without the debounce
	// delay. Subsequent changes (search input, column toggles) go through the
	// debounce.
	let firstLoad = true;
	$effect(() => {
		void searchQuery;
		void orderedSelected;
		if (firstLoad) {
			firstLoad = false;
			fetchRows();
			return;
		}
		debouncedFetch();
	});

	function formatCell(v: unknown): string {
		if (v === null || v === undefined) return '';
		if (typeof v === 'string') return v;
		return String(v);
	}

	// Sort tier for an option given the current search query. Lower = higher.
	// 0: pinned current value, 1: exact match, 2: prefix match, 3: substring,
	// 4: no match (kept because of the OR on the FK column).
	function relevance(label: string, q: string): number {
		if (!q) return 3;
		const a = label.toLowerCase();
		const b = q.toLowerCase();
		if (a === b) return 1;
		if (a.startsWith(b)) return 2;
		if (a.includes(b)) return 3;
		return 4;
	}

	const menuOptions = $derived.by<MenuOption[]>(() => {
		const fkIdx = cols.indexOf(targetColumn);
		if (fkIdx < 0) return [];
		const displayIdxs = orderedSelected.map((c) => cols.indexOf(c)).filter((i) => i >= 0);

		const q = searchQuery.trim();
		const opts: MenuOption[] = rows.map((row, i) => {
			const fkVal = formatCell(row[fkIdx]);
			const parts = displayIdxs.map((idx) => formatCell(row[idx]));
			const label = parts.filter((p) => p !== '').join(' · ') || fkVal || '(empty)';
			const isCurrent = !isNull(currentValue) && fkVal === currentValue;
			return {
				id: `${fkVal}__${i}`,
				label,
				selected: isCurrent,
				customData: { label, tier: isCurrent ? 0 : relevance(label, q), originalIdx: i },
				action: () => {
					onSave(fkVal);
					onClose();
				}
			};
		});

		opts.sort((a, b) => {
			const at = (a.customData as { tier: number; originalIdx: number }).tier;
			const bt = (b.customData as { tier: number; originalIdx: number }).tier;
			if (at !== bt) return at - bt;
			return (
				(a.customData as { originalIdx: number }).originalIdx -
				(b.customData as { originalIdx: number }).originalIdx
			);
		});
		return opts;
	});

	function setNull() {
		onSave(NULL_VALUE);
		onClose();
	}

	async function handleSetNull() {
		setNull();
	}

	async function handleCancel() {
		onClose();
	}
</script>

<div class="fk-modal-body">
	<div class="left-pane">
		<div class="pane-header">
			<p class="truncate">{targetSchema}.{targetTable}</p>
		</div>
		<div class="column-filter">
			<Input
				bind:value={columnFilter}
				placeholder="Search columns…"
				size="lg"
				noRadius
				noBorder
				style="flex-grow: 1;"
			/>
		</div>
		<div class="column-list scrollable">
			{#each filteredColumns as col (col)}
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div
					class="column-row"
					class:active={isCheckedCol(col)}
					class:fk-target={col === targetColumn}
					onclick={() => toggleCol(col)}
				>
					<Checkbox
						checked={isCheckedCol(col)}
						onchange={() => toggleCol(col)}
						size="sm"
						label={col}
					/>
				</div>
			{/each}
		</div>
	</div>
	<div class="right-pane">
		{#if error}
			<div class="error">
				<p>{error}</p>
			</div>
		{:else if loading && rows.length === 0}
			<div class="state">
				<Loader />
			</div>
		{:else}
			<Menu
				options={menuOptions}
				searchEnabled
				searchPlaceholder={`Search ${orderedSelected.join(', ') || targetColumn}…`}
				bind:searchQuery
				externalFilter
				noBorder
				width="100%"
				emptyMessage="No rows in target table"
				noResultsMessage={loading ? 'Loading…' : 'No matches'}
				optionContent={optionLabel}
			/>
		{/if}
	</div>
</div>

{#snippet optionLabel(data: unknown)}
	{@const d = data as { label: string }}
	<p class="option-label truncate">{d.label}</p>
{/snippet}

<ModalFooter
	leftAction={{ label: 'Set NULL', action: handleSetNull }}
	secondaryAction={{ label: 'Cancel', action: handleCancel }}
/>

<style>
	.fk-modal-body {
		display: flex;
		flex: 1;
		min-height: 0;
	}

	.left-pane {
		display: flex;
		flex-direction: column;
		width: 240px;
		min-width: 240px;
		border-right: var(--border);
		background-color: var(--gray-100);
	}

	.pane-header {
		padding: var(--space-sm-md);
		border-bottom: var(--border);
	}

	.column-filter {
		display: flex;
		border-bottom: var(--border);
	}

	.column-list {
		flex: 1;
		min-height: 0;
		overflow-y: auto;
		padding: var(--space-xs) 0;
	}

	.column-row {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		padding: var(--space-sm) var(--space-sm);
		min-width: 0;
	}

	.column-row :global(.checkbox-label) {
		font-family: 'JetBrains Mono', monospace;
		font-size: var(--fs-sm);
		font-weight: 200;
	}

	.column-row.fk-target :global(.checkbox-label) {
		color: var(--blue);
	}

	.right-pane {
		background-color: var(--gray-200);
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
	}

	/* The Menu component keeps its own box-shadow even with noBorder; flatten it
	   when embedded in the modal so the panel boundary is the only separator. */
	.right-pane :global(.menu-wrapper) {
		box-shadow: none;
		min-width: 0;
	}

	.option-label {
		margin: 0;
		font-size: var(--fs-sm);
		padding: var(--space-xs-sm);
		min-width: 0;
		width: 100%;
	}

	.state,
	.error {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: var(--space-lg);
	}

	.error p {
		margin: 0;
		color: var(--red, var(--gray-800));
		font-size: var(--fs-sm);
		font-family: 'JetBrains Mono', monospace;
		white-space: pre-wrap;
		max-width: 100%;
	}
</style>
