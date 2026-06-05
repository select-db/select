/**
 * Platform detection for layout and behavior.
 * Single source of truth so we don't duplicate navigator checks.
 */

const isMac =
	typeof navigator !== 'undefined' &&
	/Mac|iPod|iPhone|iPad/.test(navigator.platform);

/** True when the app uses native window controls (e.g. macOS traffic lights). Hide custom titlebar buttons when true. */
export const useNativeWindowControls = isMac;

export { isMac };
