export function highlightCommand(command: string, args: string[]): string {
	return [command, ...args].join(' ');
}
