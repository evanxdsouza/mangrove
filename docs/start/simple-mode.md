# Simple mode

Simple mode is a second face for the dashboard, for people who want "a
blog" or "a password manager" running and don't want to think about
Dockerfiles. Switch with the **Technical / Simple** toggle in the sidebar.
The choice is remembered in that browser.

It uses the same backend and the same data. Switching modes never changes
anything; it only changes what you see.

## What's different

| Technical mode | Simple mode |
|---|---|
| Projects containing deployments | One flat **Your apps** list |
| New project, then add deployments | **Add an app**: pick one, done |
| Raw statuses (`building`, `failed`) | "Starting up", "Running", "Turned off", "Having a problem" |
| Every setting, env var and resource number | Just the app, its address and its state |
| Full action set | **Try again** (after a failure) and **Remove** |

**Add an app** offers the user-facing templates under plain names: blog,
website builder, password manager, website monitor, automation, visitor
stats, spreadsheet database, app backend, Git hosting and app platform
(`SIMPLE_TEMPLATES` in `web/src/lib/plainCopy.ts`). Standalone
databases are left out on purpose; they're installed alongside the apps
that need them. Picking an app creates a project for it behind the scenes.

**Admin** is hidden in Simple mode. It's still at `/admin`, or switch back
to technical mode.

## Escape hatch

Every app's page has **Switch to advanced view**, which flips back to
technical mode on the same page. Nothing about the app changes.

## Known gap

Removing an app in Simple mode deletes the deployment but leaves its
behind-the-scenes project. It's invisible in Simple mode and shows up as an
empty project in technical mode. Delete it there if it bothers you.

More in [modes.md](../modes.md).
