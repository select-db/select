import type { Icons } from '../Icon/types';

type Position = { x: number; y: number };

export type ContextMenuOption = {
	label: string;
	icon?: Icons;
	action?: (onClose: () => void, metadata: ContextMenu['metadata']) => void;
	submenu?: ContextMenuOption[];
	divider?: boolean;
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
