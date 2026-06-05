<script lang="ts">
	import Loader from '$lib/system/Loader/Loader.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import { loadingStore } from '$lib/utils/query/loadingStore';
	import { databaseAvailabilityStore } from '$lib/components/shared/DatabaseIndicator/databaseIndicatorStore';

	type Props = {
		ids: string[];
		size?: number;
		loaderSize?: number;
	};

	let { ids, size = 18, loaderSize = 16 }: Props = $props();

	const isLoading = $derived(
		ids.some((id) => $loadingStore.some((entry) => entry.startsWith(`db:${id}`)))
	);

	const allConnected = $derived(
		ids.length > 0 && ids.every((id) => $databaseAvailabilityStore.has(id))
	);
</script>

{#if ids.length === 0}
	<Icon icon="no-db" {size} stroke="var(--gray-800)" />
{:else if isLoading}
	<Loader size={loaderSize} />
{:else}
	<div class="icon-wrapper" style={`--indicator-size: ${size}px`}>
		<div class="dot-wrapper">
			<Icon icon="server" {size} stroke="var(--gray-800)" />
			<span class="status-dot {allConnected ? 'status-dot--online' : 'status-dot--offline'}">
			</span>
		</div>
	</div>
{/if}

<style>
	.icon-wrapper {
		height: 16px;
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: center;
	}

	.icon-wrapper .dot-wrapper {
		padding-top: var(--space-xxs);
		position: relative;
		height: fit-content;
	}

	.status-dot {
		position: absolute;
		right: -1px;
		bottom: 1px;
		width: calc(var(--indicator-size, 16px) * 0.3);
		height: calc(var(--indicator-size, 16px) * 0.3);
		border-radius: 999px;
		border: 2px solid var(--gray-0);
	}

	.status-dot--online {
		background-color: var(--green);
	}

	.status-dot--offline {
		background-color: var(--red);
	}
</style>
