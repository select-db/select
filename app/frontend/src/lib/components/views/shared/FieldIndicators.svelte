<script lang="ts">
	import Icon from '$lib/system/Icon/Icon.svelte';
	import Tooltip from '$lib/system/Tooltip/Tooltip.svelte';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import ItemInfoModal from '$lib/components/views/FileSystem/modals/ItemInfoModal.svelte';

	type IndicatorItem = {
		type?: string;
		children?: unknown[];
		metadata?: {
			isPrimaryKey?: boolean;
			isForeignKey?: boolean;
			hasIndex?: boolean;
			indexes?: unknown[];
		};
	};

	let { item, active = true }: { item: IndicatorItem; active?: boolean } = $props();

	const isColumn = $derived(typeof item?.type === 'string' && item.type.startsWith('column:'));
	const isPrimaryKey = $derived(isColumn && item?.metadata?.isPrimaryKey === true);
	const isForeignKey = $derived(isColumn && item?.metadata?.isForeignKey === true);
	const hasIndex = $derived(
		isColumn &&
			(item?.metadata?.hasIndex === true ||
				(Array.isArray(item?.metadata?.indexes) && item.metadata.indexes.length > 0))
	);

	const openItemInfo = (e: MouseEvent) => {
		e.preventDefault();
		e.stopPropagation();
		modalStore.set({
			content: () => ItemInfoModal,
			props: { item },
			width: 600
		});
	};
</script>

{#if isPrimaryKey || isForeignKey || hasIndex}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="item-indicators" class:inactive={!active} onclick={openItemInfo}>
		{#if hasIndex}
			<Tooltip text="Indexed" position="top" actionable={false}>
				<div class="index-container">
					<span class="index-count">{item.children?.length ?? 0}</span>
					<Icon icon="index" size={14} stroke="var(--green)" />
				</div>
			</Tooltip>
		{/if}

		{#if isPrimaryKey}
			<Tooltip text="Primary key" position="top" actionable={false}>
				<Icon icon="key" size={14} stroke="var(--yellow)" />
			</Tooltip>
		{/if}

		{#if isForeignKey}
			<Tooltip text="Foreign key" position="top" actionable={false}>
				<Icon icon="key" size={14} stroke="var(--blue)" />
			</Tooltip>
		{/if}
	</div>
{/if}

<style>
	.item-indicators {
		height: 22px;
		display: flex;
		align-items: center;
		gap: var(--space-xxs);
		padding: 0 var(--space-xs);
		background-color: var(--gray-300);
		border-radius: var(--br-md);

		transition: background-color 0.1s ease-out;
	}

	.item-indicators:hover:not(.inactive) {
		background-color: var(--gray-600);
	}

	.index-container {
		display: flex;
		align-items: center;
		gap: var(--space-xxs);
		margin: 0 var(--space-xxs);
	}

	.index-count {
		font-size: var(--fs-xs);
		color: var(--gray-700);
		transition: color 0.1s ease-out;
	}

	.item-indicators:hover:not(.inactive) .index-count {
		color: var(--gray-900);
	}
</style>
