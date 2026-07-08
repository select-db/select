<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';
	import { must, tryCatch } from '$lib/utils/tryCatch';

	type Props = {
		onClose: () => void;
		onConfirm: () => Promise<void>;
		groupName: string;
	};

	let { onClose, onConfirm, groupName }: Props = $props();
	let deleting = $state(false);

	const handleConfirm = async () => {
		deleting = true;
		await must(tryCatch(onConfirm), null, () => {
			deleting = false;
		});
		deleting = false;
		onClose();
	};
</script>

<ModalHeader title="Delete group" />
<ModalBody>
	<p class="message">
		Delete "{groupName}"? Its members and role assignments will be removed. Users keep any roles
		granted to them directly.
	</p>
</ModalBody>
<ModalFooter
	secondaryAction={{ label: 'Cancel', action: async () => onClose(), disabled: deleting }}
	mainAction={{ label: 'Delete', action: handleConfirm, disabled: deleting }}
/>

<style>
	.message {
		color: var(--gray-900);
		text-wrap: wrap;
	}
</style>
