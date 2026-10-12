# Audit logs

Mangrove records who deployed, deleted or changed access to what, and when.

## Where to see it

- **Workspaces → Activity**: everything that happened in one workspace.
  Anyone with access to the workspace can see it, viewers included.
- **Admin → Audit log** (owners): everything, across every workspace, plus
  account-level events like user changes.

## What's recorded

| Kind | Events |
|---|---|
| **Deploys** | Manual deploys, redeploys, rollbacks, scaling, promotes, and pushes and PR previews from GitHub (recorded as `github-webhook`) |
| **Deletes** | Projects, deployments, workspaces, custom domains, accounts |
| **Access changes** | Access control (public, internal, password; never the password itself), secret variables (which key, never the value), adding a domain, workspace membership, moving a project, creating an account, revoking a session |

Each entry has the time, who did it (their email, kept even if the account
is later deleted), the action, what it acted on, and a short detail.

## Good to know

- **Entries are never pruned.** An audit trail that expires isn't one.
- **Automatic actions** (self-healing retries, idle sleep, preview cleanup)
  aren't audited as user actions. They show in each deployment's
  **History** tab and Mangrove's own logs instead.
- **GitHub token changes** aren't recorded, because tokens don't belong to
  any one workspace.
- There's no export. Owners can read it from the API
  (`GET /api/admin/audit-log`).
