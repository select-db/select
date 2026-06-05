<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';

	type Props = {
		onClose: () => void;
		onConfirm: () => Promise<void>;
		title: string;
		message: string;
	};

	let { onClose, onConfirm, title, message }: Props = $props();

	const handleConfirm = async () => {
		await onConfirm();
		onClose();
	};
</script>

<ModalHeader {title} icon="refresh" />
<ModalBody>
	<p class="message">{message}</p>
</ModalBody>
<ModalFooter mainAction={{ label: 'Reset', action: handleConfirm }} />

<style>
	.message {
		color: var(--gray-900);
		text-wrap: wrap;
	}
</style>
