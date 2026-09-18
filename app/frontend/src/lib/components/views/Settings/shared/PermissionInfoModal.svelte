<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';

	type Props = { title: string; description: string; onClose: () => void };
	let { title, description }: Props = $props();
</script>

<ModalHeader {title} icon="info" />
<ModalBody>
	<p style="white-space: wrap;">{description}</p>
</ModalBody>
