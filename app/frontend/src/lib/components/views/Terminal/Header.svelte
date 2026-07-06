<script lang="ts">
	import { onMount } from 'svelte';
	import Select from '$lib/system/Select/Select.svelte';
	import * as TerminalBackend from '$lib/wailsjs/go/terminal/Terminal';
	import { must, tryCatch } from '$lib/utils/tryCatch';

	type Props = {
		shell: string;
		onShellChange?: (shell: string) => void;
	};

	let { shell, onShellChange }: Props = $props();

	let options: Array<{ value: string; label: string }> = $state([]);
	let isLoading = $state(true);

	onMount(async () => {
		const shells = await must(tryCatch(TerminalBackend.GetAvailableShells));
		options = shells.map((s) => ({ value: s.path, label: s.name }));
		isLoading = false;
	});
</script>

<div class="wrapper">
	<div class="left-wrapper">
		<Select
			{options}
			{isLoading}
			value={shell || options[0]?.value || ''}
			onchange={(v) => {
				const s = v as string;
				if (s && s !== shell) onShellChange?.(s);
			}}
			placeholder="Shell"
			size="xs"
			width={85}
		/>
	</div>
</div>

<style>
	.wrapper {
		position: absolute;
		top: 0;
		right: 0;
		z-index: 1;

		min-height: 35px;

		display: flex;
		gap: var(--space-sm);
		align-items: start;
		padding: var(--space-sm);
	}

	.wrapper .left-wrapper {
		display: flex;
		align-items: center;
	}
</style>
