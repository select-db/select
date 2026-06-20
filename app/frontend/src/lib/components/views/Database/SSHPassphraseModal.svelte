<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';
	import Input from '$lib/system/Input/Input.svelte';

	type Props = {
		host?: string;
		onSubmit: (value: string) => void;
		onCancel: () => void;
	};

	let { host = '', onSubmit, onCancel }: Props = $props();

	let value = $state('');

	const submit = () => onSubmit(value);
	const onkeydown = (e: KeyboardEvent) => {
		if (e.key === 'Enter') submit();
	};
</script>

<ModalHeader icon="key" title="SSH key passphrase" />

<ModalBody>
	<p class="text">
		The private key for <strong>{host || 'this host'}</strong> is encrypted.
	</p>
	<p class="text">Enter its passphrase to connect.</p>
	<Input bind:value type="password" placeholder="Passphrase" autofocus {onkeydown} />
</ModalBody>

<ModalFooter
	secondaryAction={{ label: 'Cancel', action: async () => onCancel() }}
	mainAction={{ label: 'Connect', action: async () => submit() }}
/>

<style>
	.text {
		margin: 0 0 var(--space-sm);
		font-size: 13px;
		color: var(--gray-800);
	}
</style>
