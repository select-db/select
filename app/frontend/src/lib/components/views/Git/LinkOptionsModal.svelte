<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import Button from '$lib/system/Button/Button.svelte';

	type Props = {
		branch: string;
		onClose: () => void;
		onChoice: (choice: 'checkout' | 'keep') => Promise<void>;
	};

	let { branch, onClose, onChoice }: Props = $props();

	const handle = async (choice: 'checkout' | 'keep') => {
		await onChoice(choice);
		onClose();
	};
</script>

<ModalHeader title="Link options" icon="github-branch" />

<div class="statusBar"></div>

<ModalBody>
	<p class="hint">The remote branch <strong>{branch}</strong> has commits.</p>
	<p class="hint">Your local workspace has no commits yet.</p>

	<div class="actions">
		<Button
			content="Pull from remote"
			size="sm"
			emphasis="high"
			onclick={() => handle('checkout')}
		/>
		<Button content="Cancel" size="sm" emphasis="low" onclick={onClose} />
	</div>
</ModalBody>

<div class="options">
	<Button
		content="Keep my local files (set up remote without overwriting)"
		size="sm"
		emphasis="low"
		onclick={() => handle('keep')}
	/>
</div>

<style>
	.statusBar {
		width: 100%;
		height: 0;
		border-top: 0.5px var(--red-glow) solid;
	}

	.hint {
		color: var(--gray-900);
		margin: 0 0 var(--space-sm) 0;
		white-space: break;
	}

	.actions,
	.options {
		display: flex;
	}

	.actions {
		padding-top: var(--space-sm-md);
		justify-content: space-between;
	}

	.options {
		flex-direction: column;
		gap: var(--space-xs);
		border-top: var(--border);
		padding: var(--space-sm);
		background-color: var(--gray-200);
	}
</style>
