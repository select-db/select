<script lang="ts">
	import Portal from '$lib/system/Portal/Portal.svelte';
	import FloatingBox from '$lib/system/FloatingBox/FloatingBox.svelte';
	import Menu from '$lib/system/Menu/Menu.svelte';
	import { hiddenChildrenStore, toggleChildVisibility } from './helpers/childVisibilityStore';
	import { expandedItemIdsStore } from '$lib/components/views/shared/sharedStore';
	import type { MenuOption } from '$lib/system/Menu/Menu.types';

	type Item = { id: string; name: string };

	type Props = {
		parentId: string;
		items: Item[];
	};
	let { parentId, items }: Props = $props();

	let anchor = $state<HTMLElement | null>(null);
	let open = $state(false);

	const expanded = $derived($expandedItemIdsStore.get(parentId) === true);
	const hidden = $derived(new Set($hiddenChildrenStore[parentId] ?? []));
	const visibleCount = $derived(items.filter((c) => !hidden.has(c.id)).length);
	const total = $derived(items.length);

	const options = $derived<MenuOption[]>(
		items.map((c) => ({
			id: c.id,
			label: c.name,
			checkbox: true,
			checked: !hidden.has(c.id),
			action: () => toggleChildVisibility(parentId, c.id),
			onCheckClick: () => toggleChildVisibility(parentId, c.id)
		}))
	);

	function openMenu(e: MouseEvent) {
		e.stopPropagation();
		e.preventDefault();
		anchor = e.currentTarget as HTMLElement;
		open = true;
	}
</script>

{#if expanded}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<p
		class="child-vis-badge"
		data-test="tree.visibility-badge"
		onclick={openMenu}
		title="Show/hide children"
	>
		{visibleCount} of {total}
	</p>
{/if}

{#if open && anchor}
	<Portal>
		<FloatingBox {anchor} backdrop onBackdropClick={() => (open = false)}>
			<Menu
				{options}
				searchEnabled={total > 10}
				width={200}
				maxHeight={360}
				onClose={() => (open = false)}
			/>
		</FloatingBox>
	</Portal>
{/if}

<style>
	.child-vis-badge {
		color: var(--gray-700);
		font-size: var(--fs-xs);
		display: inline-flex;
		align-items: center;
		padding: var(--space-xs) 0;
	}
	.child-vis-badge:hover {
		color: var(--gray-900);
	}
</style>
