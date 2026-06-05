<script lang="ts">
	import Loader from '$lib/system/Loader/Loader.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import { serverIndicatorStore } from '$lib/components/shared/ServerIndicator/serverIndicatorStore';
	import type { ServerStatus } from '$lib/components/shared/ServerIndicator/serverIndicatorStore';

	type ServerIndicatorProps = {
		domain?: string | null;
		size?: number;
		loaderSize?: number;
		showVersion?: boolean;
		showWarning?: boolean;
	};

	let {
		domain = null,
		size = 18,
		loaderSize = 16,
		showVersion = false,
		showWarning = true
	}: ServerIndicatorProps = $props();

	const stateByDomain = $derived($serverIndicatorStore);
	const domainState = $derived(
		domain != null
			? (stateByDomain[domain] ?? { status: 'idle' as ServerStatus, manifest: null })
			: null
	);
	const status: ServerStatus = $derived(domainState?.status ?? 'idle');
	const manifest = $derived(domainState?.manifest ?? null);
</script>

{#if domain == null}
	<Icon icon="server" {size} stroke="var(--gray-800)" />
{:else if status === 'loading'}
	<Loader size={loaderSize} />
{:else}
	<div class="indicator-row">
		<div class="icon-wrapper" style={`--indicator-size: ${size}px`}>
			<div class="dot-wrapper">
				<Icon icon="server" {size} stroke="var(--gray-800)" />
				<span
					class="status-dot {status === 'live'
						? 'status-dot--online'
						: status === 'offline'
							? 'status-dot--offline'
							: 'status-dot--idle'}"
				></span>
			</div>
		</div>
		{#if showVersion && manifest?.backend_version}
			<span class="version">v{manifest.backend_version}</span>
		{/if}
	</div>
	{#if showWarning && manifest?.warning}
		<p class="warning">{manifest.warning}</p>
	{/if}
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

	.status-dot--idle {
		background-color: var(--gray-400);
	}

	.indicator-row {
		display: inline-flex;
		align-items: center;
	}

	.version {
		font-size: var(--fs-xs);
		color: var(--gray-600);
		margin-left: var(--space-xs);
	}

	.warning {
		font-size: var(--fs-xs);
		color: var(--orange);
		margin: var(--space-xs) 0 0;
		max-width: 280px;
	}
</style>
