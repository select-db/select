import type { Snippet } from 'svelte';
import type { Icons } from '$lib/system/Icon/types';

export type MenuOption<T = string> = {
	id: T;
	label: string;
	icon?: Icons;
	iconSize?: number;
	disabled?: boolean;
	badge?: string | number;
	metadata?: string;
	type?: 'option' | 'group-label' | 'divider';
	action?: (option: MenuOption<T>) => void | Promise<void>;
	/** Marks this option as selected (used for sorting and checkmark display). */
	selected?: boolean;
	/** When true, show a checkbox; row click and checkbox click are handled separately. */
	checkbox?: boolean;
	/** Checked state for the checkbox (when checkbox is true). */
	checked?: boolean;
	/** Called when the checkbox is clicked (not the row). Toggle selection. */
	onCheckClick?: (option: MenuOption<T>) => void;
	/** Optional data for custom option content; when set with optionContent prop, Menu renders the snippet instead of label. */
	customData?: unknown;
};

export type MenuOptionContentSnippet = Snippet<[unknown]>;

export type MenuProps<T = string> = {
	// Options (flat array - group labels are special options with type: 'group-label')
	options?: MenuOption<T>[];

	/** When true, sort options alphabetically (A -> Z). Defaults to false. */
	sortOptions?: boolean;

	// Search
	searchEnabled?: boolean;
	searchPlaceholder?: string;
	searchQuery?: string;
	/** When true, Menu won't filter options internally, only react to options. */
	externalFilter?: boolean;
	/** Validator for the search input. Returns an error message if invalid, or null if valid. */
	searchValidator?: (value: string) => string | null | undefined;

	// Loading state
	loading?: boolean;

	// Empty states
	emptyMessage?: string;
	noResultsMessage?: string;

	// Callbacks
	onClose?: () => void;

	// Styling
	maxHeight?: number;
	minHeight?: number;
	width?: number | string;
	style?: string;
	highlightActiveOption?: boolean;

	/** When provided, options with customData use this snippet for content instead of label. */
	optionContent?: MenuOptionContentSnippet;

	/** When true, no border is shown. Defaults to false. */
	noBorder?: boolean;
};
