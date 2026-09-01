import { call, expect, test } from './wails';

/**
 * What the app does once it has a workspace to show. The state comes from
 * `internal/cmd/e2eseed`, which fills the run's data directory before the app
 * starts.
 */
test('shows the seeded workspace once signed in', async ({ page, signIn }) => {
	await page.goto('/');
	await signIn();

	await expect(page.getByText('queries')).toBeVisible();
});

test('reads a folder from disk the first time it is opened', async ({ page, request, signIn }) => {
	await page.goto('/');
	await signIn();
	await expect(page.getByText('queries')).toBeVisible();

	const queriesFolder = async () => {
		const workspace = await call<{
			folders: { folders: { name: string; resolved: boolean; files: { name: string }[] }[] }[];
		}>(request, 'selectDb/internal/graph.Graph.GetWorkspaceGraph');
		return workspace.folders[0].folders.find((folder) => folder.name === 'queries');
	};

	// The graph the app starts from carries the workspace's folders but not
	// their files: `queries` is there, what is inside it is not.
	const beforeOpening = await queriesFolder();
	expect(beforeOpening?.resolved).toBe(false);
	expect(beforeOpening?.files).toEqual([]);
	await expect(page.getByText('hello.sql')).toHaveCount(0);

	// Opening it asks the backend to read it, and the files arrive with the
	// graph update that follows.
	await page.getByText('queries').click();

	await expect(page.getByText('hello.sql')).toBeVisible();
	await expect(page.getByText('nested')).toBeVisible();

	const afterOpening = await queriesFolder();
	expect(afterOpening?.resolved).toBe(true);
	expect(afterOpening?.files.map((file) => file.name)).toEqual(['hello.sql']);
});
