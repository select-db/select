<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import { tryCatch } from '$lib/utils/tryCatch';

	type Props = {
		onClose: () => void;
		onCreate: (name: string) => Promise<void>;
	};

	let { onClose, onCreate }: Props = $props();
	let name = $state('');
	let creating = $state(false);

	const handleCreate = async () => {
		const trimmed = name.trim();
		if (!trimmed) return;
		creating = true;
		const [, err] = await tryCatch(onCreate, trimmed);
		creating = false;
		if (!err) onClose();
	};
</script>

<ModalHeader title="New role" />
<ModalBody>
	<Input
		bind:value={name}
		placeholder="Role name"
		autofocus
		onkeydown={(e) => e.key === 'Enter' && handleCreate()}
	/>
</ModalBody>
<ModalFooter
	mainAction={{
		label: 'Create',
		action: handleCreate,
		disabled: creating || !name.trim()
	}}
/>
