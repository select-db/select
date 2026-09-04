import type { APIRequestContext } from '@playwright/test';
import { call, expect, holdSession, test as base } from './wails';
import { editor, tab, testId } from './selectors';

const GRAPH = 'selectDb/internal/graph.Graph';
const FS = 'selectDb/internal/fs_provider.FSProvider';

type Folder = { uri: string; resolved: boolean; files: { name: string }[] };

const folderNode = (request: APIRequestContext, uri: string) =>
	call<Folder | null>(request, `${GRAPH}.GetFolderNodeByID`, uri);

/** Writes a file the way anything outside the app writes one. */
const writeFile = (request: APIRequestContext, uri: string) =>
	call(request, `${FS}.Write`, { uri, content: '' });

/**
 * `folder(path)` creates a directory in the workspace and hands back its URI.
 *
 * The seeded workspace is one directory shared by every spec in the run and
 * photographed by the screenshot suite, so whatever a spec adds to it is
 * removed again once the spec ends.
 */
const test = base.extend<{ folder: (path: string) => Promise<string> }>({
	folder: async ({ request }, use) => {
		const workspace = await call<{ folders: { uri: string }[] }>(
			request,
			`${GRAPH}.GetWorkspaceGraph`
		);
		const root = workspace.folders[0].uri;
		const created: string[] = [];

		await use(async (path) => {
			const uri = `${root}/${path}`;
			created.unshift(uri);
			await call(request, `${FS}.Mkdir`, { uri });

			// Wait for the graph to be holding it before anything is written
			// inside: a file whose parent the graph does not know is taken on
			// trust and inserted, which is the state these specs exist to tell
			// apart from a folder nobody has opened.
			await expect.poll(() => folderNode(request, uri).then((f) => f?.resolved)).toBe(false);
			return uri;
		});

		for (const uri of created) await call(request, `${FS}.Delete`, { uri, recursive: true });
	}
});

/**
 * What the app does once it has a workspace to show. The state comes from
 * `internal/cmd/e2eseed`, which fills the run's data directory before the app
 * starts.
 */
test('shows the seeded workspace once signed in', async ({ page, signIn }) => {
	await page.goto('/');
	await signIn();

	await expect(page.getByText('weekly_revenue.sql')).toBeVisible();
});

/**
 * A query naming a variable nothing resolves stops for it, and the value typed
 * in reaches the database.
 *
 * The numeric type is the case worth a test: an input of type number binds a
 * number back, and the form used to call .trim() on it. That threw inside the
 * derivation behind the Run button, so an integer, decimal, date, timestamp or
 * time variable could be typed and never run.
 */
test('runs a query with a numeric runtime variable', async ({ page, signIn }) => {
	// Longer than half a second, which is when the app would otherwise decide it
	// has no token and drop back to the login screen.
	await holdSession(page);
	await page.goto('/');
	await signIn();

	await page.keyboard.press('ControlOrMeta+n');
	await expect(editor.surface(page)).toBeVisible();
	await editor.surface(page).click();
	await page.keyboard.type('SELECT id FROM orders WHERE total_cents >= $MIN_CENTS;');

	// A scratch tab starts attached to nothing, and the run is refused before it
	// ever looks for variables.
	await page.keyboard.press('ControlOrMeta+Shift+d');
	const picker = page.getByPlaceholder('Search db...');
	await expect(picker).toBeVisible();
	await page.getByText('warehouse', { exact: true }).last().click();
	await page.keyboard.press('Escape');
	await expect(picker).toBeHidden();

	await editor.surface(page).click();
	await page.keyboard.press('ControlOrMeta+Enter');

	const form = page.locator('form');
	await expect(page.getByText('Set variables')).toBeVisible();
	await form.locator('.select-container').first().click();
	await page.getByText('Integer', { exact: true }).last().click();
	await form.locator('input').click();
	await page.keyboard.type('5000');

	await form.getByRole('button', { name: 'Run' }).click();
	await expect(testId(page, 'segmented.option', 'results')).toHaveText(/[1-9]/, {
		timeout: 20_000
	});
});

test('reads a folder from disk the first time it is opened', async ({
	page,
	request,
	folder,
	signIn
}) => {
	const queries = await folder('queries');
	await writeFile(request, `${queries}/hello.sql`);

	await holdSession(page);
	await page.goto('/');
	await signIn();
	await expect(testId(page, 'tree.node', 'queries')).toBeVisible();

	// The graph the app starts from carries the workspace's folders but not
	// their files: `queries` is there, what is inside it is not.
	expect(await folderNode(request, queries)).toMatchObject({ resolved: false, files: [] });
	await expect(page.getByText('hello.sql')).toHaveCount(0);

	// Opening it asks the backend to read it, and the files arrive with the
	// graph update that follows.
	await testId(page, 'tree.node', 'queries').click();
	await expect(page.getByText('hello.sql')).toBeVisible();

	const opened = await folderNode(request, queries);
	expect(opened?.resolved).toBe(true);
	expect(opened?.files.map((file) => file.name)).toEqual(['hello.sql']);
});

test('finds a file in a folder that has never been opened', async ({
	page,
	request,
	folder,
	signIn
}) => {
	await folder('archive');
	const nested = await folder('archive/2025');
	await writeFile(request, `${nested}/b.sql`);

	await holdSession(page);
	await page.goto('/');
	await signIn();
	await expect(testId(page, 'tree.node', 'archive')).toBeVisible();

	// `archive/2025/b.sql` is two folders deep and nothing has opened either of
	// them, so it is not in the graph the app is holding — the picker asks the
	// backend for it.
	await page.getByRole('button', { name: 'Search workspace' }).click();
	await page.getByPlaceholder('Search workspace...').fill('b.sql');

	const result = page.getByRole('menuitem').filter({ hasText: 'b.sql' });
	await expect(result).toBeVisible();

	await result.click();
	await expect(tab(page, 'b.sql')).toBeVisible();
});
