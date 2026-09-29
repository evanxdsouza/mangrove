---
target: web/src/pages/ProjectsPage.tsx
total_score: 29
max_score: 40
na_heuristics: 
p0_count: 1
p1_count: 1
timestamp: 2026-09-29T07-54-06Z
slug: web-src-pages-projectspage-tsx
---
## Design Health Score

| # | Heuristic | Score | Key Issue |
|---|-----------|-------|-----------|
| 1 | Visibility of System Status | 3 | `PENDING` shows no ETA/progress/log link; a stalled build looks identical to a queued one |
| 2 | Match Between System / Real World | 4 | Vocabulary (workspace, rollup "2 of 3 running") mirrors the operator's own mental model |
| 3 | User Control and Freedom | 3 | Search has no clear/"×" button; otherwise fine |
| 4 | Consistency and Standards | 3 | One project card exposes 4 overlapping click targets, two going to the same destination |
| 5 | Error Prevention | 3 | No destructive actions here, but the workspace-scope link sits flush in the click-through footer |
| 6 | Recognition Rather Than Recall | 4 | Rollup line + pill + glyph put the decision-relevant info on the card itself |
| 7 | Flexibility and Efficiency | 2 | Only lever is a name/slug substring filter — no status filter, sort, bulk action, shortcut |
| 8 | Aesthetic and Minimalist Design | 3 | Clean cards, but `minmax(260px,1fr)` leaves the populated state looking sparse |
| 9 | Error Recovery | 2 | A card stuck at `PENDING` gives zero diagnosis without drilling into deployment detail |
| 10 | Help and Documentation | 2 | No inline explanation of status meaning for a first-timer |
| **Total** | | **29/40** | **Good** |

## Design Specificity Verdict

**LLM assessment**: Genuinely grounded, not a re-skinned generic dashboard — `PlantGlyph` and `StatusPill` share one classification map (a structural guarantee, not a coincidence), `RecentActivityPanel` is built entirely from already-fetched data with an honest empty state, and `.project-card-children` correctly renders as a recessed inset rather than a nested card, executing DESIGN.md's own rule. Copy and vocabulary are Mangrove-specific throughout. The gap: in the seeded live instance every resource sat at `PENDING`, so the "living resource" differentiator (a card that visibly looks alive vs. sick) never actually surfaced — the bones are bespoke, the demoed state doesn't show them off.

**Deterministic scan**: `detect.mjs` static source scan across the 5 changed/related files: **clean, 0 findings** (exit 0). The live-injected browser detector (a separate, DOM-aware pass) found real issues layered on top — see Priority Issues below. Two `text-occlusion` findings on the expanded/detail views are very likely the detector's own on-page overlay labels stacking on each other near the sidebar brand mark, not a real product-UI defect — flagging as a probable false positive rather than folding it into the count.

## Overall Impression

The underlying system is honest and well-structured — real data, no fabricated feeds, correct component reuse. The biggest opportunity isn't structural, it's two things: a real, systemic text-contrast bug that undermines exactly the fields an operator reads to identify a resource, and a "no dial to turn" flatness once you have more than a couple of projects (no filter/sort, cards small enough that a populated list still reads empty).

## What's Working

1. **`PlantGlyph`/`StatusPill` share one classification map** — the glyph and pill are structurally guaranteed to agree, never just visually coordinated by convention.
2. **`.project-card-children` as a recessed `bg-inset` block**, not a second bordered card — correctly avoids the "card-in-a-card" anti-pattern DESIGN.md explicitly bans.
3. **`RecentActivityPanel` built from data the page already fetched**, with an honest "nothing deployed yet" empty state instead of a fabricated feed — matches the product's own "real infrastructure, not a mock" principle.

## Priority Issues

**[P0] Systemic text-contrast failure on identifying content.** `text-faint` (`#6f6250`) measures 2.6–3.3:1 against every card/panel background it's used on (`bg-card`, `bg-inset`, `bg`, `bg-elevated`) — well under the WCAG AA 4.5:1 floor DESIGN.md itself commits to. It's applied to exactly the fields an operator uses to identify a resource: project slug, created/last-deployed timestamps, deployment build-strategy tag, activity-panel meta line — not decorative filler. This is a design-token-level issue, not a one-off: 22 call sites across the app use `text-faint`, some legitimately (true placeholder/disabled text), some for real identifying data that should never have been set this low. **Fix**: promote the identifying-data call sites (slug, timestamps, build-strategy, activity meta) to `text-dim` (`#a99b83`, ~5.7:1 — passes AA), leave genuinely decorative/disabled uses on `text-faint`. Suggested command: `/impeccable audit` → `/impeccable harden` (contrast is a hard a11y floor).

**[P1] No filter/sort beyond a name substring search.** An operator with 20+ projects can't isolate "what needs my attention" even though the worst-status-per-project is already computed client-side. **Fix**: status filter chips (All / Running / Attention) beside `.search-field`, from data already loaded — no new endpoint. Suggested command: `/impeccable layout` or fold into the redo pass.

**[P2] Redundant/overlapping click targets on one card.** `.project-card` (whole-card nav), `.project-card-name` (same destination), `.project-card-workspace` (a *different* action — changes active workspace scope), and `.project-card-chevron-btn` (local expand toggle) all live in one 260px card. The inner name-link duplicates the whole-card click; the workspace switcher sits flush in the click-through footer, inviting an accidental scope change while reaching for something else. **Fix**: drop the redundant inner name-link (keep it as a link for a11y/right-click, but it needn't visually compete), and give the workspace switcher more visual separation (e.g. its own small pill) so it doesn't read as passive metadata.

**[P2] Sparse desktop composition at realistic project counts.** At 1440px with 3 projects, `minmax(260px,1fr)` keeps cards small — the "populated" state still photographs ~60% empty. **Fix**: raise the minimum card width (e.g. 300–320px) and/or let a card show a touch more density (e.g. a mini sparkline or the plant glyph at a slightly larger size) so 3–6 projects reads as a designed page, not a sparse grid waiting to fill up.

**[P2] Mobile truncation on expanded child rows.** At 390px, `.project-child-name` truncates to 3–4 characters ("cac…", "wor…") because fixed-width siblings (build-strategy tag, status pill, timestamp) crowd out the one column that identifies the row. **Fix**: stack the meta row below the name on narrow viewports instead of forcing everything onto one flex line.

**[P3] `pending` and `stopped` are visually identical.** `pending` maps to the same bare-stem *dormant* glyph pose and gray pill as an intentionally `stopped` deployment, despite being in `IN_PROGRESS_STATUSES` — the only differentiator is a subtle beacon pulse. A queued build reads as "off," not "starting." Suggested command: fold into the redo pass (`PlantGlyph`'s pose map).

## Persona Red Flags

**Alex (power-user)**: No status filter/sort/bulk action/keyboard shortcut — must eyeball every card on every visit even though "worst status" is already computed with no extra round-trip. The 4s auto-poll never highlights what changed since the last look.

**Sam (accessibility-dependent)**: Slug/timestamp/build-strategy metadata at ~2.6:1 is unreadable for low-vision users on exactly the fields used to identify a resource — this is the P0 above, concretized. Tab elements (both the pre-existing `DeploymentDetailPage` tabs and the new `ProjectDetailPage` ones) are `<div onClick>`, not native buttons — no keyboard focus/activation without additional handling (pre-existing pattern, not introduced this pass, but real).

**Jordan (first-timer)**: Creating your very first project hard-redirects to `/projects/:id` with zero acknowledgment (the milestone `Toast` is reserved for first-deploy/first-rollback only) — the actual first milestone passes flat. Three cards sitting at `PENDING` for minutes with no "this can take a minute" copy anywhere leaves a nervous new self-hoster with no signal of normal-vs-broken.

## Minor Observations

- Zero-result search copy ("No projects match…") is plain/clinical while the empty-workspace copy is swamp-voice — a tonal seam between two adjacent empty states on the same page.
- `.project-card-workspace` correctly follows the Two-Accent Rule on hover (brass-bright), but nothing hints it's an active scope-switcher rather than passive metadata until you hover it.
- `.activity-panel` at 280px, in the all-empty-state, is itself mostly whitespace — compounds the sparse-canvas issue above.

## Questions to Consider

1. If `pending` is both an honest first-run state and a stuck build, should the UI ever distinguish "just created" from "stuck 20 minutes," or is silence the more honest answer given Mangrove's own "real infrastructure, not a mock" principle?
2. Is the Recent Activity rail earning its permanent 280px on every visit, or should it collapse once a workspace's own grid already fills the viewport?
3. On a realistic small self-hosted box where most projects sit at rest most of the time, does the page ever get to look "alive" outside a best-case screenshot — and if not, does the whole "living resource" premise need a second, at-rest-focused design pass?
