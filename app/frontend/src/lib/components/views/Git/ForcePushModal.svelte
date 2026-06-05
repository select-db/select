<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import Button from '$lib/system/Button/Button.svelte';

	type Props = {
		onClose: () => void;
		onConfirm: () => Promise<void>;
		branchName?: string;
	};

	let { onClose, onConfirm, branchName = 'this branch' }: Props = $props();
	let understood = $state(false);

	const handleConfirm = async () => {
		if (!understood) return;
		await onConfirm();
		onClose();
	};
</script>

<ModalHeader title="Force push" icon="github-branch" />

<div class="statusBar"></div>

<ModalBody>
	<p class="hint">Push was rejected. The remote has new commits.</p>
	<p class="hint">
		&#x2022; Force push will overwrite the remote {branchName}.
	</p>
	<p class="hint">&#x2022; Only do this if no one else has pushed.</p>
	<p class="hint">&#x2022; <strong>OR</strong> if you intend to replace the remote history.</p>

	<div class="divider"></div>

	<label class="checkbox">
		<input type="checkbox" bind:checked={understood} />
		<span>I understand this will overwrite the remote</span>
	</label>

	<div class="actions">
		<Button content="Cancel" size="sm" emphasis="high" onclick={onClose} />
		<Button
			content="Force push"
			size="sm"
			emphasis={understood ? 'high' : 'low'}
			disabled={!understood}
			onclick={handleConfirm}
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

	.checkbox {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		font-size: var(--fs-xs);

		margin-top: var(--space-sm-md);
		margin-bottom: var(--space-sm-md);
	}

	.checkbox span {
		color: var(--gray-900);
	}

	.actions {
		display: flex;
		gap: var(--space-sm);
	}
</style>
