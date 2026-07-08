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
	let rotating = $state(false);

	const handleConfirm = async () => {
		rotating = true;
		await must(tryCatch(onConfirm), null, () => {
			rotating = false;
		});
		rotating = false;
	};
</script>

<ModalHeader title="Rotate API key" />
<ModalBody>
	<p class="message">
		This will generate a new key for "{keyName}". The current key remains valid for 24 hours.
	</p>
</ModalBody>
<ModalFooter
	secondaryAction={{ label: 'Cancel', action: async () => onClose(), disabled: rotating }}
	mainAction={{ label: 'Rotate', action: handleConfirm, disabled: rotating }}
/>

<style>
	.message {
		color: var(--gray-900);
		text-wrap: wrap;
	}
</style>
