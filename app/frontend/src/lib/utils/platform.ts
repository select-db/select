/**
 * Platform detection for layout and behavior.
 * Single source of truth so we don't duplicate navigator checks.
 */

const isMac = typeof navigator !== 'undefined' && /Mac|iPod|iPhone|iPad/.test(navigator.platform);

export { isMac };
