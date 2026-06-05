/**
 * Registry for tool calls that have an open diff UI (e.g. edit_file).
 * DiffView reads from this to wire Allow/Deny buttons back to the chat engine.
 */

export type DiffApprovalHandlers = {
	approve: () => Promise<void>;
	deny: () => Promise<void>;
};

const registry = new Map<string, DiffApprovalHandlers>();

export function registerDiffApproval(toolCallId: string, handlers: DiffApprovalHandlers): void {
	registry.set(toolCallId, handlers);
}

export function getDiffApproval(toolCallId: string): DiffApprovalHandlers | null {
	return registry.get(toolCallId) ?? null;
}

export function clearDiffApproval(toolCallId: string): void {
	registry.delete(toolCallId);
}
