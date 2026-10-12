# Workspaces and roles

Access in Mangrove works on two levels: what someone can do to the **box**,
and what they can do inside each **workspace**.

## Box roles: owner and member

Every account is one of:

- **Owner**: runs the box. Manages accounts, sessions, ports, image
  pruning, backups and drive storage, and is an admin in every workspace.
- **Member**: everyone else. Sees only the workspaces they've been added
  to, with the role they were given there.

The first account is always an owner. There can be several owners.

### Add someone

**Admin → Team** (owners): enter their email, an initial password and a role.
Share the password with them yourself; Mangrove sends no invite emails.
They can change it in [Settings](../account/profile.md#change-password).

New accounts join the default workspace as an **editor**. Add them to other
workspaces from there.

### Remove someone

**Admin → Team → delete**. You can't delete yourself, and you can't delete
the last owner, so the box always has someone who can manage it.

## Workspaces

A workspace is a group of projects, typically one per team, client or
purpose. Every install has a default workspace that can't be deleted.

- **Workspaces → New workspace** to create one. You become its admin.
- The **station switcher** at the top of the sidebar picks which workspace
  the Projects page shows, or all of them.
- **Move to workspace** on a project moves it. You need to be an admin in
  both workspaces.
- Deleting a workspace moves its projects back to the default workspace.
  Nothing is deleted with it.

## Workspace roles

| | Viewer | Editor | Admin |
|---|:-:|:-:|:-:|
| See projects, deployments, logs, history, plain variables, domains | ✓ | ✓ | ✓ |
| Deploy, redeploy, roll back, scale, stop, restart | | ✓ | ✓ |
| Edit plain variables, install templates, connect a repo | | ✓ | ✓ |
| Create staging and PR previews, verify a domain | | ✓ | ✓ |
| Run a command inside a container | | ✓ | ✓ |
| Set secret variables | | | ✓ |
| Change access control, add or remove domains | | | ✓ |
| Delete projects and deployments | | | ✓ |
| Manage the workspace's members, delete the workspace | | | ✓ |
| Move a project in or out (admin in both workspaces) | | | ✓ |

A member with no role in a workspace can't see it at all. Asking for
something in a workspace you can't access returns "not found", not
"forbidden", so its existence isn't revealed.

Owners pass every workspace check.

## Manage members

**Workspaces → Members** (workspace admins):

- **Add an existing account by email**, with a role. The account must
  already exist; an owner creates it first under **Admin → Team**.
- Change someone's role, or remove them. Removing someone only removes
  their access to that workspace.

## Good to know

- **GitHub tokens are shared across the install**, not per workspace. Any
  signed-in user can list and use them when linking a repo.
- The dashboard hides buttons you can't use, but the API enforces every
  rule itself.
- Every delete, access change and deploy is recorded. See
  [Audit logs](audit-logs.md).

How roles are enforced: [multi-user.md](../multi-user.md).
