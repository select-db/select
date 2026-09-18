# Test selectors

How to address the UI from a test, so a rename in CSS or a refactor in a
component does not quietly break the suite.

The rule in one line: **address what the user perceives; add a hook only where
they perceive nothing.**

## Tier 1: role and accessible name

Anything a person can see and name is addressed by what it is and what it says:

```ts
page.getByRole('button', { name: 'Open Chat' });
page.getByRole('menuitem', { name: 'Views' });
page.getByRole('textbox', { name: 'Message' });
```

This is the default. It survives restyling, it breaks when the thing a user
relies on actually changes, and it fails when the accessible name is missing.
That is a real defect, not a test problem.

**Do not use `aria-label` as a test hook.** It is read aloud to people using a
screen reader. Writing `aria-label="chat-input-1"` to please a test makes the
product worse, and pinning a test to a label means an a11y wording improvement
shows up as a test failure. Give a control the name a user would say, then
address it by that name.

Related: our `Button` takes `label` for its *tooltip* and a separate
`ariaLabel`. A button with only `label` has no accessible name, so it is
unreachable to a screen reader and to `getByRole`. Set `ariaLabel` on icon-only
buttons; it fixes both at once.

## Tier 2: `data-test`, for structure with no name

Panels, regions, rows, handles: things a user sees but would not name. Inventing
an aria-label for them pollutes the accessibility tree with furniture.

```html
<div class="wrapper tab-actions" data-test="tabs.actions">
<div class="resizer" data-test="split.resizer">
<p class="child-vis-badge" data-test="tree.visibility-badge">
```

**Grammar:** `data-test="<area>.<element>"`, lowercase and dot-separated, area
first. One level of nesting; if you need two, the area is too broad.

**Identity:** when a hook repeats, carry the instance in a second attribute
rather than encoding it in the name:

```html
<li data-test="tree.node" data-test-value="orders">
```

```ts
testId('tree.node', 'orders');   // [data-test="tree.node"][data-test-value="orders"]
```

That is what removes the `.first()` / `.last()` / `{ exact: true }` noise from
the specs. Those are a symptom of having no stable hook, not a style.

## Never address a styling class

`.tab-actions`, `.input-row`, `.child-vis-badge` are CSS. Renaming a class is a
CSS decision and must stay one. Every class in a selector today is a bug
waiting for someone to tidy a stylesheet.

## Third-party DOM is quarantined

Monaco's `.view-lines`, `.suggest-widget`, `.monaco-list-row` are not ours to
name, and they change when Monaco is upgraded. Two rules:

1. Wrap it once in something that is ours: `data-test="editor.surface"`.
2. Reach inside only through a named helper in `selectors.ts`, so an upgrade is
   one file to fix rather than a search across every spec.

```ts
export const completionItem = (page: Page, column: string) =>
  page.locator('.suggest-widget .monaco-list-row', { hasText: column });
```

## Content stays content

`getByText('weekly_revenue.sql')`, `getByText('2026-03-23')` are fixture data
the user reads. Keep them as text: asserting on what is on screen is the point,
and the seed already fixes those strings.

## They ship

`data-test` stays in production builds. This is a desktop app, the bytes are
irrelevant next to the binary, and keeping them means the suite can drive a
shipped build rather than only a test one.

## Adding a hook

1. Can a user name it? Give it an accessible name and use Tier 1.
2. Otherwise add `data-test="<area>.<element>"` next to the class, never
   instead of it.
3. Repeating? Add `data-test-value`.
4. Reaching into a library's DOM? Put it in `selectors.ts`.

## Where a spec goes

Beside the component it drives, named for it:

```
src/lib/components/views/Chat/chat.spec.ts
src/lib/components/Layout/Tabs/tabs.spec.ts
```

So what covers a component is visible from its directory rather than from a
list somewhere else, and a component that moves takes its spec with it. The
`*.shot.ts` screenshots already sit this way, beside the content they show.

`tests/e2e/` keeps what is shared between specs, plus `bundle.spec.ts`, which
is about the build rather than any one component.

Specs under `src/` are the only files there allowed to import from
`tests/e2e/`; eslint enforces the rest, so a stray import cannot pull
`@playwright/test` into the shipped bundle.

## Where a helper goes

Four files, and the question that puts a helper in one of them. A spec that
writes its own copy of one of these gets a gesture with the same name and
subtly different behaviour, which is how `renameTo` came to assert the box had
closed in one spec and not in another.

| Question | File | Examples |
| -------- | ---- | -------- |
| How do I address this? | `selectors.ts` | `treeRow`, `tab`, `renameBox` |
| What do I do to the tree? | `tree.ts` | `openRootMenu`, `choose`, `renameTo` |
| What do I ask the Go side? | `wails.ts` | `call`, `exec`, `onDisk` |
| What will the model answer? | `aiProvider.ts` | `modelWillReply`, `say` |

Anything used by one spec stays in that spec. Move it here on the second copy,
not in anticipation of one.
