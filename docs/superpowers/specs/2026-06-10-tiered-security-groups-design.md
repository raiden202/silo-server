# Tiered Security Model: Group-Based Permissions — Design

Date: 2026-06-10
Status: Approved design, pending implementation plan

Commands and paths in this document assume the repository root is the cwd.

## Overview

Replace Silo's per-user authorization fields with a group-based model:

- Admins create **groups**; each group carries a full policy (feature permissions, library access, playback/transcode limits, download rights).
- Users are assigned to one or more groups; effective policy is the most-permissive union of their groups.
- Groups are the **only** source of policy. Per-user policy columns are removed.
- Two protected built-in groups ship by default: `administrators` and `users`. New accounts join the group(s) named by a server setting (initially `users`).
- The `users.role` column and JWT role claim are removed; "admin" becomes a permission granted by group membership.

Deployments may have **thousands of users**. All fan-out paths (revision bumps, member listings, migration artifacts) are designed set-based and paginated accordingly.

## Current state (what this replaces)

- `users.role` text (`"admin"`/`"user"`), checked by `RequireAdmin` middleware via JWT `Claims.Role` (`internal/api/middleware/auth.go`).
- `users.permissions text[]` with two assignable permissions (`marker_edit`, `metadata_curation`) defined in `internal/auth/permissions.go`; admins implicitly hold all.
- Per-user resource policy columns on `users`: `library_ids`, `max_streams`, `max_transcodes`, `max_profiles`, `max_playback_quality`, `download_allowed`, `download_transcode_allowed`.
- `users.access_policy_revision` invalidates profile tokens via `internal/access/resolver.go`.
- Household profiles (`user_profiles`) carry a separate restriction layer (child flag, content rating, allowed libraries). **Profiles are out of scope** — that layer is unchanged and continues to apply on top of the user's effective policy.

## Data model

### `groups`

```sql
CREATE TABLE groups (
    id                         serial PRIMARY KEY,
    slug                       text NOT NULL UNIQUE,      -- stable identifier: 'administrators', 'users', ...
    name                       text NOT NULL,             -- display name, renamable
    description                text NOT NULL DEFAULT '',
    built_in                   boolean NOT NULL DEFAULT false,
    permissions                text[] NOT NULL DEFAULT '{}',
    library_ids                integer[],                 -- NULL = all libraries
    max_streams                integer NOT NULL DEFAULT 6,
    max_transcodes             integer NOT NULL DEFAULT 2,
    max_profiles               integer NOT NULL DEFAULT 5,
    max_playback_quality       text NOT NULL DEFAULT '',  -- '' = unrestricted
    download_allowed           boolean NOT NULL DEFAULT true,
    download_transcode_allowed boolean NOT NULL DEFAULT false,
    created_at                 timestamptz NOT NULL DEFAULT now(),
    updated_at                 timestamptz NOT NULL DEFAULT now()
);
```

### `user_groups`

```sql
CREATE TABLE user_groups (
    user_id    integer NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id   integer NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, group_id)
);
CREATE INDEX user_groups_group_id_idx ON user_groups (group_id);
```

### Admin as a permission

`'admin'` is a permission string in `groups.permissions`, not a column. The resolver treats it as implying every permission and unrestricted policy, mirroring the current `HasEffectivePermission` short-circuit for `role == "admin"`. One mechanism covers everything.

Permission catalog at launch: `admin`, `marker_edit`, `metadata_curation`. The catalog stays a Go-level allowlist in `internal/auth/permissions.go`; adding a permission is a code change, as today.

### Built-in groups

| Slug | Permissions | Policy | Protections |
|---|---|---|---|
| `administrators` | `['admin']` | irrelevant (admin implies unrestricted) | not deletable; `admin` permission not removable; last enabled member not removable |
| `users` | `['marker_edit']` | all libraries, 6 streams, 2 transcodes, 5 profiles, downloads on, transcode downloads off | not deletable |

Protections are enforced at the service layer (consistent with existing last-admin guards), not by triggers.

### `users` table changes

Dropped: `role`, `permissions`, `library_ids`, `max_streams`, `max_transcodes`, `max_profiles`, `max_playback_quality`, `download_allowed`, `download_transcode_allowed`.

Kept: `enabled` (account kill-switch stays user-level), `access_policy_revision` (still the invalidation counter).

### Default group setting

A `server_settings` entry `default_group_slugs` (initially `["users"]`), consumed by all account-provisioning paths (signup, invite, OAuth) in `internal/auth/account_provisioner.go`. Invite codes may later override it; out of scope now.

## Resolution semantics

A user's **effective policy** is the most-permissive union of their groups:

| Field | Rule |
|---|---|
| `permissions` | set union; `admin` present → every permission granted |
| `library_ids` | union; any group with `NULL` (all) or `admin` → unrestricted |
| `max_streams`, `max_transcodes`, `max_profiles` | max across groups |
| `max_playback_quality` | highest wins; `''` (unrestricted) beats everything (ordering as in `internal/access/quality.go`) |
| `download_allowed`, `download_transcode_allowed` | OR |

**Zero groups → empty policy**: no permissions, no libraries, zero streams. The user can authenticate but sees an empty server. This is deliberate ("groups only grant") and gives admins a soft-disable distinct from `enabled = false`.

**Where computed:** the user repository (`internal/auth/repository.go`) aggregates group policy in the same query that loads the user (join over `user_groups`/`groups`; cost is O(memberships per user), independent of total user count). `models.User` keeps its existing policy fields (`LibraryIDs`, `MaxStreams`, `Permissions`, …) but they become **effective** values populated from the aggregate. The ~25 consumer packages (playback, catalog access filters, jellycompat, sections, audiobooks, requests, …) keep working unchanged. `Role` is replaced by `IsAdmin bool` derived from effective permissions. Assigned memberships are exposed separately (`GroupIDs`) for the admin API.

**Invalidation:** unchanged mechanism, new triggers:

- Group policy edit → bump `access_policy_revision` for all members in **one set-based `UPDATE ... FROM user_groups`** (never a per-user loop; the `users` group may have thousands of members).
- Membership add/remove → bump the affected user's revision.

The profile-token revision check in `internal/access/resolver.go` then invalidates stale scopes automatically. The post-bump re-resolution stampede is bounded: each active session re-resolves once with a single indexed query on its next request. No new caching layer until profiling shows a need.

## Enforcement changes

- **JWT claims:** `Claims.Role` removed. Claims carry identity only (`user_id`, session, profile, revision) — never policy. Fixes the existing staleness gap where a role change waited for token re-issuance; after this, revoking admin takes effect on the next request.
- **`RequireAdmin`** (`internal/api/middleware/auth.go`): loads the user (effective policy included) and checks `IsAdmin`; stashes the loaded user in request context so handlers don't re-fetch. One indexed query on low-traffic admin routes.
- **Permission middleware** (`internal/api/middleware/permissions.go`): logic unchanged — it already operates on `models.User`. Only `user.Role == "admin"` comparisons become `user.IsAdmin`.
- **Jellyfin compat** (`internal/jellycompat/handlers_auth.go`): `UserPolicy` maps from effective policy — `IsAdministrator` ← `IsAdmin`, `EnableContentDownloading` ← `DownloadAllowed`, `EnableAllFolders`/`EnabledFolders` ← `LibraryIDs`. Replaces the currently hardcoded policy values.
- **Requests service** (`internal/requests`): role-string comparisons → `IsAdmin`; otherwise unchanged.

## API surface

All new endpoints behind `RequireAdmin`, under the existing admin API namespace:

- `GET /api/admin/groups` — list groups with member counts (single aggregated count query)
- `POST /api/admin/groups` — create; slug auto-derived from name
- `GET /api/admin/groups/{id}` — group detail: policy + member **count** (members are paginated separately)
- `GET /api/admin/groups/{id}/members?offset=&limit=` — paginated member list
- `PATCH /api/admin/groups/{id}` — update name/description/policy; rejects removing `admin` from `administrators`
- `DELETE /api/admin/groups/{id}` — rejects built-ins; memberships cascade
- `PUT /api/admin/groups/{id}/members/{userID}` — add member
- `DELETE /api/admin/groups/{id}/members/{userID}` — remove member; rejects removing the last enabled member of `administrators`

Changed endpoints:

- `POST`/`PATCH` admin user endpoints: per-user policy fields removed from request bodies, replaced by `group_ids`. Responses gain `groups: [{id, slug, name}]` and a read-only `effective_policy` object.
- User-facing account/session responses keep a **derived** `role` field (`"admin"`/`"user"` from `IsAdmin`) so `silo-android` and `silo-apple` need no immediate changes; group info is added alongside.

Default-group setting rides the existing server-settings endpoints.

## Admin UI (`web/src/pages/admin`)

- New **Groups** page: list (name, member count, key-policy summary) and an editor with three sections — permissions (checkboxes), library access (all / picker, reusing the user editor's component), limits (streams, transcodes, quality, downloads, profiles). Member management with a paginated member list and user search to add.
- **Users** page: per-user policy controls replaced by a group multi-select plus a read-only "effective policy" summary showing the resolved union — the admin's answer to "why can this user do X".
- Built-in groups render with a badge; delete and protected fields disabled with explanatory tooltips.

## Migration

One transactional Goose migration created with `make migrate-create NAME=group_based_permissions`:

1. Create `groups` and `user_groups`; seed `administrators` and `users` (policy from current column defaults).
2. Map members: `role = 'admin'` → `administrators`; **all** users → `users`.
3. Preserve outliers **bucketed by distinct policy tuple**: non-admin users whose policy columns deviate from the defaults are grouped by their unique `(permissions, library_ids, max_streams, max_transcodes, max_profiles, max_playback_quality, download_allowed, download_transcode_allowed)` combination; one group per distinct tuple (`migrated-policy-1`, `-2`, …) is created carrying those values, and matching users are added. Group count is bounded by distinct policies in use, not user count. Post-migration effective policy is provably ≥ each user's prior policy; nothing is silently lost. (Admins were already unrestricted, so admin outliers need no preservation.)
4. Drop the old `users` columns; bump all `access_policy_revision` values set-based.

Down-migration recreates the columns and writes back each user's flattened effective policy (best-effort; acceptable at WIP stage).

## Testing

- Union resolver unit tests: multi-group merges, `admin` short-circuit, zero-group empty policy, NULL-libraries propagation, quality ordering (extending `internal/access/quality_test.go` coverage).
- Service-layer guard tests: delete built-in, strip `admin` from `administrators`, remove last enabled administrator.
- Migration test against a seeded DB asserting before/after effective-policy equivalence, including outlier users sharing a policy tuple.
- Handler tests for group endpoints incl. member pagination; updated user-endpoint tests for changed payloads.
- Jellycompat test asserting `UserPolicy` reflects group-derived policy.

## Risks & follow-ups

- **Client coordination:** `silo-android` / `silo-apple` keep working via the derived `role` field; follow-up tickets to surface group membership in account UI.
- **Revision-bump stampede:** bounded and acceptable (one re-resolve per active session); revisit with a cache only if profiling shows pressure.
- **Invite-code group overrides** and an expanded permission catalog (requests, watch-together, etc.) are natural follow-ups, out of scope here.
- **Plugin/API-key auth paths** (`internal/auth/api_key_repository.go`, jellycompat API keys) must be audited during implementation for any `role` reads not covered above.
