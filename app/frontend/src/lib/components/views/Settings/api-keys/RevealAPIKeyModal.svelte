<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { AlertType } from '$lib/system/Alert/types';

	type Props = {
		onClose: () => void;
		apiKey: string;
	};

	let { onClose, apiKey }: Props = $props();
	let copied = $state(false);

	async function copy() {
		try {
			await navigator.clipboard.writeText(apiKey);
			copied = true;
			notify({ type: AlertType.Success, message: 'API key copied to clipboard' });
		} catch {
			notify({ type: AlertType.Error, message: 'Could not copy. Select and copy manually.' });
		}
	}
</script>

<ModalHeader icon="key" title="Copy your API key" />
<ModalBody>
	<p class="warn">
		This is the only time the key is shown. Store it now; it cannot be retrieved again.
	</p>
	<code class="key selectable">{apiKey}</code>
</ModalBody>
<ModalFooter
	secondaryAction={{ label: copied ? 'Copied' : 'Copy', action: copy }}
	mainAction={{ label: 'Done', action: async () => onClose() }}
/>

<style>
	.warn {
		color: var(--gray-900);
		font-size: var(--fs-sm);
		margin-bottom: var(--space-sm);
		text-wrap: wrap;
	}
	.key {
		display: block;
		padding: var(--space-sm);
		border: var(--border);
		border-radius: var(--br-xs);
		background: var(--gray-100);
		font-family: var(--font-mono, monospace);
		font-size: var(--fs-xs);
		word-break: break-all;
		color: var(--gray-900);
	}
</style>
