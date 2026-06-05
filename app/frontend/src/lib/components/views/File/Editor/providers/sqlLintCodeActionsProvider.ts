import * as monaco from 'monaco-editor';
import { LINT_SOURCE } from './sqlLintProvider';

/**
 * Provides quick-fix code actions for SQL lint markers.
 *
 * For each lint marker in the selection the user gets two actions:
 *   1. Disable for this line  → inserts "-- lint-disable-next-line <rule-id>" above
 *   2. Disable for entire file → inserts "-- lint-disable <rule-id>" at line 1
 *
 */
export function createSqlLintCodeActionsProvider(): monaco.languages.CodeActionProvider {
	return {
		provideCodeActions(model, _range, context): monaco.languages.CodeActionList {
			const actions: monaco.languages.CodeAction[] = [];

			const lintMarkers = context.markers.filter((m) => m.source === LINT_SOURCE && m.code);

			for (const marker of lintMarkers) {
				const ruleId =
					typeof marker.code === 'string' ? marker.code : (marker.code as { value: string }).value;
				if (!ruleId) continue;

				const targetLine = marker.startLineNumber;

				// disable for this line only
				actions.push({
					title: `Disable: ${ruleId} (this line)`,
					kind: 'quickfix',
					edit: {
						edits: [
							{
								resource: model.uri,
								textEdit: {
									range: {
										startLineNumber: targetLine,
										startColumn: 1,
										endLineNumber: targetLine,
										endColumn: 1
									},
									text: `-- lint-disable-next-line ${ruleId}\n`
								},
								versionId: model.getVersionId()
							}
						]
					}
				});

				// disable for the entire file.
				const alreadyHasFileDisable = actions.some(
					(a) => a.title === `Disable: ${ruleId} (entire file)`
				);
				if (!alreadyHasFileDisable) {
					actions.push({
						title: `Disable: ${ruleId} (entire file)`,
						kind: 'quickfix',
						edit: {
							edits: [
								{
									resource: model.uri,
									textEdit: {
										range: {
											startLineNumber: 1,
											startColumn: 1,
											endLineNumber: 1,
											endColumn: 1
										},
										text: `-- lint-disable ${ruleId}\n`
									},
									versionId: model.getVersionId()
								}
							]
						}
					});
				}
			}

			return { actions, dispose: () => {} };
		}
	};
}
