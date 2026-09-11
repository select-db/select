<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';
	import { folderStore } from '$lib/components/PageFolder/folderStore';
	import { must, tryCatch } from '$lib/utils/tryCatch';

	type Props = {
		onClose: () => void;
		onConfirm: () => Promise<void>;
		workspaceName: string;
	};

	let { onClose, onConfirm, workspaceName }: Props = $props();
	let deleting = $state(false);

	const server = $derived($folderStore?.currentServer ?? '');

	const handleConfirm = async () => {
		deleting = true;
		await must(tryCatch(onConfirm), null, () => {
			deleting = false;
		});
		deleting = false;
		onClose();
	};
</script>

<ModalHeader title="Delete workspace" />
<ModalBody>
	<p class="message">
		Deletes <strong>{workspaceName}</strong>
		{#if server}from <strong>{server}</strong>{/if}, for everyone in it: its users, roles and
		permissions go with it.
	</p>
	<p class="message">
		Your files stay where they are. Only <code>select.config.json</code> is removed from the folder,
		and you are signed out.
	</p>
</ModalBody>
<ModalFooter
	secondaryAction={{ label: 'Cancel', action: async () => onClose(), disabled: deleting }}
	mainAction={{ label: 'Delete', action: handleConfirm, disabled: deleting }}
/>

<style>
	.message {
		margin: 0 0 var(--space-sm);
		color: var(--gray-900);
		text-wrap: pretty;
	}

	.message:last-child {
		margin-bottom: 0;
	}

	code {
		font-family: 'JetBrains Mono', monospace;
		font-size: var(--fs-xs);
		word-break: break-all;
	}
</style>
