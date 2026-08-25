<script lang="ts">
	import { clickOutside } from '$lib/utils/clickOutside';
	import { renamingItemIdStore } from '$lib/components/views/shared/sharedStore';
	import { updateFileTabsAfterRename } from '$lib/components/Layout/layoutStore';

	import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';
	import { must, tryCatch } from '$lib/utils/tryCatch';

	let {
		id,
		name,
		muted,
		type
	}: {
		id: string;
		name: string;
		muted: boolean;
		type?: string;
	} = $props();

	let initialName = $state(name);
	let localName = $state(name);

	$effect(() => {
		initialName = name;
		localName = name;
	});

	let inputEl: HTMLInputElement;

	// Prevents the keyup from the triggering Enter press from immediately closing rename mode.
	// WebKit delivers keyup to the element that has focus at release time, not at press time.
	let ignoreNextKeyup = false;

	const RENAMABLE_TYPES = new Set(['file', 'folder']);

	$effect(() => {
		if ($renamingItemIdStore === id && inputEl) {
			if (type && !RENAMABLE_TYPES.has(type)) {
				renamingItemIdStore.set(null);
				return;
			}
			requestAnimationFrame(() => {
				ignoreNextKeyup = true;
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

		// For files and folders, rename the item
		const lastSlash = id.lastIndexOf('/');
		const base = lastSlash === -1 ? id : id.slice(0, lastSlash + 1);
		const newUri = `${base}${trimmed}`;

		if (id === newUri) return;

		await must(
			tryCatch(fs.Rename, {
				old_uri: id,
				new_uri: newUri
			})
		);

		// Update any open file tabs to point at the new path (keeps tab open)
		updateFileTabsAfterRename(id, newUri, trimmed);
	};
</script>

<!-- svelte-ignore a11y_autofocus -->
<input
	class={`${$renamingItemIdStore === id ? `showing` : `hidden`} name-input`}
	onkeydown={(e) => {
		if ($renamingItemIdStore !== id) return;
		if (e.key === 'Enter' || e.key === 'Escape') {
			e.preventDefault();
			e.stopPropagation();
		}
	}}
	onkeyup={async (e) => {
		if (ignoreNextKeyup && e.key === 'Enter') {
			ignoreNextKeyup = false;
			return;
		}
		ignoreNextKeyup = false;
		if ($renamingItemIdStore !== id) return;
		if (!['Enter', 'Escape'].includes(e.key)) return;
		switch (e.key) {
			case 'Enter':
				await stopEditing(true);
				break;
			case 'Escape':
				await stopEditing(false);
				break;
		}
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
