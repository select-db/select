<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import { tryCatch } from '$lib/utils/tryCatch';

	type Props = {
		currentName: string;
		onClose: () => void;
		onRename: (name: string) => Promise<void>;
	};

	let { currentName, onClose, onRename }: Props = $props();
	let name = $state(currentName);
	let saving = $state(false);

	const handleRename = async () => {
		const trimmed = name.trim();
		if (!trimmed) return;
		if (trimmed === currentName) {
			onClose();
			return;
		}
		saving = true;
		const [, err] = await tryCatch(onRename, trimmed);
		saving = false;
		if (!err) onClose();
	};
</script>

<ModalHeader title="Rename group" />
<ModalBody>
	<Input
		bind:value={name}
		placeholder="Group name"
		autofocus
		onkeydown={(e) => e.key === 'Enter' && handleRename()}
		style="width: 367px"
	/>
</ModalBody>
<ModalFooter
	mainAction={{
		label: 'Rename',
		action: handleRename,
		disabled: saving || !name.trim()
	}}
/>
