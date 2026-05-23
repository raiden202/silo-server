# Server Query Hardening And Optimization Audit

Last updated: 2026-04-19

## Purpose

This document is a living reference for server-side query hardening and performance work.
It captures what the current queries are trying to do, where the highest-leverage issues are,
and what order to tackle them in.

This pass was a read-only code audit across the main server query surfaces. It was grounded in
the current code paths, but it did not include live `EXPLAIN ANALYZE` sampling, so treat the
items below as implementation-informed priorities rather than measured query plans.

A second read-only verification pass reviewed this document for accuracy, expected benefit, and
implementation risk. The recommendations below incorporate that review, including places where
the original plan was too broad, slightly stale, or needed behavior-preservation caveats.

## Scope Covered

- Shared catalog/query engine
- API handlers and browse/query endpoints
- Auth, session, profile, and user-state storage
- Scanner, metadata, admin jobs, and catalog seed import/export
- Playback, Jelly compat, subtitles, plugins, sections, node pool, and webhook sync

## Top Priorities

### P1: Fix live catalog count and pagination correctness

The highest-priority issue is that some catalog and browse paths count joined rows instead of
deduped items.

- `internal/catalog/browse.go`
- `internal/api/handlers/catalog.go`

Why it matters:

- `total` and `has_more` can be inflated
- items can be effectively overcounted when they belong to multiple libraries or person joins
- fallback query loops can do extra pages based on a bad total

Recommended fix:

- Make the count query operate on the same deduped relation as the data query
- Prefer `COUNT(DISTINCT mi.content_id)` or a grouped subquery over raw `COUNT(*)`

### P1: Stop materializing full candidate sets in query-source fallbacks

The main query-source fallback path currently does too much work when it cannot stay on the
direct SQL path.

- `internal/catalog/catalog_resolver.go`
- `internal/catalog/item_repo.go`

Current pattern:

- fetch all search candidates
- count inside `Search`
- page through `Search`
- sometimes re-fetch all candidates again for `name_prefix` or secondary sort handling

Why it matters:

- scales poorly with library size
- duplicates count work
- turns many searches into full-set materialization problems

Recommended fix:

- move resolver call sites onto the existing no-total page paths where only `has_more` is needed
- push `name_prefix` down into SQL only with a semantics-preserving rewrite
- avoid whole-candidate fetches for non-relevance sorts unless absolutely necessary

### P1: Fix technical filter and sort scoping for disabled libraries

Technical media-file predicates and joins do not consistently honor disabled-library exclusions.

- `internal/catalog/query_executor.go`
- `internal/catalog/query_builder.go`

Why it matters:

- a disabled-library file can still satisfy `resolution`, `hdr`, `bitrate`,
  `audio_language`, or `subtitle_language`
- this is both a correctness issue and a trust-boundary issue for filtered views

Recommended fix:

- pass disabled-library scope all the way into media-file `EXISTS` predicates and sort joins
- make technical filter/sort scoping use the same effective library rules as item visibility

## Cross-Cutting Themes

### Deduped counts must match data semantics

Anywhere a query joins `media_item_libraries`, `item_people`, or similar fanout tables, the
count path must match the item-level dedupe semantics of the data query.

### Avoid repeated full counts when `has_more` is enough

Several paths still pay for `COUNT(*)` on every page even when the caller only needs to know
whether another page exists. Add explicit no-total execution paths where possible.

### Batch hydration instead of per-item lookups

The codebase has multiple season/detail/compat/profile surfaces that still do follow-up queries
per item, per episode, or per installation. Those should move to batched list queries or
per-request caching.

### Wrap multi-step reconciliation in transactions

Some write paths are still read-check-write loops or multi-statement reconciliation sequences
without a transaction. Those should be hardened before deeper tuning.

### Preserve Existing Behavior While Optimizing

Several of the recommendations below are only safe if they preserve current semantics:

- `name_prefix` must continue matching the current `title OR sort_title` behavior
- `release_date` must keep its cross-scope movie / series / episode contract
- provider-chain batching must preserve fallback display names, priority defaults, and ordering
- representative-file batching must preserve current first-hit and fallback selection behavior
- compat startup changes must not regress clients that depend on current manifest readiness behavior
- subtitle dedupe must become conflict-aware before it is made more aggressive

### Indexes should match real read shapes

A number of hot paths have indexes that are close, but not quite aligned with their current
query predicates or sort order.

## Detailed Findings By Area

### Shared Catalog Query Layer

Files:

- `internal/catalog/browse.go`
- `internal/catalog/query_definition.go`
- `internal/catalog/query_builder.go`
- `internal/catalog/query_executor.go`
- `internal/catalog/catalog_resolver.go`
- `internal/catalog/air_date_sql.go`

Key findings:

- Browse counts do not match deduped item semantics when library or person joins are present.
- `release_date` is modeled as a text expression in the shared query definition even though the
  movie-side field is date-typed. That weakens type semantics and index use, but it is also part
  of a cross-scope contract for series and episode surfaces, so it cannot be split carelessly.
- `name_prefix` is often filtered in Go after fetching candidates instead of being pushed into SQL,
  but any pushdown must preserve current `title OR sort_title` matching semantics.
- `fetchAllBrowseCandidates()` currently pays for a full count on every page.
- Browse `ASC` sorts on nullable fields do not preserve the same null-handling semantics as the
  shared query-builder sort path. Fixing that will change visible ordering and should be treated
  as a behavior adjustment, not only as an optimization.
- `effectiveLastAirDateExpr()` is centralized and semantically strong, but it can still be reused
  more efficiently on the browse side when a sort join has already materialized the same aggregate.

What is already strong:

- explicit access scoping
- stable tie-break ordering
- centralized `last_air_date` normalization
- same-file technical rule collapsing for positive technical predicates

Recommended next steps:

1. Fix count/data parity in browse and legacy catalog query paths.
2. Push `name_prefix` into SQL with an exact semantics-preserving rewrite and matching index plan.
3. Rework `release_date` handling carefully so planner gains do not break series/episode parity.
4. Move fallback resolver call sites onto the existing no-total browse/query paths.

### API Handlers And Surface-Level Query Use

Files:

- `internal/api/handlers/catalog.go`
- `internal/api/handlers/items.go`
- `internal/api/handlers/user_state.go`
- `internal/api/handlers/profiles.go`
- `internal/api/handlers/admin.go`
- `internal/api/handlers/nodes.go`

Key findings:

- The legacy `POST /catalog/query` path has the same join-row count inflation risk as the shared
  browse layer.
- Catalog item hydration still does extra follow-up passes for overlay summaries, user state,
  and episode metadata.
- Season detail still contains per-episode aggregate user-state work; episode detail is much less
  problematic and should not be lumped into the same recommendation.
- Profile creation currently does extra pre-read work and has race potential around `max_profiles`
  and first-primary assignment.
- Some profile summary uses only need names and IDs, but the backing repository eagerly loads
  additional profile-library state. This is mainly an admin and session-list summary issue, not a
  reason to change full profile payloads everywhere.
- Node force-reload loads all nodes and filters in Go instead of using the enabled-node query shape.

Recommended next steps:

1. Collapse season aggregate hydration into batched file and progress reads.
2. Make profile creation atomic with locking/transaction semantics that preserve bootstrap and
   allowed-library write behavior.
3. Add lightweight profile summary queries for admin and session-list surfaces instead of changing
   the semantics of full profile list calls.

### Auth, Sessions, Profiles, And User State

Files:

- `internal/auth/session.go`
- `internal/userstore/pgstore/progress.go`
- `internal/userstore/pgstore/section_overrides.go`
- `internal/userstore/pgstore/collections.go`
- `internal/api/handlers/api_keys.go`

Key findings:

- `auth_sessions` already supports user-scoped listing, revocation, and expiry cleanup at the
  repository level, but the schema does not appear to have ideal index support for the current
  list/revoke shapes.
- Hidden-history suppression already has a PK on `(user_id, profile_id, media_item_id)` and a
  profile/time index. A wider composite index may still help the suppression probes, but that
  should be validated with `EXPLAIN` before treating it as an obvious win.
- Profile progress listing sorts by `updated_at DESC` without a matching index shape.
- Postgres section overrides are stored as a single JSON blob in `user_settings`, which causes
  read amplification and creates lost-update risk. Full normalization is a later structural option,
  but smaller compare-and-swap or transactional protections may be the better first step.
- Collection listing still does per-collection profile hydration, but the ROI depends on how large
  those lists get in practice.
- The admin API-key create path is safe so long as it remains admin-gated, but it should stay
  clearly separated from self-service creation semantics.

Recommended next steps:

1. Add session indexes aligned with the real list/revoke paths, and only add expiry-cleanup support
   if a cleanup job is actually wired.
2. Add a progress-list index aligned with the real sort shape.
3. Add transactional CAS/versioning protection to section overrides first; treat full normalization
   as a later cleanup if the surface keeps growing.
4. Batch collection/profile membership loading where the surface area justifies it.

### Scanner, Metadata, Admin Jobs, And Catalog Seed

Files:

- `internal/scanner/scanner.go`
- `internal/metadata/chain.go`
- `internal/metadata/refresh_debt_repo.go`
- `internal/adminjob/item_refresh.go`
- `internal/adminjob/repository.go`
- `internal/catalogseed/service.go`

Key findings:

- `syncPresentLibraryState` performs a multi-step reconciliation without a transaction.
- Metadata provider-chain resolution is still chatty and partially N+1, but any batching must
  preserve current fallback display-name, default-priority, and ordering semantics.
- `AppendProviderToAllChains` is a read-check-write loop without a transaction.
- Representative-file resolution for series and seasons is still N+1, but any replacement needs to
  preserve the current first-hit and fallback selection behavior.
- Refresh-debt claiming performs broad cleanup work inline with claim flow. If that cleanup moves,
  an equivalent prune path must remain in place.
- Admin job list-by-type lacks an ideal supporting index.
- Catalog export still does avoidable read amplification, especially when loading totals, but this
  is lower-priority admin progress plumbing rather than a top-tier hot-path issue.

Recommended next steps:

1. Wrap the truly non-transactional reconciliation and mutation paths in transactions.
2. Batch provider-chain metadata and priority resolution only if fallback and ordering semantics are
   preserved explicitly.
3. Replace representative-file N+1 lookups only with a query that preserves first-hit selection and
   fallback behavior.
4. Move broad queue cleanup out of hot claim paths only if an equivalent prune mechanism remains.
5. Treat export total collapsing as a lower-priority admin-path tuning item.

### Playback, Jelly Compat, Subtitles, Plugins, Sections, Nodes, Webhook Sync

Files:

- `internal/jellycompat/streams.go`
- `internal/jellycompat/playback_sessions.go`
- `internal/jellycompat/handlers_items.go`
- `internal/jellycompat/content_direct.go`
- `internal/playback/session.go`
- `internal/subtitles/pgrepo.go`
- `internal/subtitles/manager.go`
- `internal/plugins/task_registry.go`
- `internal/plugins/user_config.go`
- `internal/webhooksync/repo_events.go`
- `internal/sections/fetcher.go`
- `internal/nodepool/repository.go`
- `internal/watchtogether/repository.go`

Key findings:

- Compat playback route resolution still scans active sessions linearly.
- Downloaded subtitle rows are repeatedly reloaded on visible compat/playback paths.
- Compat browse and season surfaces still over-fetch and rehydrate too much state.
- Some stream paths re-read in-memory session state immediately after update.
- Manifest waiting uses tight polling instead of an event or gentler backoff, but that behavior
  exists partly for client compatibility and should not be changed casually.
- Playback session bookkeeping still does repeated full-map scans by user and media file.
- Subtitle dedupe is based on pre-check plus insert instead of an authoritative uniqueness rule, and
  any change here must become conflict-aware to avoid deleting another request's successful insert.
- Plugin task-registry building is still N+1 across installations and capabilities.
- Plugin user config scans all user settings for prefix filtering and does non-transactional replace.
- Webhook event retention trims with a broad self-correlated delete on every insert. The issue is
  the delete shape and churn cost, not a missing index in the current schema.
- Random sections use `ORDER BY RANDOM()`.
- Watch-together lookup may want a functional index on `lower(code)` only if case-insensitive room
  codes are a real requirement; normalizing input to exact-match semantics may be cheaper.

Recommended next steps:

1. Add reverse indexes for compat and playback session lookup.
2. Cache or batch downloaded subtitle hydration.
3. Add a uniqueness-backed, conflict-safe dedupe path for subtitle storage.
4. Flatten plugin registry rebuild queries.
5. Replace `ORDER BY RANDOM()` only if random sections become a notable cost center.

## Index Candidates

These are the most obvious index opportunities from this audit:

- `auth_sessions` indexes aligned with `ListByUser` / `RevokeAllByUser`, plus an `expires_at`
  index only if expiry cleanup is actually scheduled
- consider `user_history_hidden_items (user_id, profile_id, media_item_id, hidden_before DESC)`
  only if `EXPLAIN` shows it materially improves suppression probes beyond the PK and current
  profile/time index
- progress listing index aligned with `updated_at DESC`
- `downloaded_subtitles (media_file_id, created_at DESC)` if subtitle history ordering matters
- webhook event retention delete-shape tuning; the current composite index already exists
- functional index on `lower(code)` for watch-together room lookup only if case-insensitive codes
  are a real product requirement
- compat session support indexes on expiry and streamapp user keys

## Suggested Rollout Order

### Wave 1: Correctness And Hot Query Shape Fixes

- Fix deduped count semantics in browse and legacy catalog query paths
- Fix disabled-library scoping in technical filters and sorts
- Remove full-set query fallback behavior where possible
- Make scanner reconciliation transactional

### Wave 2: Index Pack

- session indexes aligned with real list/revoke paths, plus optional expiry-cleanup support
- hidden-history index improvement only if supported by `EXPLAIN`
- progress-list index
- webhook retention delete-shape tuning
- watch-together lookup strategy, which may be normalization rather than a new functional index

### Wave 3: Batch Hydration

- season aggregate detail and user-state hydration
- compat subtitle loading
- compat season/progress surfaces
- plugin registry loading
- collection/profile membership loading where ROI is clear

### Wave 4: Structural Cleanups

- normalize section overrides away from JSON blob storage only if smaller CAS/versioning fixes are
  not sufficient
- improve provider-chain resolution and mutation paths without changing current fallback/order
  semantics
- replace broad queue cleanup in refresh-debt claiming only with an equivalent prune path
- revisit random-section strategy if still needed

## How To Update This Document

When a query issue is fixed or re-scoped:

1. Keep the section, but mark the item as addressed or reduced in scope.
2. Add the file path or migration that changed the behavior.
3. If a finding was disproven with measurement, note the evidence and why it is no longer a priority.
4. Prefer moving solved items into a short "Resolved" subsection instead of deleting the history.

## Notes

- This audit intentionally distinguishes existing good structure from true gaps. The catalog query
  layer is already a strong foundation; most of the leverage now is in tightening count semantics,
  avoiding full-set fallbacks, batching repeated hydration, and hardening multi-step mutations.
- Reliability can still beat small theoretical query savings on user-facing browse surfaces. Where
  exact totals are important, keep exact totals, but make sure they are counting the right thing.
