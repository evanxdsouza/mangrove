# Working in this repo

Before exploring the codebase from scratch, read
[docs/REPO_MAP.md](docs/REPO_MAP.md) first. It's a maintained index of what
lives where, how each component builds/runs/tests, and pointers to the
deeper `docs/*.md` files for the "why." Use it to orient, then read the
specific files/docs it points at rather than re-discovering structure that's
already mapped.

When a change adds, removes, or moves something the map describes —
a directory's purpose, a build/test/run command, a "where to look for X"
pointer, or the verified-status snapshot at the bottom — update
`docs/REPO_MAP.md` in the same change. If you re-verify something in the
"Verified status" section (build, tests, lint, e2e), refresh its result and
the "Last verified" date rather than leaving a stale snapshot. If you're
unsure whether an edit is map-worthy, prefer updating it over leaving it
stale — the map is only useful if it stays accurate.

User-facing docs live in `docs/` by section (`start/`, `deploy/`, `data/`,
`run/`, `account/`, `teams/`, `operate/`, `interfaces/`), indexed by
[docs/README.md](docs/README.md). When a change alters something a user sees
or configures, update the matching page in the same change. The
design-level docs at the top of `docs/` (architecture.md, deployment.md,
etc.) are linked from code comments by path -- don't move or rename them.
