# The Workspace Folder

A workspace is a folder on your machine. Any folder: a repository you cloned, a
directory of `.sql` files you have had for years, or an empty one you just made.
SELECT opens it the way an editor does, and reads what is already in it.

There is no managed location and nothing hidden in your application data
directory. Your files stay where you put them.

## Opening a folder

Signing in with no folder open leaves you in the app itself, with the folders
you opened recently where your files would be. Open one from there, or with the
button in the top left corner -- `Cmd+O`.

![The app with no folder open: recent folders where the files would be, and the workspace button in the top left corner.](/shots/folder.start.light.webp)

What happens next depends on what is in the folder.

**Already a workspace.** It holds a `select.config.json`, so SELECT connects to
the right server and builds the tree.

**Not a workspace yet.** SELECT offers to make one, named after the folder.
Creating it adds that one file and changes nothing else; an empty folder also
gets a small sample database to start from.

![Opening a folder that is not a workspace yet: its path, the workspace name filled in from the folder, and what creating one will add.](/shots/folder.setup.light.webp)

**On another server.** Roles and permissions come from the server a workspace
lives on, so sign out, pick that server, and open the folder again.

**Not yours to open.** The workspace was deleted, or you are not in it. Ask
someone in the workspace to invite you. `select.config.json` is left alone,
because your team probably shares it -- starting a new workspace in the folder
is a separate, deliberate click.

## `select.config.json`

This is the file that makes a folder a workspace. It sits at the root and holds
two things:

```json select.config.json
{
  "$comment": "Managed by SELECT. Do not edit by hand.",
  "version": 1,
  "server": "app.select-db.com",
  "workspaceId": "ws_01hq..."
}
```

Do not edit it by hand. SELECT writes it when you create a workspace and reads
it every time you open the folder.

> [!TIP]
> **Commit it.** That is the point of the file. A teammate clones the
> repository, opens the folder, and lands in the right workspace on the right
> server without being told which.

The workspace's **name** is deliberately not in it. The name lives on the
server, shared with everyone in the workspace, so renaming it renames it for the
team. What you call the directory on your own disk is your business.

## Sharing a workspace with your team

1. One person creates the workspace in a folder and commits
   `select.config.json` along with their queries.
2. Everyone else clones the repository and opens the folder.
3. The owner invites them under **Settings -> Users**.

Git moves the files; the SELECT backend moves the team settings. See
[Git](/docs/workspace/git/) for what belongs in each.

## Large folders

SELECT reads your `.gitignore` and skips the folders it excludes when building
the tree and watching for changes, so opening a repository with a `node_modules`
or a `target` in it stays fast. Only folders are skipped, never files: `.env` is
gitignored by default and SELECT still reads your `$VARIABLES` from it. See
[.gitignore](/docs/workspace-files/gitignore/).

## Deleting a workspace

Deleting a workspace removes it from the server and deletes
`select.config.json` from the folder. **Your files are not touched.** The
folder, the queries and the databases in it stay exactly as they were, and you
can create a new workspace in the same folder afterwards.

Only the owner can delete a workspace. On their machine the folder closes and
`select.config.json` goes with it; everyone else finds out on their next sync,
where the folder closes and the config stays. Your files are still there either
way, and the folder then opens on the screen above.

Losing access to a workspace someone else deleted, or being removed from one,
looks the same from your side: the folder closes and you are left on the start
screen, still signed in.

## Folders SELECT will not open

Your home directory and a filesystem root are refused. Both mean indexing
everything you own, and both are mis-clicks rather than intentions. Pick a
folder inside them instead.
