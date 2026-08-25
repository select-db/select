import type { Icons } from '../Icon/types';

type Position = { x: number; y: number };

export type ContextMenuOption = {
	label: string;
	icon?: Icons;
	action?: (onClose: () => void, metadata: ContextMenu['metadata']) => void;
	submenu?: ContextMenuOption[];
	divider?: boolean;
	/**
	 * Run this action when the item is double-clicked. At most one option per
	 * menu carries it, and an item without one simply has no double-click.
	 */
	runOnDoubleClick?: boolean;
};

export type ContextMenu = {
	isOpen: boolean;
	position: Position;
	options: ContextMenuOption[];
	depth: number;
	selectedIndex: number;
	parentMenu: ContextMenu | null;
	direction: 'right' | 'left';
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	metadata?: any;
};
