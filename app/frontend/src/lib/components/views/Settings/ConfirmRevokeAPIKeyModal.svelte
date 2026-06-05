<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';
	import { must, tryCatch } from '$lib/utils/tryCatch';

	type Props = {
		onClose: () => void;
		onConfirm: () => Promise<void>;
		keyName: string;
	};

	let { onClose, onConfirm, keyName }: Props = $props();
	let revoking = $state(false);

	const handleConfirm = async () => {
		revoking = true;
		await must(tryCatch(onConfirm), null, () => {
			revoking = false;
		});
		revoking = false;
		onClose();
	};
</script>

<ModalHeader title="Revoke API key" />
<ModalBody>
	<p class="message">
		Revoke "{keyName}"? Any client using it stops working immediately. This cannot be undone.
	</p>
</ModalBody>
<ModalFooter
	secondaryAction={{ label: 'Cancel', action: async () => onClose(), disabled: revoking }}
	mainAction={{ label: 'Revoke', action: handleConfirm, disabled: revoking }}
/>

<style>
	.message {
		color: var(--gray-900);
		text-wrap: wrap;
	}
</style>
