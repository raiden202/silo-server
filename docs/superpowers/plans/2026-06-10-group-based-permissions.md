# Group-Based Tiered Security Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace per-user authorization fields with a group-based model: groups carry full policy, users join one or more groups, effective policy is the most-permissive union, and `admin` is a permission granted by the built-in `administrators` group.

**Architecture:** Two new tables (`groups`, `user_groups`) become the only source of policy. The user repository hydrates `models.User` with *effective* values (union of the user's groups) so the ~25 consumer packages keep working unchanged. `users.role`, `users.permissions`, and per-user policy columns are dropped; JWT claims lose `Role`; `RequireAdmin` loads the user and checks a derived `IsAdmin`. Invalidation reuses `access_policy_revision` (set-based bumps on group changes).

**Tech Stack:** Go (chi, pgx/v5), Goose SQL migrations, PostgreSQL, React + TypeScript (react-query hooks, shadcn-style components).

**Spec:** `docs/superpowers/specs/2026-06-10-tiered-security-groups-design.md`

Commands assume the repository root is the cwd. Backend tests: `go test ./internal/... `. The repo has a **no-testcontainers policy** — pure logic gets unit tests; SQL repositories are verified by build + handler tests with fakes; the migration is verified against the local docker compose Postgres.

**Branch:** create `feat/group-based-permissions` off `main` before Task 1.

---

## File Structure

| File | Responsibility |
|---|---|
| `migrations/sql/<ts>_group_based_permissions.sql` | Create/seed tables, map memberships, drop old columns |
| `internal/models/group.go` | `Group`, `CreateGroupInput`, `UpdateGroupInput`, built-in slugs |
| `internal/models/user.go` | `User` gains `IsAdmin`, `GroupIDs`; inputs lose policy fields |
| `internal/access/quality.go` | Add `MaxQuality` (most-permissive merge) |
| `internal/auth/effective_policy.go` | Pure union resolver `ApplyEffectivePolicy` |
| `internal/auth/permissions.go` | Add `admin` permission; helpers use `IsAdmin` |
| `internal/auth/group_repository.go` | Group CRUD, membership, guards, revision bumps |
| `internal/auth/repository.go` | User load hydrates effective policy from groups |
| `internal/auth/service.go` | Setup/signup/impersonation use groups |
| `internal/auth/account_provisioner.go` | Default-group assignment on account creation |
| `internal/auth/jwt.go` | Remove `Role` from claims |
| `internal/api/middleware/auth.go` | `RequireAdmin` checks loaded user's `IsAdmin` |
| `internal/api/handlers/admin_groups.go` | Group admin endpoints |
| `internal/api/handlers/admin.go` | User payloads: `group_ids` in, groups + derived role out |
| `internal/api/router.go` | Wire group routes and middleware changes |
| `internal/jellycompat/handlers_auth.go` | `UserPolicy` from effective policy |
| `web/src/api/types.ts` (or the existing types module) | `AdminGroup` types |
| `web/src/hooks/queries/admin/groups.ts` | React-query hooks for groups |
| `web/src/pages/AdminGroups.tsx` | Groups admin page |
| `web/src/pages/AdminUsers.tsx`, `AdminUserDetail.tsx` | Group multi-select + effective policy summary |

---

### Task 1: `MaxQuality` helper in `internal/access`

The union resolver needs "most permissive quality wins; `''` (unrestricted) beats everything". `internal/access/quality.go` already has `CompareQuality`, `MinQuality`, `NormalizePlaybackQuality`.

**Files:**
- Modify: `internal/access/quality.go`
- Test: `internal/access/quality_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/access/quality_test.go`:

```go
func TestMaxQuality(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{"", "", ""},
		{"", "1080p", ""},        // unrestricted beats any ceiling
		{"2160p", "", ""},
		{"1080p", "2160p", "2160p"},
		{"2160p", "1080p", "2160p"},
		{"1080p", "1080p", "1080p"},
		{"720p", "480p", "1080p"}, // presets normalize: 720p/480p both -> 1080p (standard)
	}
	for _, tc := range cases {
		if got := MaxQuality(tc.a, tc.b); got != tc.want {
			t.Errorf("MaxQuality(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/access/ -run TestMaxQuality -v`
Expected: FAIL — `undefined: MaxQuality`

- [ ] **Step 3: Implement `MaxQuality`**

Append to `internal/access/quality.go`:

```go
// MaxQuality returns the more permissive quality ceiling. An empty string
// means unrestricted and beats any concrete ceiling.
func MaxQuality(a, b string) string {
	a = NormalizePlaybackQuality(a)
	b = NormalizePlaybackQuality(b)
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return ""
	}
	if CompareQuality(a, b) >= 0 {
		return a
	}
	return b
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/access/ -v`
Expected: PASS (all existing quality tests too)

- [ ] **Step 5: Commit**

```bash
git add internal/access/quality.go internal/access/quality_test.go
git commit -m "feat(access): add MaxQuality for most-permissive quality merge"
```

---

### Task 2: Group model, `IsAdmin`/`GroupIDs` on User, `admin` permission

Additive model changes that keep everything compiling and behaving as before. The interim trick: `scanUser` sets `IsAdmin = (Role == "admin")` so all `IsAdmin`-based logic is correct before the migration lands (Task 6 replaces this with group-derived values).

**Files:**
- Create: `internal/models/group.go`
- Modify: `internal/models/user.go` (add fields only — do NOT remove anything yet)
- Modify: `internal/auth/permissions.go`
- Modify: `internal/auth/repository.go` (scanUser/scanUsers: set interim `IsAdmin`)
- Test: `internal/auth/permissions_test.go`

- [ ] **Step 1: Create `internal/models/group.go`**

```go
package models

import "time"

// Built-in group slugs. Built-in groups cannot be deleted; the
// administrators group cannot lose the admin permission or its last
// enabled member.
const (
	GroupSlugAdministrators = "administrators"
	GroupSlugUsers          = "users"
)

// Group represents a row in the groups table. Groups are the only source of
// authorization policy: a user's effective policy is the most-permissive
// union of their groups.
type Group struct {
	ID                       int
	Slug                     string
	Name                     string
	Description              string
	BuiltIn                  bool
	Permissions              []string
	LibraryIDs               []int // nil = all libraries
	MaxStreams               int
	MaxTranscodes            int
	MaxProfiles              int
	MaxPlaybackQuality       string // "" = unrestricted
	DownloadAllowed          bool
	DownloadTranscodeAllowed bool
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// CreateGroupInput contains the fields to create a new group. The slug is
// derived from Name. Nil pointer fields use the DB defaults.
type CreateGroupInput struct {
	Name                     string // required
	Description              string
	Permissions              []string
	LibraryIDs               []int // nil = all libraries
	MaxStreams               *int
	MaxTranscodes            *int
	MaxProfiles              *int
	MaxPlaybackQuality       *string
	DownloadAllowed          *bool
	DownloadTranscodeAllowed *bool
}

// UpdateGroupInput contains optional fields for updating a group.
// Nil means "don't update". LibraryIDs follows the same convention the old
// models.UpdateUserInput used: a non-nil pointer to a nil slice means
// "all libraries"; a pointer to an empty slice means "none".
type UpdateGroupInput struct {
	Name                     *string
	Description              *string
	Permissions              *[]string
	LibraryIDs               *[]int // nil = don't update; *nil = all; *[] = none
	MaxStreams               *int
	MaxTranscodes            *int
	MaxProfiles              *int
	MaxPlaybackQuality       *string
	DownloadAllowed          *bool
	DownloadTranscodeAllowed *bool
}
```

- [ ] **Step 2: Add fields to `models.User`** in `internal/models/user.go` (after `Role`):

```go
	// IsAdmin and GroupIDs are derived from group membership at load time.
	IsAdmin  bool
	GroupIDs []int
```

- [ ] **Step 3: Write failing tests for the `admin` permission and `IsAdmin`-based helpers**

In `internal/auth/permissions_test.go`, update every test constructing `&models.User{Role: "admin", ...}` to also set `IsAdmin: true`, and add:

```go
func TestAdminPermissionIsAssignable(t *testing.T) {
	normalized, err := NormalizePermissions([]string{"admin"})
	if err != nil {
		t.Fatalf("admin must be an assignable permission: %v", err)
	}
	if len(normalized) != 1 || normalized[0] != "admin" {
		t.Fatalf("got %v, want [admin]", normalized)
	}
}

func TestIsAdminGrantsAllPermissions(t *testing.T) {
	u := &models.User{Enabled: true, IsAdmin: true}
	if !HasEffectivePermission(u, PermissionMarkerEdit) {
		t.Error("admin should hold marker_edit")
	}
	if !HasEffectivePermission(u, PermissionMetadataCuration) {
		t.Error("admin should hold metadata_curation")
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run 'TestAdminPermission|TestIsAdmin' -v`
Expected: FAIL — `unknown permission "admin"` and admin-grant check failing (IsAdmin not consulted yet)

- [ ] **Step 5: Update `internal/auth/permissions.go`**

```go
const (
	PermissionAdmin            Permission = "admin"
	PermissionMarkerEdit       Permission = "marker_edit"
	PermissionMetadataCuration Permission = "metadata_curation"
)

var assignablePermissions = map[Permission]struct{}{
	PermissionAdmin:            {},
	PermissionMarkerEdit:       {},
	PermissionMetadataCuration: {},
}
```

Replace the two `user.Role == "admin"` checks with `user.IsAdmin`:

```go
func HasEffectivePermission(user *models.User, permission Permission) bool {
	if user == nil || !user.Enabled {
		return false
	}
	if user.IsAdmin {
		return isAssignablePermission(permission)
	}
	return HasAssignedPermission(user, permission)
}

func EffectivePermissions(user *models.User) []string {
	if user == nil || !user.Enabled {
		return []string{}
	}
	if user.IsAdmin {
		return assignablePermissionList()
	}
	permissions, err := NormalizePermissions(user.Permissions)
	if err != nil {
		return []string{}
	}
	return permissions
}
```

- [ ] **Step 6: Interim `IsAdmin` derivation in `scanUser`/`scanUsers`** (`internal/auth/repository.go`)

In both functions, after the successful `Scan`, add:

```go
	u.IsAdmin = u.Role == "admin"
```

This keeps every `IsAdmin` consumer correct until Task 6 derives it from groups.

- [ ] **Step 7: Build and run the package tests**

Run: `go build ./... && go test ./internal/auth/ ./internal/models/ -count=1`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/models/group.go internal/models/user.go internal/auth/permissions.go internal/auth/permissions_test.go internal/auth/repository.go
git commit -m "feat(auth): group model, admin permission, IsAdmin derivation"
```

---

### Task 3: Effective-policy union resolver

Pure function; full TDD.

**Files:**
- Create: `internal/auth/effective_policy.go`
- Test: `internal/auth/effective_policy_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/auth/effective_policy_test.go`:

```go
package auth

import (
	"reflect"
	"sort"
	"testing"

	"github.com/Silo-Server/silo-server/internal/models"
)

func TestApplyEffectivePolicyZeroGroups(t *testing.T) {
	u := &models.User{ID: 1, Enabled: true}
	ApplyEffectivePolicy(u, nil)

	if u.IsAdmin {
		t.Error("zero groups must not grant admin")
	}
	if len(u.Permissions) != 0 {
		t.Errorf("permissions = %v, want empty", u.Permissions)
	}
	if u.LibraryIDs == nil || len(u.LibraryIDs) != 0 {
		t.Errorf("library ids = %v, want empty non-nil (access to none)", u.LibraryIDs)
	}
	if u.MaxStreams != 0 || u.MaxTranscodes != 0 || u.MaxProfiles != 0 {
		t.Error("limits must be zero for zero groups")
	}
	if u.DownloadAllowed || u.DownloadTranscodeAllowed {
		t.Error("downloads must be denied for zero groups")
	}
}

func TestApplyEffectivePolicyUnion(t *testing.T) {
	u := &models.User{ID: 1, Enabled: true}
	ApplyEffectivePolicy(u, []models.Group{
		{ID: 10, Permissions: []string{"marker_edit"}, LibraryIDs: []int{1, 2},
			MaxStreams: 2, MaxTranscodes: 1, MaxProfiles: 3,
			MaxPlaybackQuality: "1080p", DownloadAllowed: false},
		{ID: 20, Permissions: []string{"metadata_curation"}, LibraryIDs: []int{2, 3},
			MaxStreams: 4, MaxTranscodes: 2, MaxProfiles: 1,
			MaxPlaybackQuality: "2160p", DownloadAllowed: true},
	})

	wantPerms := []string{"marker_edit", "metadata_curation"}
	sort.Strings(u.Permissions)
	if !reflect.DeepEqual(u.Permissions, wantPerms) {
		t.Errorf("permissions = %v, want %v", u.Permissions, wantPerms)
	}
	wantLibs := []int{1, 2, 3}
	sort.Ints(u.LibraryIDs)
	if !reflect.DeepEqual(u.LibraryIDs, wantLibs) {
		t.Errorf("libraries = %v, want %v", u.LibraryIDs, wantLibs)
	}
	if u.MaxStreams != 4 || u.MaxTranscodes != 2 || u.MaxProfiles != 3 {
		t.Errorf("limits = %d/%d/%d, want 4/2/3", u.MaxStreams, u.MaxTranscodes, u.MaxProfiles)
	}
	if u.MaxPlaybackQuality != "2160p" {
		t.Errorf("quality = %q, want 2160p", u.MaxPlaybackQuality)
	}
	if !u.DownloadAllowed {
		t.Error("downloads must be OR-ed")
	}
	wantGroups := []int{10, 20}
	sort.Ints(u.GroupIDs)
	if !reflect.DeepEqual(u.GroupIDs, wantGroups) {
		t.Errorf("group ids = %v, want %v", u.GroupIDs, wantGroups)
	}
	if u.IsAdmin {
		t.Error("no group granted admin")
	}
}

func TestApplyEffectivePolicyNilLibrariesMeansAll(t *testing.T) {
	u := &models.User{}
	ApplyEffectivePolicy(u, []models.Group{
		{ID: 1, LibraryIDs: []int{1}},
		{ID: 2, LibraryIDs: nil}, // all libraries
	})
	if u.LibraryIDs != nil {
		t.Errorf("library ids = %v, want nil (unrestricted)", u.LibraryIDs)
	}
}

func TestApplyEffectivePolicyAdminShortCircuit(t *testing.T) {
	u := &models.User{}
	ApplyEffectivePolicy(u, []models.Group{
		{ID: 1, Permissions: []string{"admin"}, LibraryIDs: []int{1},
			MaxStreams: 1, MaxPlaybackQuality: "480p"},
	})
	if !u.IsAdmin {
		t.Fatal("admin permission must set IsAdmin")
	}
	if u.LibraryIDs != nil {
		t.Error("admin must be library-unrestricted")
	}
	if u.MaxPlaybackQuality != "" {
		t.Error("admin must be quality-unrestricted")
	}
}

func TestApplyEffectivePolicyQualityUnrestrictedWins(t *testing.T) {
	u := &models.User{}
	ApplyEffectivePolicy(u, []models.Group{
		{ID: 1, MaxPlaybackQuality: "1080p"},
		{ID: 2, MaxPlaybackQuality: ""}, // unrestricted
	})
	if u.MaxPlaybackQuality != "" {
		t.Errorf("quality = %q, want unrestricted", u.MaxPlaybackQuality)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run TestApplyEffectivePolicy -v`
Expected: FAIL — `undefined: ApplyEffectivePolicy`

- [ ] **Step 3: Implement** `internal/auth/effective_policy.go`:

```go
package auth

import (
	"sort"

	"github.com/Silo-Server/silo-server/internal/access"
	"github.com/Silo-Server/silo-server/internal/models"
)

// ApplyEffectivePolicy populates u's policy fields with the most-permissive
// union of the given groups. Groups are the only source of policy:
//
//   - permissions: set union; "admin" implies everything
//   - libraries:   union; any nil (= all) or admin makes it unrestricted
//   - limits:      max across groups
//   - quality:     most permissive; "" (unrestricted) wins
//   - booleans:    OR
//
// Zero groups yields the empty policy: no permissions, an empty non-nil
// library list (access to none), zero limits, downloads denied. The quality
// ceiling is moot at zero streams and left at the lowest concrete value.
func ApplyEffectivePolicy(u *models.User, groups []models.Group) {
	u.GroupIDs = make([]int, 0, len(groups))
	u.IsAdmin = false
	u.Permissions = []string{}
	u.MaxStreams, u.MaxTranscodes, u.MaxProfiles = 0, 0, 0
	u.DownloadAllowed, u.DownloadTranscodeAllowed = false, false
	u.MaxPlaybackQuality = "480p"
	u.LibraryIDs = []int{}

	if len(groups) == 0 {
		return
	}

	permSet := make(map[string]struct{})
	libSet := make(map[int]struct{})
	allLibraries := false

	for i, g := range groups {
		u.GroupIDs = append(u.GroupIDs, g.ID)
		for _, p := range g.Permissions {
			permSet[p] = struct{}{}
		}
		if g.LibraryIDs == nil {
			allLibraries = true
		} else {
			for _, id := range g.LibraryIDs {
				libSet[id] = struct{}{}
			}
		}
		if g.MaxStreams > u.MaxStreams {
			u.MaxStreams = g.MaxStreams
		}
		if g.MaxTranscodes > u.MaxTranscodes {
			u.MaxTranscodes = g.MaxTranscodes
		}
		if g.MaxProfiles > u.MaxProfiles {
			u.MaxProfiles = g.MaxProfiles
		}
		if i == 0 {
			u.MaxPlaybackQuality = g.MaxPlaybackQuality
		} else {
			u.MaxPlaybackQuality = access.MaxQuality(u.MaxPlaybackQuality, g.MaxPlaybackQuality)
		}
		u.DownloadAllowed = u.DownloadAllowed || g.DownloadAllowed
		u.DownloadTranscodeAllowed = u.DownloadTranscodeAllowed || g.DownloadTranscodeAllowed
	}

	if _, ok := permSet[string(PermissionAdmin)]; ok {
		u.IsAdmin = true
		allLibraries = true
		u.MaxPlaybackQuality = ""
		u.DownloadAllowed = true
	}

	u.Permissions = make([]string, 0, len(permSet))
	for p := range permSet {
		u.Permissions = append(u.Permissions, p)
	}
	sort.Strings(u.Permissions)

	if allLibraries {
		u.LibraryIDs = nil
	} else {
		u.LibraryIDs = make([]int, 0, len(libSet))
		for id := range libSet {
			u.LibraryIDs = append(u.LibraryIDs, id)
		}
		sort.Ints(u.LibraryIDs)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/effective_policy.go internal/auth/effective_policy_test.go
git commit -m "feat(auth): effective policy union resolver"
```

---

### Task 4: Database migration

**Files:**
- Create: `migrations/sql/<timestamp>_group_based_permissions.sql` via `make migrate-create NAME=group_based_permissions` (never hand-name the file; never create paired up/down files)

- [ ] **Step 1: Create the migration file**

Run: `make migrate-create NAME=group_based_permissions`
Expected: prints the created timestamped file path under `migrations/sql/`.

- [ ] **Step 2: Write the migration**

Replace the file's contents with:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE groups (
    id                         serial PRIMARY KEY,
    slug                       text NOT NULL UNIQUE,
    name                       text NOT NULL,
    description                text NOT NULL DEFAULT '',
    built_in                   boolean NOT NULL DEFAULT false,
    permissions                text[] NOT NULL DEFAULT '{}',
    library_ids                integer[],
    max_streams                integer NOT NULL DEFAULT 6,
    max_transcodes             integer NOT NULL DEFAULT 2,
    max_profiles               integer NOT NULL DEFAULT 5,
    max_playback_quality       text NOT NULL DEFAULT '',
    download_allowed           boolean NOT NULL DEFAULT true,
    download_transcode_allowed boolean NOT NULL DEFAULT false,
    created_at                 timestamptz NOT NULL DEFAULT now(),
    updated_at                 timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_groups (
    user_id    integer NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id   integer NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, group_id)
);
CREATE INDEX user_groups_group_id_idx ON user_groups (group_id);

INSERT INTO groups (slug, name, description, built_in, permissions)
VALUES
  ('administrators', 'Administrators',
   'Full access to everything, including server administration.', true,
   ARRAY['admin']),
  ('users', 'Users',
   'Default group for new accounts.', true,
   ARRAY['marker_edit']);

-- Admins -> administrators (admin implies unrestricted policy).
INSERT INTO user_groups (user_id, group_id)
SELECT u.id, g.id
FROM users u
JOIN groups g ON g.slug = 'administrators'
WHERE u.role = 'admin';

-- Non-admin users whose policy deviates from the defaults: bucket by
-- distinct policy tuple, one group per tuple, members join ONLY that group
-- (joining 'users' too would erase restrictions via the permissive union).
WITH outliers AS (
    SELECT u.*
    FROM users u
    WHERE u.role IS DISTINCT FROM 'admin'
      AND (
           COALESCE(u.permissions, '{}') IS DISTINCT FROM ARRAY['marker_edit']::text[]
        OR u.library_ids IS NOT NULL
        OR u.max_streams IS DISTINCT FROM 6
        OR u.max_transcodes IS DISTINCT FROM 2
        OR u.max_profiles IS DISTINCT FROM 5
        OR COALESCE(u.max_playback_quality, '') IS DISTINCT FROM ''
        OR u.download_allowed IS DISTINCT FROM true
        OR u.download_transcode_allowed IS DISTINCT FROM false
      )
),
buckets AS (
    SELECT o.id AS user_id,
           dense_rank() OVER (
               ORDER BY o.permissions, o.library_ids, o.max_streams,
                        o.max_transcodes, o.max_profiles,
                        o.max_playback_quality, o.download_allowed,
                        o.download_transcode_allowed
           ) AS n,
           o.permissions, o.library_ids, o.max_streams, o.max_transcodes,
           o.max_profiles, o.max_playback_quality, o.download_allowed,
           o.download_transcode_allowed
    FROM outliers o
),
distinct_buckets AS (
    SELECT DISTINCT ON (n) n, permissions, library_ids, max_streams,
           max_transcodes, max_profiles, max_playback_quality,
           download_allowed, download_transcode_allowed
    FROM buckets
),
created AS (
    INSERT INTO groups (slug, name, description, built_in, permissions,
                        library_ids, max_streams, max_transcodes,
                        max_profiles, max_playback_quality, download_allowed,
                        download_transcode_allowed)
    SELECT 'migrated-policy-' || n,
           'Migrated policy ' || n,
           'Auto-created during the group migration to preserve a pre-existing per-user policy.',
           false,
           COALESCE(permissions, '{}'),
           library_ids,
           COALESCE(max_streams, 6),
           COALESCE(max_transcodes, 2),
           COALESCE(max_profiles, 5),
           COALESCE(max_playback_quality, ''),
           COALESCE(download_allowed, true),
           COALESCE(download_transcode_allowed, false)
    FROM distinct_buckets
    RETURNING id, slug
)
INSERT INTO user_groups (user_id, group_id)
SELECT b.user_id, c.id
FROM buckets b
JOIN created c ON c.slug = 'migrated-policy-' || b.n;

-- Everyone not yet in any group (non-admin, default policy) -> users.
INSERT INTO user_groups (user_id, group_id)
SELECT u.id, g.id
FROM users u
JOIN groups g ON g.slug = 'users'
WHERE NOT EXISTS (SELECT 1 FROM user_groups ug WHERE ug.user_id = u.id);

ALTER TABLE users
    DROP COLUMN role,
    DROP COLUMN permissions,
    DROP COLUMN library_ids,
    DROP COLUMN max_streams,
    DROP COLUMN max_transcodes,
    DROP COLUMN max_profiles,
    DROP COLUMN max_playback_quality,
    DROP COLUMN download_allowed,
    DROP COLUMN download_transcode_allowed;

UPDATE users SET access_policy_revision = access_policy_revision + 1;

INSERT INTO server_settings (key, value)
VALUES ('users.default_group_slugs', '["users"]')
ON CONFLICT (key) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users
    ADD COLUMN role text,
    ADD COLUMN permissions text[] NOT NULL DEFAULT '{}',
    ADD COLUMN library_ids integer[],
    ADD COLUMN max_streams integer NOT NULL DEFAULT 6,
    ADD COLUMN max_transcodes integer NOT NULL DEFAULT 2,
    ADD COLUMN max_profiles integer NOT NULL DEFAULT 5,
    ADD COLUMN max_playback_quality text NOT NULL DEFAULT '',
    ADD COLUMN download_allowed boolean NOT NULL DEFAULT true,
    ADD COLUMN download_transcode_allowed boolean NOT NULL DEFAULT false;

-- Best-effort flattening of effective policy back onto users.
UPDATE users u SET
    role = CASE WHEN EXISTS (
        SELECT 1 FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id
        WHERE ug.user_id = u.id AND 'admin' = ANY(g.permissions)
    ) THEN 'admin' ELSE 'user' END,
    permissions = COALESCE((
        SELECT array_agg(DISTINCT p ORDER BY p)
        FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id, unnest(g.permissions) AS p
        WHERE ug.user_id = u.id AND p <> 'admin'
    ), '{}'),
    library_ids = CASE WHEN EXISTS (
        SELECT 1 FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id
        WHERE ug.user_id = u.id
          AND (g.library_ids IS NULL OR 'admin' = ANY(g.permissions))
    ) THEN NULL ELSE (
        SELECT array_agg(DISTINCT lid ORDER BY lid)
        FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id, unnest(g.library_ids) AS lid
        WHERE ug.user_id = u.id
    ) END,
    max_streams = COALESCE((
        SELECT MAX(g.max_streams) FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id WHERE ug.user_id = u.id), 6),
    max_transcodes = COALESCE((
        SELECT MAX(g.max_transcodes) FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id WHERE ug.user_id = u.id), 2),
    max_profiles = COALESCE((
        SELECT MAX(g.max_profiles) FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id WHERE ug.user_id = u.id), 5),
    download_allowed = EXISTS (
        SELECT 1 FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id
        WHERE ug.user_id = u.id AND g.download_allowed),
    download_transcode_allowed = EXISTS (
        SELECT 1 FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id
        WHERE ug.user_id = u.id AND g.download_transcode_allowed);

DELETE FROM server_settings WHERE key = 'users.default_group_slugs';
DROP TABLE user_groups;
DROP TABLE groups;

UPDATE users SET access_policy_revision = access_policy_revision + 1;
-- +goose StatementEnd
```

Note: the Down path leaves `max_playback_quality` at its default (`''`); a per-user quality flatten requires the Go-side rank order and is deliberately best-effort (documented in the spec).

- [ ] **Step 3: Verify against the local database**

Start services and seed a scenario, then migrate and assert:

```bash
docker compose up -d postgres redis
make migrate-status   # confirm the new migration is pending
```

Seed test users via `psql` (use the connection settings from `docker-compose.yml`; the default local DSN is in `make dev-backend`'s environment or `.env`):

```sql
-- in psql, BEFORE migrate-up:
INSERT INTO users (email, username, password_hash, role, permissions) VALUES
 ('a@test.local', 'admin1',   'x', 'admin', '{}'),
 ('b@test.local', 'default1', 'x', 'user',  '{marker_edit}'),
 ('c@test.local', 'restricted1', 'x', 'user', '{marker_edit}'),
 ('d@test.local', 'restricted2', 'x', 'user', '{marker_edit}');
UPDATE users SET library_ids = '{1,2}', max_streams = 3 WHERE username IN ('restricted1','restricted2');
```

```bash
make migrate-up
```

```sql
-- in psql, AFTER migrate-up — all must hold:
-- 1. admin1 is only in administrators:
SELECT g.slug FROM user_groups ug JOIN groups g ON g.id = ug.group_id
JOIN users u ON u.id = ug.user_id WHERE u.username = 'admin1';
-- expect: administrators

-- 2. default1 is only in users:
SELECT g.slug FROM user_groups ug JOIN groups g ON g.id = ug.group_id
JOIN users u ON u.id = ug.user_id WHERE u.username = 'default1';
-- expect: users

-- 3. restricted1 and restricted2 share ONE migrated group, and are NOT in users:
SELECT u.username, g.slug, g.library_ids, g.max_streams
FROM user_groups ug JOIN groups g ON g.id = ug.group_id
JOIN users u ON u.id = ug.user_id WHERE u.username LIKE 'restricted%';
-- expect: both rows show the same migrated-policy-1 with {1,2} and 3

-- 4. old columns are gone:
SELECT column_name FROM information_schema.columns
WHERE table_name = 'users' AND column_name IN ('role','permissions','library_ids');
-- expect: zero rows
```

Then delete the seeded test users.

- [ ] **Step 4: Commit**

```bash
git add migrations/sql/
git commit -m "feat(auth): group-based permissions schema migration"
```

> **Build note:** from this commit until Task 6 lands, the Go code still selects the dropped columns, so the server won't run against a migrated DB. Tests stay green (no testcontainers). Tasks 4–7 must land on this branch before anything is deployed; do not run `make migrate-up` against a real deployment until the branch is complete.

---

### Task 5: GroupRepository

CRUD, membership, guards, and set-based revision bumps. Guards live here (transactional, race-safe via a `FOR UPDATE` lock on the group row).

**Files:**
- Create: `internal/auth/group_repository.go`
- Test: `internal/auth/group_repository_test.go` (pure parts: slugify)

- [ ] **Step 1: Write the failing slugify test**

Create `internal/auth/group_repository_test.go`:

```go
package auth

import "testing"

func TestSlugifyGroupName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Family", "family"},
		{"Power Users", "power-users"},
		{"  Trimmed  ", "trimmed"},
		{"Ünïcode & Symbols!", "nicode-symbols"},
		{"--multi---dash--", "multi-dash"},
	}
	for _, tc := range cases {
		if got := slugifyGroupName(tc.in); got != tc.want {
			t.Errorf("slugifyGroupName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -run TestSlugifyGroupName -v`
Expected: FAIL — `undefined: slugifyGroupName`

- [ ] **Step 3: Implement the repository**

Create `internal/auth/group_repository.go`:

```go
package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Silo-Server/silo-server/internal/models"
)

// Sentinel errors for group operations.
var (
	ErrGroupNotFound       = errors.New("group not found")
	ErrBuiltInGroup        = errors.New("built-in group cannot be deleted")
	ErrAdminPermRequired   = errors.New("administrators group must keep the admin permission")
	ErrLastAdministrator   = errors.New("cannot remove the last enabled administrator")
)

// GroupRepository provides CRUD and membership operations for groups.
// All mutations bump access_policy_revision for affected users set-based so
// the access resolver invalidates stale scopes.
type GroupRepository struct {
	pool *pgxpool.Pool
}

func NewGroupRepository(pool *pgxpool.Pool) *GroupRepository {
	return &GroupRepository{pool: pool}
}

const groupColumns = `id, slug, name, description, built_in, permissions,
	library_ids, max_streams, max_transcodes, max_profiles,
	max_playback_quality, download_allowed, download_transcode_allowed,
	created_at, updated_at`

func scanGroup(row pgx.Row) (*models.Group, error) {
	var g models.Group
	err := row.Scan(
		&g.ID, &g.Slug, &g.Name, &g.Description, &g.BuiltIn, &g.Permissions,
		&g.LibraryIDs, &g.MaxStreams, &g.MaxTranscodes, &g.MaxProfiles,
		&g.MaxPlaybackQuality, &g.DownloadAllowed, &g.DownloadTranscodeAllowed,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGroupNotFound
		}
		return nil, fmt.Errorf("scanning group: %w", err)
	}
	return &g, nil
}

var slugStripRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugifyGroupName derives a stable slug from a display name.
func slugifyGroupName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugStripRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// GroupWithMemberCount pairs a group with its member count for list views.
type GroupWithMemberCount struct {
	models.Group
	MemberCount int
}

// List returns all groups with member counts, built-ins first.
func (r *GroupRepository) List(ctx context.Context) ([]GroupWithMemberCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+groupColumns+`,
		       (SELECT COUNT(*) FROM user_groups ug WHERE ug.group_id = groups.id) AS member_count
		FROM groups
		ORDER BY built_in DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing groups: %w", err)
	}
	defer rows.Close()

	var out []GroupWithMemberCount
	for rows.Next() {
		var g GroupWithMemberCount
		if err := rows.Scan(
			&g.ID, &g.Slug, &g.Name, &g.Description, &g.BuiltIn, &g.Permissions,
			&g.LibraryIDs, &g.MaxStreams, &g.MaxTranscodes, &g.MaxProfiles,
			&g.MaxPlaybackQuality, &g.DownloadAllowed, &g.DownloadTranscodeAllowed,
			&g.CreatedAt, &g.UpdatedAt, &g.MemberCount,
		); err != nil {
			return nil, fmt.Errorf("scanning group row: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *GroupRepository) GetByID(ctx context.Context, id int) (*models.Group, error) {
	return scanGroup(r.pool.QueryRow(ctx,
		`SELECT `+groupColumns+` FROM groups WHERE id = $1`, id))
}

func (r *GroupRepository) GetBySlug(ctx context.Context, slug string) (*models.Group, error) {
	return scanGroup(r.pool.QueryRow(ctx,
		`SELECT `+groupColumns+` FROM groups WHERE slug = $1`, slug))
}

// GroupsForUser returns the groups a user belongs to.
func (r *GroupRepository) GroupsForUser(ctx context.Context, userID int) ([]models.Group, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+groupColumns+`
		FROM groups
		JOIN user_groups ug ON ug.group_id = groups.id
		WHERE ug.user_id = $1
		ORDER BY groups.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("loading user groups: %w", err)
	}
	defer rows.Close()
	return collectGroups(rows)
}

// GroupsForUsers returns group memberships for many users in one query.
func (r *GroupRepository) GroupsForUsers(ctx context.Context, userIDs []int) (map[int][]models.Group, error) {
	out := make(map[int][]models.Group, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT ug.user_id, `+groupColumns+`
		FROM groups
		JOIN user_groups ug ON ug.group_id = groups.id
		WHERE ug.user_id = ANY($1::int[])
		ORDER BY groups.id`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("loading users' groups: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID int
		var g models.Group
		if err := rows.Scan(
			&userID,
			&g.ID, &g.Slug, &g.Name, &g.Description, &g.BuiltIn, &g.Permissions,
			&g.LibraryIDs, &g.MaxStreams, &g.MaxTranscodes, &g.MaxProfiles,
			&g.MaxPlaybackQuality, &g.DownloadAllowed, &g.DownloadTranscodeAllowed,
			&g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning user group row: %w", err)
		}
		out[userID] = append(out[userID], g)
	}
	return out, rows.Err()
}

func collectGroups(rows pgx.Rows) ([]models.Group, error) {
	var out []models.Group
	for rows.Next() {
		var g models.Group
		if err := rows.Scan(
			&g.ID, &g.Slug, &g.Name, &g.Description, &g.BuiltIn, &g.Permissions,
			&g.LibraryIDs, &g.MaxStreams, &g.MaxTranscodes, &g.MaxProfiles,
			&g.MaxPlaybackQuality, &g.DownloadAllowed, &g.DownloadTranscodeAllowed,
			&g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning group row: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Create inserts a new (non-built-in) group.
func (r *GroupRepository) Create(ctx context.Context, input models.CreateGroupInput) (*models.Group, error) {
	permissions, err := NormalizePermissions(input.Permissions)
	if err != nil {
		return nil, err
	}
	slug := slugifyGroupName(input.Name)
	if slug == "" {
		return nil, fmt.Errorf("group name must contain at least one alphanumeric character")
	}

	cols := []string{"slug", "name", "description", "permissions", "library_ids"}
	args := []any{slug, strings.TrimSpace(input.Name), input.Description, permissions, input.LibraryIDs}
	optional := []struct {
		col string
		val any
		set bool
	}{
		{"max_streams", derefInt(input.MaxStreams), input.MaxStreams != nil},
		{"max_transcodes", derefInt(input.MaxTranscodes), input.MaxTranscodes != nil},
		{"max_profiles", derefInt(input.MaxProfiles), input.MaxProfiles != nil},
		{"max_playback_quality", derefString(input.MaxPlaybackQuality), input.MaxPlaybackQuality != nil},
		{"download_allowed", derefBool(input.DownloadAllowed), input.DownloadAllowed != nil},
		{"download_transcode_allowed", derefBool(input.DownloadTranscodeAllowed), input.DownloadTranscodeAllowed != nil},
	}
	for _, o := range optional {
		if o.set {
			cols = append(cols, o.col)
			args = append(args, o.val)
		}
	}
	placeholders := make([]string, len(args))
	for i := range args {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	query := fmt.Sprintf("INSERT INTO groups (%s) VALUES (%s) RETURNING %s",
		strings.Join(cols, ", "), strings.Join(placeholders, ", "), groupColumns)

	group, err := scanGroup(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, fmt.Errorf("%w: %s", ErrDuplicate, extractConstraint(err))
		}
		return nil, fmt.Errorf("creating group: %w", err)
	}
	return group, nil
}

func derefInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
func derefString(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}
func derefBool(p *bool) any {
	if p == nil {
		return nil
	}
	return *p
}

// Update modifies a group and bumps all members' access_policy_revision when
// a policy field changes. The administrators group cannot lose the admin
// permission.
func (r *GroupRepository) Update(ctx context.Context, id int, input models.UpdateGroupInput) (*models.Group, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning group update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the group row: serializes concurrent policy/membership mutations.
	current, err := scanGroup(tx.QueryRow(ctx,
		`SELECT `+groupColumns+` FROM groups WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}

	if input.Permissions != nil && current.Slug == models.GroupSlugAdministrators {
		hasAdmin := false
		for _, p := range *input.Permissions {
			if p == string(PermissionAdmin) {
				hasAdmin = true
				break
			}
		}
		if !hasAdmin {
			return nil, ErrAdminPermRequired
		}
	}

	setClauses := []string{}
	policyChanged := false
	args := []any{}
	idx := 1
	add := func(col string, val any, policy bool) {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, val)
		idx++
		if policy {
			policyChanged = true
		}
	}
	if input.Name != nil {
		add("name", strings.TrimSpace(*input.Name), false)
	}
	if input.Description != nil {
		add("description", *input.Description, false)
	}
	if input.Permissions != nil {
		permissions, err := NormalizePermissions(*input.Permissions)
		if err != nil {
			return nil, err
		}
		add("permissions", permissions, true)
	}
	if input.LibraryIDs != nil {
		add("library_ids", *input.LibraryIDs, true)
	}
	if input.MaxStreams != nil {
		add("max_streams", *input.MaxStreams, true)
	}
	if input.MaxTranscodes != nil {
		add("max_transcodes", *input.MaxTranscodes, true)
	}
	if input.MaxProfiles != nil {
		add("max_profiles", *input.MaxProfiles, true)
	}
	if input.MaxPlaybackQuality != nil {
		add("max_playback_quality", *input.MaxPlaybackQuality, true)
	}
	if input.DownloadAllowed != nil {
		add("download_allowed", *input.DownloadAllowed, true)
	}
	if input.DownloadTranscodeAllowed != nil {
		add("download_transcode_allowed", *input.DownloadTranscodeAllowed, true)
	}
	if len(setClauses) == 0 {
		return current, nil
	}
	setClauses = append(setClauses, "updated_at = NOW()")

	args = append(args, id)
	query := fmt.Sprintf("UPDATE groups SET %s WHERE id = $%d RETURNING %s",
		strings.Join(setClauses, ", "), idx, groupColumns)
	updated, err := scanGroup(tx.QueryRow(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("updating group: %w", err)
	}

	if policyChanged {
		if err := bumpGroupMemberRevisions(ctx, tx, id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing group update: %w", err)
	}
	return updated, nil
}

// bumpGroupMemberRevisions invalidates all members' resolved scopes in one
// set-based statement (never loop per user — groups can have thousands of
// members).
func bumpGroupMemberRevisions(ctx context.Context, tx pgx.Tx, groupID int) error {
	_, err := tx.Exec(ctx, `
		UPDATE users u
		SET access_policy_revision = access_policy_revision + 1
		FROM user_groups ug
		WHERE ug.user_id = u.id AND ug.group_id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("bumping member policy revisions: %w", err)
	}
	return nil
}

// Delete removes a non-built-in group. Member revisions are bumped before the
// memberships cascade away.
func (r *GroupRepository) Delete(ctx context.Context, id int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning group delete: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	group, err := scanGroup(tx.QueryRow(ctx,
		`SELECT `+groupColumns+` FROM groups WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return err
	}
	if group.BuiltIn {
		return ErrBuiltInGroup
	}
	if err := bumpGroupMemberRevisions(ctx, tx, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM groups WHERE id = $1`, id); err != nil {
		return fmt.Errorf("deleting group: %w", err)
	}
	return tx.Commit(ctx)
}

// GroupMember is one row of a paginated member listing.
type GroupMember struct {
	UserID   int
	Username string
	Email    string
	Enabled  bool
}

// ListMembers returns one page of a group's members plus the total count.
func (r *GroupRepository) ListMembers(ctx context.Context, groupID, offset, limit int) ([]GroupMember, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_groups WHERE group_id = $1`, groupID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting members: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.username, u.email, u.enabled
		FROM user_groups ug
		JOIN users u ON u.id = ug.user_id
		WHERE ug.group_id = $1
		ORDER BY u.username
		OFFSET $2 LIMIT $3`, groupID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("listing members: %w", err)
	}
	defer rows.Close()

	var members []GroupMember
	for rows.Next() {
		var m GroupMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.Email, &m.Enabled); err != nil {
			return nil, 0, fmt.Errorf("scanning member: %w", err)
		}
		members = append(members, m)
	}
	return members, total, rows.Err()
}

// AddMember adds a user to a group (idempotent) and bumps their revision.
func (r *GroupRepository) AddMember(ctx context.Context, groupID, userID int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning add member: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := scanGroup(tx.QueryRow(ctx,
		`SELECT `+groupColumns+` FROM groups WHERE id = $1 FOR UPDATE`, groupID)); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO user_groups (user_id, group_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, userID, groupID)
	if err != nil {
		return fmt.Errorf("adding member: %w", err)
	}
	if tag.RowsAffected() > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE users SET access_policy_revision = access_policy_revision + 1
			WHERE id = $1`, userID); err != nil {
			return fmt.Errorf("bumping member revision: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// RemoveMember removes a user from a group. Removing the last enabled member
// of the administrators group is rejected.
func (r *GroupRepository) RemoveMember(ctx context.Context, groupID, userID int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning remove member: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	group, err := scanGroup(tx.QueryRow(ctx,
		`SELECT `+groupColumns+` FROM groups WHERE id = $1 FOR UPDATE`, groupID))
	if err != nil {
		return err
	}
	if group.Slug == models.GroupSlugAdministrators {
		var remaining int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM user_groups ug
			JOIN users u ON u.id = ug.user_id
			WHERE ug.group_id = $1 AND u.enabled AND ug.user_id <> $2`,
			groupID, userID).Scan(&remaining); err != nil {
			return fmt.Errorf("counting administrators: %w", err)
		}
		if remaining == 0 {
			return ErrLastAdministrator
		}
	}
	tag, err := tx.Exec(ctx,
		`DELETE FROM user_groups WHERE group_id = $1 AND user_id = $2`, groupID, userID)
	if err != nil {
		return fmt.Errorf("removing member: %w", err)
	}
	if tag.RowsAffected() > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE users SET access_policy_revision = access_policy_revision + 1
			WHERE id = $1`, userID); err != nil {
			return fmt.Errorf("bumping member revision: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// ReplaceUserGroups sets a user's memberships to exactly groupIDs, applying
// the last-administrator guard if administrators membership is being removed.
func (r *GroupRepository) ReplaceUserGroups(ctx context.Context, userID int, groupIDs []int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning group replacement: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the administrators group row so concurrent removals serialize.
	var adminGroupID int
	if err := tx.QueryRow(ctx, `
		SELECT id FROM groups WHERE slug = $1 FOR UPDATE`,
		models.GroupSlugAdministrators).Scan(&adminGroupID); err != nil {
		return fmt.Errorf("locking administrators group: %w", err)
	}

	keepingAdmin := false
	for _, id := range groupIDs {
		if id == adminGroupID {
			keepingAdmin = true
			break
		}
	}
	if !keepingAdmin {
		var wasAdmin bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM user_groups WHERE user_id = $1 AND group_id = $2)`,
			userID, adminGroupID).Scan(&wasAdmin); err != nil {
			return fmt.Errorf("checking administrators membership: %w", err)
		}
		if wasAdmin {
			var remaining int
			if err := tx.QueryRow(ctx, `
				SELECT COUNT(*)
				FROM user_groups ug
				JOIN users u ON u.id = ug.user_id
				WHERE ug.group_id = $1 AND u.enabled AND ug.user_id <> $2`,
				adminGroupID, userID).Scan(&remaining); err != nil {
				return fmt.Errorf("counting administrators: %w", err)
			}
			if remaining == 0 {
				return ErrLastAdministrator
			}
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_groups WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clearing memberships: %w", err)
	}
	for _, groupID := range groupIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_groups (user_id, group_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, userID, groupID); err != nil {
			return fmt.Errorf("adding membership %d: %w", groupID, err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users SET access_policy_revision = access_policy_revision + 1
		WHERE id = $1`, userID); err != nil {
		return fmt.Errorf("bumping user revision: %w", err)
	}
	return tx.Commit(ctx)
}
```

- [ ] **Step 4: Build and test**

Run: `go build ./... && go test ./internal/auth/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/group_repository.go internal/auth/group_repository_test.go
git commit -m "feat(auth): group repository with guards and revision bumps"
```

---

### Task 6: User repository, models inputs, auth service, account provisioner, admin user payloads

This is the cutover task: the user repository stops reading the dropped columns and hydrates effective policy from groups; everything that compiled against the old inputs changes with it. One commit at the end.

**Files:**
- Modify: `internal/models/user.go` (remove old fields from inputs; `User` keeps effective fields)
- Modify: `internal/auth/repository.go`
- Modify: `internal/auth/service.go`
- Modify: `internal/auth/account_provisioner.go`
- Modify: `internal/api/handlers/admin.go`
- Tests: `internal/auth/account_provisioner_test.go`, `internal/api/handlers/admin_test.go` (update fakes/payloads)

- [ ] **Step 1: Update `internal/models/user.go`**

In `User`: delete the `Role` field (keep `IsAdmin`, `GroupIDs`, and all effective policy fields — they are now populated by the repository, not columns). Update the comment on `LibraryIDs` to "effective; nil = all libraries".

Replace `CreateUserInput` and `UpdateUserInput`:

```go
// CreateUserInput contains the fields required to create a new user.
// Policy comes from group membership; GroupIDs nil means the caller resolves
// defaults (see AccountProvisioner).
type CreateUserInput struct {
	Email                     string // required
	Username                  string // required
	Password                  string // plaintext, will be bcrypt-hashed
	LocalPasswordLoginEnabled *bool
	GroupIDs                  []int
}

// UpdateUserInput contains optional fields for updating a user.
// Pointer fields: nil means "don't update".
type UpdateUserInput struct {
	Email                     *string
	Username                  *string
	Password                  *string
	LocalPasswordLoginEnabled *bool
	Enabled                   *bool
	GroupIDs                  *[]int
}
```

- [ ] **Step 2: Rewrite `internal/auth/repository.go`**

`UserRepository` gains a groups dependency:

```go
type UserRepository struct {
	pool   *pgxpool.Pool
	groups *GroupRepository
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool, groups: NewGroupRepository(pool)}
}
```

`allColumns` shrinks to identity + account fields:

```go
const allColumns = `id, email, username, password_hash, local_password_login_enabled,
	enabled, access_policy_revision, created_at, updated_at`
```

`scanUser`/`scanUsers` scan only those fields (delete the dropped-column scan targets and the Task 2 interim `IsAdmin` line — hydration now happens in the methods below).

Every getter hydrates effective policy:

```go
func (r *UserRepository) GetByID(ctx context.Context, id int) (*models.User, error) {
	query := `SELECT ` + allColumns + ` FROM users WHERE id = $1`
	user, err := scanUser(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		return nil, err
	}
	return user, r.hydrate(ctx, user)
}

// hydrate loads the user's groups and applies the effective policy union.
func (r *UserRepository) hydrate(ctx context.Context, user *models.User) error {
	groups, err := r.groups.GroupsForUser(ctx, user.ID)
	if err != nil {
		return err
	}
	ApplyEffectivePolicy(user, groups)
	return nil
}
```

Apply the same pattern to `GetByUsername` and `GetByEmail`. `List` hydrates in bulk:

```go
func (r *UserRepository) List(ctx context.Context) ([]*models.User, error) {
	query := `SELECT ` + allColumns + ` FROM users ORDER BY id ASC`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	users, err := scanUsers(rows)
	if err != nil {
		return nil, err
	}
	ids := make([]int, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	byUser, err := r.groups.GroupsForUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		ApplyEffectivePolicy(u, byUser[u.ID])
	}
	return users, nil
}
```

`Create` inserts identity fields only, then memberships, in a transaction:

```go
func (r *UserRepository) Create(ctx context.Context, input models.CreateUserInput) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}
	localPasswordLoginEnabled := true
	if input.LocalPasswordLoginEnabled != nil {
		localPasswordLoginEnabled = *input.LocalPasswordLoginEnabled
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning user create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		INSERT INTO users (email, username, password_hash, local_password_login_enabled)
		VALUES ($1, $2, $3, $4)
		RETURNING `+allColumns,
		NormalizeEmail(input.Email), NormalizeUsername(input.Username),
		string(hash), localPasswordLoginEnabled)
	user, err := scanUser(row)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, fmt.Errorf("%w: %s", ErrDuplicate, extractConstraint(err))
		}
		return nil, fmt.Errorf("creating user: %w", err)
	}
	for _, groupID := range input.GroupIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_groups (user_id, group_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, user.ID, groupID); err != nil {
			return nil, fmt.Errorf("assigning group %d: %w", groupID, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing user create: %w", err)
	}
	return user, r.hydrate(ctx, user)
}
```

`Update` keeps only the identity/account branches (`Email`, `Username`, `Password`, `LocalPasswordLoginEnabled`, `Enabled` with its revision-bump predicate) and delegates memberships:

```go
	// ... after the identity-field UPDATE (or instead of it when only groups changed):
	if input.GroupIDs != nil {
		if err := r.groups.ReplaceUserGroups(ctx, id, *input.GroupIDs); err != nil {
			return err
		}
	}
```

Delete every branch for `Role`, `Permissions`, `LibraryIDs`, `MaxPlaybackQuality`, `MaxStreams`, `MaxTranscodes`, `MaxProfiles`, `DownloadAllowed`, `DownloadTranscodeAllowed`. Remove `DefaultUserPermissions` from `internal/auth/permissions.go` (its only caller was `Create`).

- [ ] **Step 3: Update `internal/auth/service.go`**

`SetupInitialUser` (line ~294): replace `Role: "admin"` in the create input with a post-create membership — the service needs the group repo. Add a `groups *GroupRepository` field to `Service`, set in `NewService` (derive from the same pool the user repo uses — add a `Groups()` accessor on `UserRepository` returning `r.groups` to avoid widening `NewService`'s parameter list):

```go
	user, err := s.accounts.CreateAccount(ctx, CreateAccountInput{ /* identity fields */ })
	if err != nil { ... }
	adminGroup, err := s.users.Groups().GetBySlug(ctx, models.GroupSlugAdministrators)
	if err != nil {
		return nil, nil, fmt.Errorf("loading administrators group: %w", err)
	}
	if err := s.users.Groups().AddMember(ctx, adminGroup.ID, user.ID); err != nil {
		return nil, nil, fmt.Errorf("assigning administrators group: %w", err)
	}
	// reload so the returned user carries effective admin policy
	user, err = s.users.GetByID(ctx, user.ID)
```

`StartImpersonation` (~line 415): `admin.Role != "admin"` → `!admin.IsAdmin`; `target.Role == "admin"` → `target.IsAdmin`.
`validateImpersonator` (~line 540): `impersonator.Role != "admin"` → `!impersonator.IsAdmin`.
`Signup` (~line 330): leave `GroupIDs` nil; the provisioner resolves defaults (next step).

- [ ] **Step 4: Default groups in `internal/auth/account_provisioner.go`**

Add the settings dependency and default resolution:

```go
const DefaultGroupSlugsSettingKey = "users.default_group_slugs"

type GroupResolver interface {
	GetBySlug(ctx context.Context, slug string) (*models.Group, error)
}
```

Extend `AccountProvisioner` with `settings SettingsGetter` and `groups GroupResolver` fields (update `NewAccountProvisioner` and both call sites: `internal/auth/service.go` and `internal/api/handlers/admin.go`). In `CreateAccount`, before `p.users.Create`:

```go
	if input.User.GroupIDs == nil {
		ids, err := p.defaultGroupIDs(ctx)
		if err != nil {
			return nil, err
		}
		input.User.GroupIDs = ids
	}
```

```go
// defaultGroupIDs resolves the configured default group slugs, falling back
// to the built-in users group.
func (p *AccountProvisioner) defaultGroupIDs(ctx context.Context) ([]int, error) {
	slugs := []string{models.GroupSlugUsers}
	if p.settings != nil {
		raw, err := p.settings.Get(ctx, DefaultGroupSlugsSettingKey)
		if err == nil && strings.TrimSpace(raw) != "" {
			var configured []string
			if jsonErr := json.Unmarshal([]byte(raw), &configured); jsonErr == nil && len(configured) > 0 {
				slugs = configured
			}
		}
	}
	ids := make([]int, 0, len(slugs))
	for _, slug := range slugs {
		group, err := p.groups.GetBySlug(ctx, slug)
		if err != nil {
			if errors.Is(err, ErrGroupNotFound) {
				continue // a stale setting must not block signups
			}
			return nil, fmt.Errorf("resolving default group %q: %w", slug, err)
		}
		ids = append(ids, group.ID)
	}
	return ids, nil
}
```

Write a unit test in `account_provisioner_test.go` with a fake `GroupResolver` and fake settings asserting: (a) nil `GroupIDs` resolves to the configured slugs, (b) explicit `GroupIDs` are passed through untouched, (c) unknown slugs are skipped.

- [ ] **Step 5: Update `internal/api/handlers/admin.go`**

`createUserRequest`: delete `Role`, `Permissions`, `LibraryIDs`, `MaxPlaybackQuality`, `MaxStreams`, `MaxTranscodes`, `MaxProfiles`, `DownloadAllowed`, `DownloadTranscodeAllowed`; add `GroupIDs []int \`json:"group_ids"\``.

`updateUserRequest`: same deletions; add `GroupIDs *[]int \`json:"group_ids,omitempty"\``. Delete the now-unused `createStringSliceField`, `updateStringSliceField`, `updateLibraryIDsField` types if nothing else references them (grep first).

`adminUserResponse`: keep all effective fields (they read from the hydrated user), make `Role` derived, and add groups:

```go
type adminGroupRef struct {
	ID   int    `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}
```

In `adminUserResponse` add `Groups []adminGroupRef \`json:"groups"\`` and in `toAdminUserResponse`:

```go
	role := "user"
	if u.IsAdmin {
		role = "admin"
	}
```

…and populate `Role: role`. Groups need names: the handler loads them via the group repo (`GroupsForUsers` for list, `GroupsForUser` for single) — add a `groupRepo *auth.GroupRepository` field to `AdminHandler`, set in `NewAdminHandler` from the same pool. Change `toAdminUserResponse(u *models.User)` to `toAdminUserResponse(u *models.User, groups []models.Group)` and map the refs.

`HandleCreateUser`/`HandleUpdateUser`: map `GroupIDs` through to the inputs; remove all deleted-field plumbing (including the permission/quality validation that moved to groups).

- [ ] **Step 6: Fix remaining compile errors in the auth package and handlers**

Run `go build ./...` and fix every error this cutover surfaces — they will all be references to `models.User.Role` or deleted input fields. Apply these rules:

| Old | New |
|---|---|
| `user.Role == "admin"` | `user.IsAdmin` |
| `user.Role != "admin"` | `!user.IsAdmin` |
| `Role: user.Role` (into a response/claims struct) | derived string: `"admin"`/`"user"` from `IsAdmin` (responses) — claims handled in Task 7 |
| `input.Role = ...` / `Permissions:` etc. in CreateUserInput | group membership via `GroupIDs` |

Do NOT touch `claims.Role` sites yet (Task 7). If a `claims.Role` site needs the user's admin status now, leave it compiling (claims still has `Role` until Task 7).

Also grep for raw SQL touching dropped columns:

```bash
grep -rn "FROM users" --include="*.go" internal/ | grep -vi test
```

Inspect each hit's column list; any query selecting `role`, `permissions`, `library_ids`, `max_streams`, `max_transcodes`, `max_profiles`, `max_playback_quality`, `download_allowed`, or `download_transcode_allowed` from `users` must be rewritten to join `user_groups`/`groups` or to use the hydrated `models.User` instead. (Known candidates: jellycompat batch loaders, admin stats, requests service — verify each.)

- [ ] **Step 7: Update tests, build, run full backend tests**

Update fakes in `internal/api/handlers/admin_test.go` and any test constructing `models.CreateUserInput`/`UpdateUserInput` with removed fields. Then:

Run: `go build ./... && go test ./internal/... -count=1`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add -A internal/
git commit -m "feat(auth): users derive effective policy from groups"
```

---

### Task 7: Claims without Role, RequireAdmin via loaded user, role-check sweep

**Files:**
- Modify: `internal/auth/jwt.go`, `internal/auth/service.go`
- Modify: `internal/api/middleware/auth.go`
- Modify: `internal/api/router.go`
- Modify: every `claims.Role` site (~25, mostly `internal/api/handlers`, `internal/jellycompat`, `internal/requests`)
- Test: `internal/auth/jwt_test.go`, `internal/api/middleware/` tests

- [ ] **Step 1: Remove `Role` from claims and token generators**

`internal/auth/jwt.go`: delete `Role string \`json:"role"\`` from `Claims`. Change signatures:

```go
func (j *JWTService) GenerateAccessToken(userID int, sessionID string) (string, error)
func (j *JWTService) GenerateRefreshToken(userID int, sessionID string) (string, error)
func (j *JWTService) GeneratePluginAccessToken(userID int, sessionID string, ttl time.Duration) (string, error)
```

(drop the `role` parameter and the `Role:` field in each `Claims{...}` literal). Update `jwt_test.go` accordingly.

- [ ] **Step 2: RequireAdmin loads the user**

In `internal/api/middleware/auth.go`, add a user context key + accessor and convert `RequireAdmin` to a method:

```go
// userKey is the context key for the loaded authenticated user.
const userKey contextKey = "user"

// AdminUserLoader loads users for admin authorization checks.
type AdminUserLoader interface {
	GetByID(ctx context.Context, id int) (*models.User, error)
}
```

Add `userLoader AdminUserLoader` to `AuthMiddleware` and its constructor (reuse the same repo passed for API keys; pass it explicitly from `router.go`).

```go
// RequireAdmin enforces that the authenticated user currently holds the
// admin permission. The check is server-side against group-derived policy —
// never against token contents — so revoking admin takes effect on the next
// request. The loaded user is stashed in the context for handlers.
func (am *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaims(r.Context())
		if claims == nil {
			writeUnauthorized(w, "Authentication required")
			return
		}
		user, err := am.userLoader.GetByID(r.Context(), claims.UserID)
		if err != nil || user == nil || !user.Enabled || !user.IsAdmin {
			writeForbidden(w, "Admin access required")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUser returns the user loaded by RequireAdmin, or nil.
func GetUser(ctx context.Context) *models.User {
	user, ok := ctx.Value(userKey).(*models.User)
	if !ok {
		return nil
	}
	return user
}
```

Keep a deprecated-free transition: delete the old package-level `RequireAdmin` and fix `internal/api/router.go` references (`middleware.RequireAdmin` → `authMiddleware.RequireAdmin`).

The API-key path in `RequireAuth` builds `Claims` with `Role:` — delete that field there too.

- [ ] **Step 3: Sweep all remaining `claims.Role` / role-string sites**

```bash
grep -rn "claims.Role\|Claims.Role\|\.Role ==\|\.Role !=" --include="*.go" internal/ | grep -v _test
```

For each hit apply:

| Pattern | Replacement |
|---|---|
| `claims.Role == "admin"` gate in a handler | load the user (most handlers already have a user repo; otherwise use `middleware.GetUser(ctx)` under admin routes) and check `user.IsAdmin` |
| `claims.Role` placed into a response | derive from the loaded user's `IsAdmin` |
| `GenerateAccessToken(id, role, sid)` call | `GenerateAccessToken(id, sid)` |

In `internal/jellycompat` the auth layer loads `models.User` already (`handlers_auth.go`, `auth_api_key.go`) — its role checks become `user.IsAdmin`. In `internal/requests`, same: it consumes `models.User`.

Then run the same grep expecting **zero** non-test hits, and:

```bash
grep -rn '"role"' --include="*.go" internal/ | grep -v _test
```

Remaining hits must be deliberate (derived role in API responses; jellycompat policy mapping). Update all affected tests.

- [ ] **Step 4: Build and test**

Run: `go build ./... && go test ./internal/... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A internal/
git commit -m "feat(auth): server-side admin checks; remove role from JWT claims"
```

---

### Task 8: Jellyfin compat UserPolicy from effective policy

**Files:**
- Modify: `internal/jellycompat/handlers_auth.go` (the `UserPolicy` construction around lines 216–239)
- Test: `internal/jellycompat/auth_test.go`

- [ ] **Step 1: Write/extend the failing test**

In `internal/jellycompat/auth_test.go`, add a test that builds the policy for a hydrated user and asserts the mapping:

```go
func TestUserPolicyFromEffectivePolicy(t *testing.T) {
	u := &models.User{
		ID: 7, Enabled: true, IsAdmin: false,
		LibraryIDs:      []int{3, 5},
		DownloadAllowed: false,
	}
	policy := buildUserPolicy(u) // extract the construction into this helper
	if policy.IsAdministrator {
		t.Error("non-admin must not be administrator")
	}
	if policy.EnableAllFolders {
		t.Error("restricted user must not see all folders")
	}
	if policy.EnableContentDownloading {
		t.Error("downloads disabled must map through")
	}

	admin := &models.User{ID: 1, Enabled: true, IsAdmin: true, LibraryIDs: nil, DownloadAllowed: true}
	adminPolicy := buildUserPolicy(admin)
	if !adminPolicy.IsAdministrator || !adminPolicy.EnableAllFolders || !adminPolicy.EnableContentDownloading {
		t.Error("admin policy must be unrestricted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/jellycompat/ -run TestUserPolicyFromEffectivePolicy -v`
Expected: FAIL — `undefined: buildUserPolicy`

- [ ] **Step 3: Implement**

Extract the inline `UserPolicy` literal in `handlers_auth.go` into:

```go
// buildUserPolicy maps Silo's group-derived effective policy onto the
// Jellyfin UserPolicy shape.
func buildUserPolicy(u *models.User) jellyUserPolicy {
	policy := jellyUserPolicy{ // use the actual struct type name from handlers_auth.go
		IsAdministrator:          u.IsAdmin,
		EnableContentDownloading: u.DownloadAllowed,
		EnableAllFolders:         u.LibraryIDs == nil,
		// ...keep every other field exactly as the current literal sets it...
	}
	if u.LibraryIDs != nil {
		policy.EnabledFolders = libraryIDsToFolderGUIDs(u.LibraryIDs) // reuse the existing library->GUID helper used by access_filter.go
	}
	return policy
}
```

Use the existing library-ID→Jellyfin-folder-GUID conversion already used by `internal/jellycompat/access_filter.go` (find it with `grep -n "Guid\|folderID" internal/jellycompat/access_filter.go`); do not invent a new mapping. Replace both construction sites (lines ~216 and wherever else `IsAdministrator:` is set) with `buildUserPolicy(user)`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/jellycompat/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/jellycompat/
git commit -m "feat(jellycompat): derive UserPolicy from group-based effective policy"
```

---### Task 9: Admin groups API

**Files:**
- Create: `internal/api/handlers/admin_groups.go`
- Test: `internal/api/handlers/admin_groups_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write failing handler tests**

Create `internal/api/handlers/admin_groups_test.go` following the fake-based pattern in `admin_test.go`. Define a `fakeGroupRepo` implementing the `GroupStore` interface (defined in Step 2) and test at minimum:

```go
// - GET list returns groups with member_count
// - POST create returns 201 and the created group
// - PATCH on administrators removing "admin" -> 409 with code "admin_permission_required"
//   (fake returns auth.ErrAdminPermRequired)
// - DELETE built-in -> 409 with code "built_in_group" (fake returns auth.ErrBuiltInGroup)
// - DELETE member that is last admin -> 409 with code "last_administrator"
//   (fake returns auth.ErrLastAdministrator)
// - GET members returns {members, total, offset, limit}
```

Write these as real table-driven httptest cases — construct the handler with the fake, build requests with chi route contexts (copy the pattern used by existing admin handler tests).

- [ ] **Step 2: Run tests to verify they fail, then implement**

Run: `go test ./internal/api/handlers/ -run TestAdminGroups -v` → FAIL (undefined types)

Create `internal/api/handlers/admin_groups.go`:

```go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Silo-Server/silo-server/internal/auth"
	"github.com/Silo-Server/silo-server/internal/models"
)

// GroupStore is the group repository surface the admin API needs.
type GroupStore interface {
	List(ctx context.Context) ([]auth.GroupWithMemberCount, error)
	GetByID(ctx context.Context, id int) (*models.Group, error)
	Create(ctx context.Context, input models.CreateGroupInput) (*models.Group, error)
	Update(ctx context.Context, id int, input models.UpdateGroupInput) (*models.Group, error)
	Delete(ctx context.Context, id int) error
	ListMembers(ctx context.Context, groupID, offset, limit int) ([]auth.GroupMember, int, error)
	AddMember(ctx context.Context, groupID, userID int) error
	RemoveMember(ctx context.Context, groupID, userID int) error
}

// AdminGroupsHandler serves /admin/groups endpoints.
type AdminGroupsHandler struct {
	groups GroupStore
}

func NewAdminGroupsHandler(groups GroupStore) *AdminGroupsHandler {
	return &AdminGroupsHandler{groups: groups}
}

type adminGroupResponse struct {
	ID                       int       `json:"id"`
	Slug                     string    `json:"slug"`
	Name                     string    `json:"name"`
	Description              string    `json:"description"`
	BuiltIn                  bool      `json:"built_in"`
	Permissions              []string  `json:"permissions"`
	LibraryIDs               []int     `json:"library_ids"` // null = all libraries
	MaxStreams               int       `json:"max_streams"`
	MaxTranscodes            int       `json:"max_transcodes"`
	MaxProfiles              int       `json:"max_profiles"`
	MaxPlaybackQuality       string    `json:"max_playback_quality"`
	DownloadAllowed          bool      `json:"download_allowed"`
	DownloadTranscodeAllowed bool      `json:"download_transcode_allowed"`
	MemberCount              *int      `json:"member_count,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

func toAdminGroupResponse(g *models.Group, memberCount *int) adminGroupResponse {
	return adminGroupResponse{
		ID: g.ID, Slug: g.Slug, Name: g.Name, Description: g.Description,
		BuiltIn: g.BuiltIn, Permissions: append([]string{}, g.Permissions...),
		LibraryIDs: g.LibraryIDs, MaxStreams: g.MaxStreams,
		MaxTranscodes: g.MaxTranscodes, MaxProfiles: g.MaxProfiles,
		MaxPlaybackQuality: g.MaxPlaybackQuality,
		DownloadAllowed:    g.DownloadAllowed,
		DownloadTranscodeAllowed: g.DownloadTranscodeAllowed,
		MemberCount: memberCount, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt,
	}
}

type createGroupRequest struct {
	Name                     string   `json:"name"`
	Description              string   `json:"description"`
	Permissions              []string `json:"permissions"`
	LibraryIDs               []int    `json:"library_ids"` // null = all
	MaxStreams               *int     `json:"max_streams,omitempty"`
	MaxTranscodes            *int     `json:"max_transcodes,omitempty"`
	MaxProfiles              *int     `json:"max_profiles,omitempty"`
	MaxPlaybackQuality       *string  `json:"max_playback_quality,omitempty"`
	DownloadAllowed          *bool    `json:"download_allowed,omitempty"`
	DownloadTranscodeAllowed *bool    `json:"download_transcode_allowed,omitempty"`
}

type updateGroupRequest struct {
	Name                     *string                `json:"name,omitempty"`
	Description              *string                `json:"description,omitempty"`
	Permissions              updateStringSliceField `json:"permissions,omitempty"`
	LibraryIDs               updateLibraryIDsField  `json:"library_ids,omitempty"`
	MaxStreams               *int                   `json:"max_streams,omitempty"`
	MaxTranscodes            *int                   `json:"max_transcodes,omitempty"`
	MaxProfiles              *int                   `json:"max_profiles,omitempty"`
	MaxPlaybackQuality       *string                `json:"max_playback_quality,omitempty"`
	DownloadAllowed          *bool                  `json:"download_allowed,omitempty"`
	DownloadTranscodeAllowed *bool                  `json:"download_transcode_allowed,omitempty"`
}
```

(If `updateStringSliceField`/`updateLibraryIDsField` were deleted in Task 6, restore them here — their set-vs-null semantics are exactly what group library/permission updates need.)

Handlers (write each in full; the shapes below):

```go
// HandleListGroups handles GET /admin/groups.
// HandleGetGroup handles GET /admin/groups/{id} (member count included).
// HandleCreateGroup handles POST /admin/groups -> 201.
// HandleUpdateGroup handles PATCH /admin/groups/{id}.
// HandleDeleteGroup handles DELETE /admin/groups/{id} -> 204.
// HandleListGroupMembers handles GET /admin/groups/{id}/members?offset=&limit=
//   -> {"members": [...], "total": n, "offset": o, "limit": l}
// HandleAddGroupMember handles PUT /admin/groups/{id}/members/{userID} -> 204.
// HandleRemoveGroupMember handles DELETE /admin/groups/{id}/members/{userID} -> 204.
```

Shared error mapping helper used by every mutation handler:

```go
func writeGroupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrGroupNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Group not found")
	case errors.Is(err, auth.ErrBuiltInGroup):
		writeError(w, http.StatusConflict, "built_in_group", "Built-in groups cannot be deleted")
	case errors.Is(err, auth.ErrAdminPermRequired):
		writeError(w, http.StatusConflict, "admin_permission_required", "The administrators group must keep the admin permission")
	case errors.Is(err, auth.ErrLastAdministrator):
		writeError(w, http.StatusConflict, "last_administrator", "Cannot remove the last enabled administrator")
	case errors.Is(err, auth.ErrDuplicate):
		writeError(w, http.StatusConflict, "duplicate", "A group with that name already exists")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Group operation failed")
	}
}
```

Member listing parses `offset`/`limit` with `strconv.Atoi`, defaulting to 0/50.

- [ ] **Step 3: Wire routes in `internal/api/router.go`**

Next to the existing admin user routes (~line 1943), inside the same admin-guarded router group:

```go
	adminGroupsHandler := handlers.NewAdminGroupsHandler(auth.NewGroupRepository(deps.DB))
	r.Get("/groups", adminGroupsHandler.HandleListGroups)
	r.Post("/groups", adminGroupsHandler.HandleCreateGroup)
	r.Get("/groups/{id}", adminGroupsHandler.HandleGetGroup)
	r.Patch("/groups/{id}", adminGroupsHandler.HandleUpdateGroup)
	r.Delete("/groups/{id}", adminGroupsHandler.HandleDeleteGroup)
	r.Get("/groups/{id}/members", adminGroupsHandler.HandleListGroupMembers)
	r.Put("/groups/{id}/members/{userID}", adminGroupsHandler.HandleAddGroupMember)
	r.Delete("/groups/{id}/members/{userID}", adminGroupsHandler.HandleRemoveGroupMember)
```

- [ ] **Step 4: Run tests**

Run: `go build ./... && go test ./internal/api/... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/
git commit -m "feat(api): admin group management endpoints"
```

---

### Task 10: End-to-end backend verification against the migrated DB

- [ ] **Step 1: Run the full backend suite and lint**

```bash
go test ./... -count=1
make lint
```

Expected: PASS / no findings.

- [ ] **Step 2: Boot the server against the migrated local DB**

```bash
docker compose up -d postgres redis
make migrate-up        # if not already applied in Task 4
make dev-backend
```

Manual smoke (new terminal, with an admin token from login):

```bash
# login still works, role claim gone but /me reports derived role
curl -s -X POST localhost:8080/api/v1/auth/login -d '{"username":"<admin>","password":"<pw>"}' -H 'Content-Type: application/json'
# groups list shows the two built-ins
curl -s localhost:8080/api/admin/groups -H "Authorization: Bearer $TOKEN"
# deleting administrators is rejected
curl -s -X DELETE localhost:8080/api/admin/groups/1 -H "Authorization: Bearer $TOKEN" -i   # expect 409
```

(Adjust paths to the actual route prefixes in `internal/api/router.go`.)

- [ ] **Step 3: Commit any fixes, then commit a checkpoint**

```bash
git add -A
git commit -m "test(auth): backend verification fixes for group-based permissions"
```

(Skip the commit if nothing changed.)

---

### Task 11: Frontend — types and query hooks

Follow the existing patterns in `web/src/hooks/queries/admin/users.ts` and the types module imported as `@/api/types` (read both before writing).

**Files:**
- Modify: the `@/api/types` module — add group types; update `AdminUser`, `CreateUserRequest`, `UpdateUserRequest`
- Create: `web/src/hooks/queries/admin/groups.ts`

- [ ] **Step 1: Add types** (match the backend JSON exactly):

```typescript
export interface AdminGroup {
  id: number;
  slug: string;
  name: string;
  description: string;
  built_in: boolean;
  permissions: string[];
  library_ids: number[] | null; // null = all libraries
  max_streams: number;
  max_transcodes: number;
  max_profiles: number;
  max_playback_quality: string;
  download_allowed: boolean;
  download_transcode_allowed: boolean;
  member_count?: number;
  created_at: string;
  updated_at: string;
}

export interface CreateGroupRequest {
  name: string;
  description?: string;
  permissions: string[];
  library_ids: number[] | null;
  max_streams?: number;
  max_transcodes?: number;
  max_profiles?: number;
  max_playback_quality?: string;
  download_allowed?: boolean;
  download_transcode_allowed?: boolean;
}

export type UpdateGroupRequest = Partial<CreateGroupRequest>;

export interface GroupMember {
  user_id: number;
  username: string;
  email: string;
  enabled: boolean;
}

export interface GroupMembersPage {
  members: GroupMember[];
  total: number;
  offset: number;
  limit: number;
}

export interface AdminGroupRef {
  id: number;
  slug: string;
  name: string;
}
```

Update `AdminUser`: remove the per-user policy request fields from `CreateUserRequest`/`UpdateUserRequest`, add `group_ids: number[]` / `group_ids?: number[]`; on `AdminUser` add `groups: AdminGroupRef[]` (the effective policy fields stay — the backend still returns them read-only).

- [ ] **Step 2: Create `web/src/hooks/queries/admin/groups.ts`**

Mirror `users.ts` exactly (same query-client import, same fetch wrapper, same key conventions). Hooks: `useAdminGroups()`, `useAdminGroup(id)`, `useGroupMembers(id, offset, limit)`, `useCreateGroup()`, `useUpdateGroup()`, `useDeleteGroup()`, `useAddGroupMember()`, `useRemoveGroupMember()`. Mutations invalidate `["admin", "groups"]` and `["admin", "users"]` (membership changes alter user rows).

- [ ] **Step 3: Lint and commit**

```bash
cd web && pnpm run lint && pnpm run format:check && cd ..
git add web/src/
git commit -m "feat(web): group types and admin query hooks"
```

---

### Task 12: Frontend — Groups admin page

**Files:**
- Create: `web/src/pages/AdminGroups.tsx`
- Modify: the admin route registration + admin nav (find with `grep -rn "AdminUsers" web/src --include="*.tsx" -l` — register `/admin/groups` the same way `/admin/users` is registered, and add a "Groups" nav item beside "Users")

- [ ] **Step 1: Build the page**

`AdminGroups.tsx` structure (reuse the component vocabulary of `AdminUsers.tsx` — `Table`, `Dialog`, `Badge`, `Button`, `Input`, `Switch`, `LibraryAccessSelector`, `ConfirmDialog`, playback-quality preset helpers):

- **List view:** table of groups — Name (+ "Built-in" `Badge` when `built_in`), member count, permissions summary, libraries summary ("All libraries" when `library_ids === null`, else count), streams/transcodes, quality. Row actions: Edit, Delete (delete button disabled with tooltip for built-ins).
- **Create/Edit dialog** with three sections:
  1. *Permissions:* one checkbox per permission (`admin`, `marker_edit`, `metadata_curation`); the `admin` checkbox is disabled (checked) when editing the `administrators` group.
  2. *Library access:* `LibraryAccessSelector` with an "All libraries" toggle mapping to `library_ids: null`.
  3. *Limits:* numeric inputs for max streams/transcodes/profiles, quality `Select` using `PLAYBACK_QUALITY_OPTIONS`, switches for downloads.
- **Members section** (inside Edit dialog or a detail drawer): paginated member table driven by `useGroupMembers(id, offset, 50)` with Previous/Next, plus a user-search add flow (reuse the user list from `useAdminUsers()` filtered client-side by the search input — acceptable; the user list endpoint is already loaded on the Users page) and a remove button per row. Surface backend 409 errors (`last_administrator`) as a toast/inline error, not a crash.

- [ ] **Step 2: Register route + nav**

Add the lazy route for `/admin/groups` exactly parallel to `/admin/users`, and the nav entry (look at how the admin sidebar/nav lists Users — same file pattern).

- [ ] **Step 3: Verify, lint, commit**

```bash
cd web && pnpm run lint && pnpm run format:check && cd ..
make dev-frontend   # manual check: groups list renders, create/edit/delete flows work against dev backend
git add web/src/
git commit -m "feat(web): admin groups management page"
```

---

### Task 13: Frontend — rework Users admin pages

**Files:**
- Modify: `web/src/pages/AdminUsers.tsx`
- Modify: `web/src/pages/AdminUserDetail.tsx`

- [ ] **Step 1: Replace per-user policy controls with group membership**

In both create and edit user forms:

- **Remove:** role select, permissions checkboxes, `LibraryAccessSelector`, max streams/transcodes/profiles inputs, playback quality select, download switches — every control that wrote the deleted request fields.
- **Add:** a group multi-select (checkbox list fed by `useAdminGroups()`; groups are few, no pagination needed) writing `group_ids`.
- **Add:** a read-only **Effective policy** summary panel rendering the user's effective values returned by the API (permissions badges, "All libraries"/library count, streams/transcodes/profiles, quality, download flags) with the caption "Derived from group membership". This is the admin's answer to "why can this user do X".
- The user table's Role column now renders from the derived `role` field (unchanged JSON shape) — verify it still displays; show group names as badges in a new column.

- [ ] **Step 2: Lint, typecheck, manual verify**

```bash
cd web && pnpm run lint && pnpm run format:check && cd ..
```

Manual: create a user with groups, edit memberships, confirm the effective panel updates after save, confirm removing the last admin's administrators membership surfaces the 409 message.

- [ ] **Step 3: Commit**

```bash
git add web/src/
git commit -m "feat(web): group membership controls on user admin pages"
```

---

### Task 14: Final verification and merge prep

- [ ] **Step 1: Full verification suite**

```bash
go build ./... && go test ./... -count=1
make lint
cd web && pnpm run lint && pnpm run format:check && cd ..
make verify-local-paths
```

Expected: all pass.

- [ ] **Step 2: Residual-reference sweep**

```bash
grep -rn '\.Role\b' --include="*.go" internal/ | grep -v _test | grep -v RateTier
grep -rn 'max_streams\|download_allowed\|library_ids' --include="*.go" internal/ | grep "users\b" | grep -v _test
```

Expected: zero hits referencing the dropped `users` columns or `models.User.Role`.

- [ ] **Step 3: Multi-repo follow-up notes**

Open follow-up issues (do not implement here):
- `silo-android` / `silo-apple`: surface group membership in account UI (derived `role` keeps current builds working).
- Invite-code default-group override.
- Audit plugin SDK surface for any role assumptions (coordination with `silo-plugin-sdk`).

- [ ] **Step 4: Commit any final fixes; hand off via finishing-a-development-branch skill**

---

## Self-Review Notes (resolved during plan writing)

- The spec's original migration step 2 ("all users → users") would have erased outlier library restrictions via the permissive union; the spec was corrected (commit `9fdc31b1`) and Task 4 implements the corrected bucketing.
- `users.enabled` and `access_policy_revision` deliberately remain user-level columns.
- Between Task 4 (migration) and Task 6 (repository cutover) the server cannot run against a migrated DB; the plan keeps this window inside one branch and defers any deployment until Task 10 verification.
