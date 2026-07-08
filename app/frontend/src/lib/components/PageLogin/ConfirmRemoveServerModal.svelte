<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import { tryCatch } from '$lib/utils/tryCatch';

	type Props = {
		domain: string;
		onClose: () => void;
		onConfirm: () => Promise<void>;
	};

	let { domain, onClose, onConfirm }: Props = $props();
	let removing = $state(false);

	async function handleConfirm() {
		removing = true;
		const [, err] = await tryCatch(onConfirm);
		removing = false;
		if (!err) onClose();
	}
</script>

<ModalHeader title="Remove server" />
<ModalBody>
	<p class="message">
		Remove server "{domain}"? Its data on this device will be deleted. You can add it again later.
	</p>
</ModalBody>
<footer class="footer">
	<Button content="Cancel" size="sm" emphasis="high" onclick={onClose} disabled={removing} />
	<Button content="Remove" size="sm" emphasis="low" onclick={handleConfirm} disabled={removing} />
</footer>

<style>
	.message {
		color: var(--gray-900);
		text-wrap: wrap;
	}
	.footer {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-sm);
		padding: var(--space-sm);
		border-top: var(--border);
		background-color: var(--gray-200);
	}
</style>
