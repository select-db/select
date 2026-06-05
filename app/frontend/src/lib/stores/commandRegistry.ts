import { tryCatch } from '$lib/utils/tryCatch';

type CommandHandler = () => void | Promise<void>;

// Use globalThis to persist commands across HMR
const COMMANDS_KEY = '__selectdb_commands__';
function getCommandsMap(): Map<string, CommandHandler> {
	if (!(globalThis as Record<string, unknown>)[COMMANDS_KEY]) {
		(globalThis as Record<string, unknown>)[COMMANDS_KEY] = new Map<string, CommandHandler>();
	}
	return (globalThis as Record<string, unknown>)[COMMANDS_KEY] as Map<string, CommandHandler>;
}

export function registerCommand(id: string, handler: CommandHandler): void {
	getCommandsMap().set(id, handler);
}

export function unregisterCommand(id: string, handler?: CommandHandler): void {
	const map = getCommandsMap();
	// Only deletes the slot if the current handler is the one we registered.
	if (handler && map.get(id) !== handler) return;
	map.delete(id);
}

export function hasCommand(id: string): boolean {
	return getCommandsMap().has(id);
}

export async function executeCommand(id: string): Promise<boolean> {
	const commands = getCommandsMap();
	const handler = commands.get(id);
	if (!handler) return false;

	const [, err] = await tryCatch(async () => handler());
	if (err) {
		return false;
	}

	return true;
}

export function getRegisteredCommands(): string[] {
	return Array.from(getCommandsMap().keys());
}

import { registerWorkbenchCommands } from './workbenchCommands';
registerWorkbenchCommands();
