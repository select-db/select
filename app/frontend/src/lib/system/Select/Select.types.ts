import type { Snippet } from 'svelte';

/** Option shape for Select. Value type is generic (default string). */
export type SelectOption<V = string> = {
	value: V;
	label: string;
};

/** Group of options for grouped Select. */
export type SelectOptionGroup<V = string> = {
	label: string;
	options: SelectOption<V>[];
};

/** Snippet that renders an option (or null for placeholder) in the trigger and in the menu. */
export type OptionDisplay<V = string> = Snippet<[SelectOption<V> | null]>;
export type SummaryDisplay<V = string[]> = Snippet<[SelectOption<V>[]]>;

/**
 * Select props. Single vs multi is determined by `multiple`:
 * - Single (default): `value` is V, `onchange` receives V.
 * - Multi: `multiple: true`, `value` is V[], `onchange` receives V[].
 */
export type SelectProps<V = string> = {
	/** Flat list of options. Ignored when optionGroups is provided. */
	options?: SelectOption<V>[];
	/** Grouped options (group label + options). When set, options is ignored. */
	optionGroups?: SelectOptionGroup<V>[];
	/** Single: one value. Multi: array of values. */
	value: V | V[];
	onchange?: (value: V | V[]) => void;
	/** Multi only: how to show selection in trigger. Default: "N selected". */
	optionDisplay?: OptionDisplay<V>;
	placeholder?: string;

	multiple?: boolean;
	summaryDisplay?: SummaryDisplay<V>;

	/** Enable search field in the underlying Menu. */
	searchEnabled?: boolean;
	searchPlaceholder?: string;

	/** When set with searchEnabled, show a "Create X" option when search doesn't match any option. */
	createOptionLabel?: (query: string) => string;
	/** Called when the user chooses the create option. Receives the search query. */
	onCreate?: (query: string) => void | Promise<void>;
	/**
	 * Optional validator for the create option.
	 * Return true to allow, or a string to show as an inline error instead of the create button.
	 */
	canCreate?: (query: string) => true | string;

	/** Visual emphasis of the trigger. High = default bordered, low = subtle/secondary. */
	emphasis?: 'high' | 'low';

	leftIcon?: import('$lib/system/Icon/types').Icons;
	iconSize?: number;
	iconColor?: string;
	isLoading?: boolean;
	loaderSize?: number;
	size?: 'xs' | 'sm';
	style?: string;

	/** Fixed pixel width for the trigger (and menu when menuWidth is not set). */
	width?: number;
	menuWidth?: number;

	noRadius?: boolean;

	/** Control the open state of the dropdown. */
	open?: boolean;
};

export function isMulti<V>(
	props: SelectProps<V>
): props is SelectProps<V> & { multiple: true; value: V[] } {
	return props.multiple === true;
}
