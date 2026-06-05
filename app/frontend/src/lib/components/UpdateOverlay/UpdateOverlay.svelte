<script lang="ts">
	import Loader from '$lib/system/Loader/Loader.svelte';
	import { updateStore } from '$lib/stores/updateStore';
</script>

{#if $updateStore && ($updateStore.status === 'downloading' || $updateStore.status === 'installing')}
	<div class="overlay">
		<div class="card">
			<Loader size={28} />

			<div class="text">
				<p class="title">
					{$updateStore.status === 'installing' ? 'Installing update' : 'Updating Select'}
					{#if $updateStore.version}
						<span class="version">v{$updateStore.version}</span>
					{/if}
				</p>
				<p class="message">{$updateStore.message}</p>
			</div>

			{#if $updateStore.status === 'downloading' && $updateStore.progress > 0}
				<div class="progress-track">
					<div class="progress-bar" style="width: {$updateStore.progress}%"></div>
				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.overlay {
		position: fixed;
		inset: 0;
		z-index: 100;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(0, 0, 0, 0.6);
		backdrop-filter: blur(4px);
	}

	.card {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-md);
		background: var(--gray-0);
		border: var(--border);
		border-radius: var(--br-md);
		padding: var(--space-lg) calc(var(--space-lg) * 2);
		min-width: 280px;
	}

	.text {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-sm);
		text-align: center;
	}

	.title {
		font-size: var(--fs-lg);
		color: var(--gray-1000);
		display: flex;
		align-items: center;
		gap: var(--space-xs);
	}

	.version {
		font-size: var(--fs-lg);
		color: var(--gray-600);
	}

	.message {
		color: var(--gray-600);
	}

	.progress-track {
		width: 100%;
		height: 3px;
		background: var(--gray-200);
		border-radius: 99px;
		overflow: hidden;
	}

	.progress-bar {
		height: 100%;
		background: var(--gray-1000);
		border-radius: 99px;
		transition: width 0.2s ease;
	}
</style>
