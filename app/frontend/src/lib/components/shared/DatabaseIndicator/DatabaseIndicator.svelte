<script lang="ts">
	import Loader from '$lib/system/Loader/Loader.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import { loadingStore } from '$lib/utils/query/loadingStore';
	import { databaseAvailabilityStore } from '$lib/components/shared/DatabaseIndicator/databaseIndicatorStore';

	type DatabaseIndicatorProps = {
		id?: string | null;
		size?: number;
		loaderSize?: number;
		error?: boolean;
	};

	let { id = null, size = 18, loaderSize = 16, error = false }: DatabaseIndicatorProps = $props();

	const dbPrefix = $derived(() => (id ? `db:${id}` : null));
	const isLoading = $derived(() => {
		const prefix = dbPrefix();
		if (!prefix) return false;
		return $loadingStore.some((entry) => entry.startsWith(prefix));
	});
</script>

{#if id}
	{#if isLoading()}
		<Loader size={loaderSize} />
	{:else if error}
		<div class="icon-wrapper" style={`--indicator-size: ${size}px`}>
			<div class="dot-wrapper">
				<Icon icon="db" {size} />
				<span class="error-cross">
					<Icon icon="cross" size={size * 0.55} stroke="var(--red)" strokeWidth={3} />
				</span>
			</div>
		</div>
	{:else}
		{@const isAvailable = $databaseAvailabilityStore.has(id)}
		<div class="icon-wrapper" style={`--indicator-size: ${size}px`}>
			<div class="dot-wrapper">
				<Icon icon="db" {size} stroke="var(--gray-800)" />
				<span
					class="status-dot {isAvailable === true
						? 'status-dot--online'
						: isAvailable === false
							? 'status-dot--offline'
							: 'status-dot--unknown'}"
				></span>
			</div>
		</div>
	{/if}
{:else}
	<Icon icon="no-db" {size} stroke="var(--gray-800)" />
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

	.error-cross {
		position: absolute;
		right: -1px;
		bottom: 1px;
		display: flex;
		background: var(--gray-0);
		border-radius: 999px;
	}
</style>
