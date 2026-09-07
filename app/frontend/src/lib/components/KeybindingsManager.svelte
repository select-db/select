<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		chordFromEvent,
		findMatchingKeybinding,
		initKeybindings
	} from '$lib/stores/keybindingsStore';
	import { getContext } from '$lib/stores/keybindingsContextStore';
	import { executeCommand } from '$lib/stores/commandRegistry';

	let initialized = false;
	/**
	 * Whether the menu is being walked with the shortcut key held down.
	 *
	 * The picker works the way VS Code's does: hold the modifier, tap the key to
	 * step through, let go to take what is selected. Which modifier that is comes
	 * from the binding that was matched, not from the platform -- the keymap has
	 * already decided.
	 */
	let steppingThroughMenu = false;

	function handleKeydown(e: KeyboardEvent) {
		if (!initialized) return;

		const chord = chordFromEvent(e);
		if (!chord) return;

		const context = getContext();

		// Escape closes the menu whatever is held with it.
		if (e.key === 'Escape' && context.menuFocus) {
			e.preventDefault();
			e.stopPropagation();
			executeCommand('menu.close');
			steppingThroughMenu = false;
			return;
		}

		const keybinding = findMatchingKeybinding(chord, context);
		if (!keybinding) return;

		if (context.menuFocus && keybinding.command === 'menu.selectNext' && (e.metaKey || e.ctrlKey)) {
			steppingThroughMenu = true;
		}

		// A binding with no command is an unbinding: the app stands aside and the
		// keystroke goes on to whoever else wants it -- the editor, the browser,
		// the window manager.
		if (!keybinding.command) return;

		e.preventDefault();
		e.stopPropagation();
		executeCommand(keybinding.command);
	}

	function handleKeyup(e: KeyboardEvent) {
		if (!initialized) return;

		// Letting the modifier go takes what the menu has landed on.
		if ((e.key === 'Meta' || e.key === 'Control') && steppingThroughMenu) {
			const context = getContext();
			if (context.menuFocus) executeCommand('menu.confirm');
			steppingThroughMenu = false;
		}
	}

	onMount(async () => {
		await initKeybindings();
		initialized = true;

		window.addEventListener('keydown', handleKeydown, true);
		window.addEventListener('keyup', handleKeyup, true);
	});

	onDestroy(() => {
		window.removeEventListener('keydown', handleKeydown, true);
		window.removeEventListener('keyup', handleKeyup, true);
	});
</script>
