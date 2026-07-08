<script lang="ts">
	import { tick } from 'svelte';
	import type { Component } from 'svelte';
	import { notifyError } from '$lib/system/Notifications/notificationsStore';
	import Button from '$lib/system/Button/Button.svelte';
	import Select from '$lib/system/Select/Select.svelte';
	import type { SelectOption } from '$lib/system/Select/Select.types';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import CellValueModal from './CellValueModal.svelte';
	import ForeignKeyPickerModal from './ForeignKeyPickerModal.svelte';

	export type ForeignKeyContext = {
		databaseId: string;
		targetSchema: string;
		targetTable: string;
		targetColumn: string;
		availableColumns: string[];
		selectedColumns: string[];
		onSelectColumns: (cols: string[]) => void;
	};

	type Props = {
		value: string;
		hasAllPrimaryKeys: boolean;
		isPrimaryKey: boolean;
		missingPrimaryKeys: string[];
		isEdited: boolean;
		enumValues?: string[];
		dataType?: string;
		columnName?: string;
		foreignKey?: ForeignKeyContext;
		onEdit: (value: string) => void;
		onEndEdit: () => void;
		onMoveEdit: (direction: 'up' | 'down' | 'left' | 'right') => void;
		onRollback: () => void;
	};

	let {
		value,
		hasAllPrimaryKeys,
		isPrimaryKey,
		missingPrimaryKeys,
		isEdited,
		enumValues,
		dataType,
		columnName,
		foreignKey,
		onEdit,
		onEndEdit,
		onMoveEdit,
		onRollback
	}: Props = $props();

	const editable = $derived(hasAllPrimaryKeys && !isPrimaryKey);

	// NULL emits the literal "NULL" string, matching what typing NULL into the
	// text input does today, so the edit pipeline stays byte-for-byte unchanged.
	const NULL_VALUE = 'NULL';
	const useMenu = $derived(editable && !!enumValues && enumValues.length > 0);
	const menuOptions = $derived<SelectOption[]>([
		...(enumValues ?? []).map((v) => ({ value: v, label: v })),
		{ value: NULL_VALUE, label: '(NULL)' }
	]);

	let menuOpen = $state(true);
	let menuValue = $state('');
	let menuWidth = $state(0);

	$effect(() => {
		menuValue = formatValue(value);
	});

	// Closing the menu (pick, Escape, or click-away) ends the edit. A pick has
	// already pushed its value via onchange by the time open flips to false.
	let menuWasOpen = false;
	$effect(() => {
		if (menuOpen) {
			menuWasOpen = true;
			return;
		}
		if (menuWasOpen) {
			menuWasOpen = false;
			onEndEdit();
		}
	});

	function handleMenuChange(next: string | string[]) {
		const picked = Array.isArray(next) ? next[0] : next;
		if (picked === undefined) return;
		if (!checkEditable()) return;
		onEdit(picked);
	}

	let inputValue = $state('');
	let inputElement: HTMLInputElement | null = $state(null);
	let modalOpen = $state(false);

	let actionsWidth = $state(0);

	// Sync input value when prop changes (e.g. after rollback)
	$effect(() => {
		inputValue = formatValue(value);
	});

	// Focus and select on mount
	$effect(() => {
		tick().then(() => {
			inputElement?.focus();
			inputElement?.select();
		});
	});

	function formatValue(val: unknown): string {
		if (val === null) return 'NULL';
		if (val === undefined) return '';
		return typeof val === 'string' ? val : String(val);
	}

	function handleInput() {
		if (!checkEditable()) {
			inputValue = value;
			return;
		}
		onEdit(inputValue);
	}

	function handleBlur() {
		if (modalOpen) return;
		onEdit(inputValue);
		onEndEdit();
	}

	function openModal() {
		if (!checkEditable()) return;
		modalOpen = true;
		const onCancel = () => {
			modalStore.set(null);
			modalOpen = false;
			tick().then(() => {
				inputElement?.focus();
			});
		};

		if (foreignKey) {
			modalStore.set({
				content: () => ForeignKeyPickerModal as unknown as Component,
				width: 'min(80vw, 860px)',
				height: 'min(70vh, 620px)',
				props: {
					databaseId: foreignKey.databaseId,
					currentValue: inputValue,
					targetSchema: foreignKey.targetSchema,
					targetTable: foreignKey.targetTable,
					targetColumn: foreignKey.targetColumn,
					availableColumns: foreignKey.availableColumns,
					selectedColumns: foreignKey.selectedColumns,
					onSelectColumns: foreignKey.onSelectColumns,
					onSave: (v: string) => onEdit(v),
					onCancel
				}
			});
			return;
		}

		modalStore.set({
			content: () => CellValueModal as unknown as Component,
			width: 'min(80vw, 860px)',
			height: 'min(70vh, 620px)',
			props: {
				value: inputValue,
				dataType,
				columnName,
				onSave: (v: string) => onEdit(v),
				onCancel
			}
		});
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && e.shiftKey) {
			e.preventDefault();
			openModal();
			return;
		}

		if (e.key === 'Escape' || e.key === 'Enter') {
			e.preventDefault();
			handleBlur();
			return;
		}

		// Only navigate if no text is selected
		const hasSelection = inputElement && inputElement.selectionStart !== inputElement.selectionEnd;
		if (hasSelection) return;

		const cursorPos = inputElement?.selectionStart ?? 0;
		const textLen = inputValue.length;

		if (e.key === 'ArrowUp') {
			e.preventDefault();
			onMoveEdit('up');
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			onMoveEdit('down');
		} else if (e.key === 'ArrowLeft' && cursorPos === 0) {
			e.preventDefault();
			onMoveEdit('left');
		} else if (e.key === 'ArrowRight' && cursorPos === textLen) {
			e.preventDefault();
			onMoveEdit('right');
		}
	}

	function checkEditable(): boolean {
		if (editable) return true;

		if (isPrimaryKey) {
			notifyError(`Column is a primary key and cannot be edited.`);
		} else if (missingPrimaryKeys.length > 0) {
			notifyError(
				`Missing primary key${missingPrimaryKeys.length > 1 ? 's' : ''} in SELECT: ${missingPrimaryKeys.join(', ')}`
			);
		} else {
			notifyError(`Column cannot be edited: computed expression or no primary key.`);
		}
		return false;
	}
</script>

{#if useMenu}
	<div class="editable-menu-container" bind:clientWidth={menuWidth}>
		<Select
			options={menuOptions}
			value={menuValue}
			onchange={handleMenuChange}
			bind:open={menuOpen}
			emphasis="low"
			placeholder={menuValue}
			width={menuWidth || undefined}
			searchEnabled
			searchPlaceholder="Filter values…"
		/>
	</div>
{:else}
	<div class="editable-input-container" style="--actions-w: {actionsWidth}px;">
		<input
			bind:this={inputElement}
			bind:value={inputValue}
			oninput={handleInput}
			onkeydown={handleKeydown}
			onblur={handleBlur}
			class="editable-input"
			class:edited={isEdited}
		/>
		<p class="cell-ghost">{inputValue}</p>
		<div class="cell-actions" bind:clientWidth={actionsWidth}>
			<Button
				onmousedown={openModal}
				label="Open in editor (Shift+Enter)"
				leftIcon="arrow-diagonal"
				iconSize={16}
				size="sm"
				emphasis="low"
			/>
			{#if isEdited}
				<Button
					onmousedown={() => {
						onRollback();
						onEndEdit();
					}}
					label="Rollback edit"
					leftIcon="refresh"
					iconSize={16}
					size="sm"
					emphasis="low"
				/>
			{/if}
		</div>
	</div>
{/if}

<style>
	.editable-menu-container {
		/* in-flow, not absolute: the row sets contain:paint so an absolute
		   inset:0 would anchor to the whole tr, not this cell */
		position: relative;
		display: flex;
		align-items: stretch;
		width: 100%;
		height: 100%;
	}

	.editable-menu-container :global(.select-wrapper),
	.editable-menu-container :global(.select-container),
	.editable-menu-container :global(.select-trigger) {
		width: 100%;
		height: 100%;
	}

	.editable-menu-container :global(.select-trigger) {
		background-color: var(--gray-300);
		border-radius: var(--br-sm);
		outline: 0.5px solid var(--gray-800);
		outline-offset: -1px;
	}

	.editable-input-container {
		position: relative;
		display: block;
		width: 100%;
		height: 100%;
	}

	.cell-ghost {
		padding: var(--space-sm-md) var(--space-sm);
		visibility: hidden;
		white-space: pre;
		margin: 0;
		display: inline-block;
	}

	.editable-input {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		height: 100%;
		width: 100%;
		background-color: var(--gray-300);
		padding: var(--space-sm-md) var(--space-sm);
		/* always reserve room for the buttons, sized to the real cluster
		   (1 button when not edited, 2 when edited) */
		padding-right: calc(var(--space-sm) + var(--space-xs) + var(--actions-w, 0px));

		border: none;
		border-radius: var(--br-sm);
		outline: 0.5px solid var(--gray-600);
		outline-offset: -1px;

		box-shadow:
			var(--shadow) 0px 10px 10px -2px inset,
			var(--shadow) 0px 5px 5px -5px inset;

		box-sizing: border-box;
	}

	.editable-input.edited {
		outline-color: var(--green);
	}

	.cell-actions {
		position: absolute;
		right: var(--space-sm);
		top: 50%;
		transform: translateY(-50%);
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		z-index: 1;
		opacity: 0;
		transition: opacity 0.15s ease-out;
	}
	.editable-input-container:hover .cell-actions {
		opacity: 1;
	}
</style>
