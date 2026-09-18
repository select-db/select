<script lang="ts">
	import { untrack } from 'svelte';
	import { clickOutside } from '$lib/utils/clickOutside';
	import { renamingItemIdStore } from '$lib/components/views/shared/sharedStore';
	import { renameEntry } from '$lib/components/views/shared/renameEntry';

	let {
		id,
		uri,
		name,
		muted,
		type
	}: {
		id: string;
		/** Where the item is, which for a database is not its `id`. */
		uri: string;
		name: string;
		muted: boolean;
		type?: string;
	} = $props();

	// Seeded once; the effect below re-syncs both when the prop changes.
	let initialName = $state(untrack(() => name));
	let localName = $state(untrack(() => name));

	$effect(() => {
		// A rename in progress owns the field. The row re-renders for reasons that
		// have nothing to do with it -- a graph rebuild, a config written by
		// something else -- and syncing then overwrites what is being typed.
		if ($renamingItemIdStore === id) return;

		initialName = name;
		localName = name;
	});

	let inputEl: HTMLInputElement;

	// Focus and selection are placed when the box opens, not again while it stays
	// open: re-selecting mid-rename puts the caret back and the next keystroke
	// replaces what was typed.
	let placed = false;

	const RENAMABLE_TYPES = new Set(['file', 'folder', 'db_instance']);

	$effect(() => {
		if ($renamingItemIdStore !== id) {
			placed = false;
			return;
		}

		if (inputEl && !placed) {
			if (type && !RENAMABLE_TYPES.has(type)) {
				renamingItemIdStore.set(null);
				return;
			}
			placed = true;
			requestAnimationFrame(() => {
				inputEl?.focus();

				const firstDotIndex = localName.indexOf('.');
				if (firstDotIndex !== -1) {
					// Select from start to first dot (excluding the dot)
					inputEl?.setSelectionRange(0, firstDotIndex);
				} else {
					// No dot found, select all text
					inputEl?.select();
				}
			});
		}
	});

	const stopEditing = async (shouldSave: boolean) => {
		if ($renamingItemIdStore !== id) return;

		renamingItemIdStore.set(null);

		if (!shouldSave) {
			localName = initialName;
			return;
		}

		const trimmed = localName.trim();

		// A refused rename leaves the entry where it is, so the row has to go
		// back to saying so.
		if (!(await renameEntry(uri, trimmed))) {
			localName = initialName;
		}
	};
</script>

<!-- svelte-ignore a11y_autofocus -->
<input
	aria-label="Name"
	class={`${$renamingItemIdStore === id ? `showing` : `hidden`} name-input`}
	onkeydown={async (e) => {
		if ($renamingItemIdStore !== id) return;
		if (e.key !== 'Enter' && e.key !== 'Escape') return;

		e.preventDefault();
		e.stopPropagation();

		// On keydown, not keyup. The Enter that opens rename mode is pressed on
		// the menu item, so its keydown never reaches this input -- only its
		// keyup does, since WebKit delivers keyup to whatever has focus at
		// release time. Committing on keydown tells the two Enters apart with no
		// state to keep, and nothing to go stale when the row re-renders.
		await stopEditing(e.key === 'Enter');
	}}
	bind:value={localName}
	bind:this={inputEl}
	use:clickOutside={() => stopEditing(true)}
	autofocus
	autocapitalize="off"
	autocomplete="off"
	spellcheck="false"
/>

<p class={`name ${$renamingItemIdStore === id ? 'hidden' : 'showing'}`} class:muted>
	{localName}
</p>

<style>
	.name-input {
		width: 100%;
		min-width: 0;
		height: 100%;
		padding: 0 var(--space-sm) 0 var(--space-xs-sm);
		background: var(--gray-400);
		border: none;

		color: var(--gray-1000);
		margin-left: -5px;
	}

	.name {
		padding-right: var(--space-sm);
	}
	.name.muted {
		color: var(--gray-700);
	}

	.name-input.showing,
	.name.showing {
		display: block;
	}

	.name-input.hidden,
	.name.hidden {
		display: none;
	}
</style>
