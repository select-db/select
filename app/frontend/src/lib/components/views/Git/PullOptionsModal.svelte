<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import Button from '$lib/system/Button/Button.svelte';

	type Props = {
		onClose: () => void;
		onChoice: (choice: 'merge' | 'rebase' | 'reset') => Promise<void>;
	};

	let { onClose, onChoice }: Props = $props();

	const handle = async (choice: 'merge' | 'rebase' | 'reset') => {
		await onChoice(choice);
		onClose();
	};
</script>

<ModalHeader title="Pull options" icon="github-branch" />

<div class="statusBar"></div>

<ModalBody>
	<p class="hint">Your branch and the remote have diverged.</p>
	<p class="hint">How do you want to pull?</p>

	<div class="actions">
		<Button content="Merge" size="sm" emphasis="high" onclick={() => handle('merge')} />
		<Button content="Cancel" size="sm" emphasis="low" onclick={onClose} />
	</div>

	<div class="divider"></div>

	<div class="options">
		<Button content="Pull with rebase" size="sm" emphasis="low" onclick={() => handle('rebase')} />
		<Button
			content="Reset to remote (discard my local commits)"
			size="sm"
			emphasis="low"
			onclick={() => handle('reset')}
		/>
	</div>
</ModalBody>

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

	.divider {
		margin-top: var(--space-sm-md);
		border-top: var(--border);
	}

	.actions,
	.options {
		display: flex;
		margin-top: var(--space-sm-md);
	}

	.actions {
		justify-content: space-between;
	}

	.options {
		flex-direction: column;
		gap: var(--space-xs);
	}
</style>
