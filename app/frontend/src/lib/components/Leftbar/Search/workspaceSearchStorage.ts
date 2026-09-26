const STORAGE_KEY = 'selectdb.workspaceSearch.v1';

export type WorkspaceSearchPersisted = {
	query: string;
	datasourceOn: Record<string, boolean>;
	schemaOn: Record<string, boolean>;
};

const EMPTY: WorkspaceSearchPersisted = { query: '', datasourceOn: {}, schemaOn: {} };

export function readWorkspaceSearch(): WorkspaceSearchPersisted {
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return EMPTY;
		const p = JSON.parse(raw);
		return {
			query: typeof p.query === 'string' ? p.query : '',
			datasourceOn: p.datasourceOn && typeof p.datasourceOn === 'object' ? p.datasourceOn : {},
			schemaOn: p.schemaOn && typeof p.schemaOn === 'object' ? p.schemaOn : {}
		};
	} catch {
		return EMPTY;
	}
}

export function writeWorkspaceSearch(data: WorkspaceSearchPersisted): void {
	try {
		localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
	} catch {
		// quota / private mode
	}
}
