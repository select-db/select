<script lang="ts">
	import { untrack } from 'svelte';
	import { clickOutside } from '$lib/utils/clickOutside';
	import { renamingItemIdStore } from '$lib/components/views/shared/sharedStore';
	import { updateFileTabsAfterRename } from '$lib/components/Layout/layoutStore';

	import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';
	import { RenameDatabase } from '$lib/bindings/selectDb/internal/graph/graph';
	import { tryCatch } from '$lib/utils/tryCatch';
	import { notifyError } from '$lib/system/Notifications/notificationsStore';

	let {
		id,
		uri,
		name,
		muted,
		type
	}: {
		id: string;
		/** Where the item is. The same as `id` for a file or folder, but a
		 *  database answers to its config's id, which is not a path. */
		uri: string;
		name: string;
		muted: boolean;
		type?: string;
	} = $props();

	// Seeded once; the effect below re-syncs both when the prop changes.
	let initialName = $state(untrack(() => name));
	let localName = $state(untrack(() => name));

	$effect(() => {
		initialName = name;
		localName = name;
	});

	let inputEl: HTMLInputElement;

	// Prevents the keyup from the triggering Enter press from immediately closing rename mode.
	// WebKit delivers keyup to the element that has focus at release time, not at press time.
	let ignoreNextKeyup = false;

	const RENAMABLE_TYPES = new Set(['file', 'folder', 'db_instance']);

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

		// A database is a directory named after itself, so renaming one is not
		// renaming a path: the backend cleans the name into something a
		// filesystem will take, and refuses one a sibling already has.
		if (type === 'db_instance') {
			const [renamed, dbErr] = await tryCatch(RenameDatabase, { uri, name: trimmed });
			if (dbErr || !renamed) {
				notifyError(dbErr?.message ?? `Could not rename ${initialName}`);
				localName = initialName;
				return;
			}
			// What it ended up called, which is not always what was typed.
			localName = renamed.name;
			updateFileTabsAfterRename(uri, renamed.uri, renamed.name);
			return;
		}

		// For files and folders, rename the item
		const lastSlash = id.lastIndexOf('/');
		const base = lastSlash === -1 ? id : id.slice(0, lastSlash + 1);
		const newUri = `${base}${trimmed}`;

		if (id === newUri) return;

		const [, err] = await tryCatch(fs.Rename, {
			old_uri: id,
			new_uri: newUri
		});

		// A refused rename — a name already taken, most often — leaves the file
		// where it is, so the row has to go back to saying so.
		if (err) {
			notifyError(err.message);
			localName = initialName;
			return;
		}

		// Update any open file tabs to point at the new path (keeps tab open)
		updateFileTabsAfterRename(id, newUri, trimmed);
	};
</script>

<!-- svelte-ignore a11y_autofocus -->
<input
	aria-label="Name"
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
