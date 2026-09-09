<script lang="ts">
	import { type Component } from 'svelte';
	import { scale, fade } from 'svelte/transition';
	import { modalStore } from './ModalStore';
	import { setContext } from '$lib/stores/keybindingsContextStore';
	import { registerCommand, unregisterCommand } from '$lib/stores/commandRegistry';
	import { onMount, onDestroy } from 'svelte';

	let ContentComponent: Component | null = null;
	let ContentComponentProps: Record<string, unknown> = {};

	$: if ($modalStore?.content) {
		ContentComponent = $modalStore.content();
		ContentComponentProps = $modalStore.props || {};
	}

	$: setContext('modalOpen', $modalStore !== null);

	const cssSize = (v: number | string | undefined, fallback: string) =>
		v === undefined ? fallback : typeof v === 'number' ? `${v}px` : v;

	const onClose = () => {
		const cancelFn = $modalStore?.props?.onCancel as (() => void) | undefined;
		if (typeof cancelFn === 'function') {
			cancelFn();
		} else {
			modalStore.set(null);
		}
	};

	onMount(() => {
		registerCommand('modal.close', onClose);
	});

	onDestroy(() => {
		unregisterCommand('modal.close');
	});
</script>

{#if $modalStore}
	<div class="backdrop" role="presentation" onmousedown={onClose} out:fade={{ duration: 100 }}>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="modal-wrapper" onmousedown={(e) => e.stopPropagation()}>
			<div
				class="modal"
				role="presentation"
				style={`width: ${cssSize($modalStore.width, '300px')}; height: ${cssSize($modalStore.height, 'fit-content')};`}
				onmousedown={(e) => e.stopPropagation()}
				transition:scale={{ duration: 100, start: 0.9 }}
			>
				{#if ContentComponent}
					<svelte:component this={ContentComponent} {...ContentComponentProps} {onClose} />
				{/if}
			</div>
		</div>
	</div>
{/if}

<style>
	.backdrop {
		position: fixed;
		top: 0;
		left: 0;
		z-index: 5;

		width: 100%;
		height: 100%;

		display: flex;
		justify-content: center;
	}

	:global([data-theme='light']) .backdrop {
		background: linear-gradient(
			180deg,
			hsla(var(--hue), 12%, 94%, 0.2) 50%,
			hsla(var(--hue), 12%, 100%, 0.8) 90%
		);
	}

	:global([data-theme='dark']) .backdrop {
		background: linear-gradient(
			180deg,
			hsla(var(--hue), 12%, 8%, 0.3) 50%,
			hsla(var(--hue), 12%, 8%, 0.9) 90%
		);
	}

	.modal-wrapper {
		/* Safe click zone */
		position: absolute;
		top: 15vh;
		max-height: 70vh;
	}

	.modal {
		display: flex;
		flex-direction: column;
		box-shadow: var(--shadow) 0px 7px 29px 0px;
		margin: var(--space-md);

		border: var(--border);
		border-radius: var(--br-sm);
		overflow: hidden;
	}
</style>
