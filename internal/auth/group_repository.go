package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/unicode/norm"

	"github.com/Silo-Server/silo-server/internal/models"
)

// Sentinel errors for group operations.
var (
	ErrGroupNotFound     = errors.New("group not found")
	ErrBuiltInGroup      = errors.New("built-in group cannot be deleted")
	ErrAdminPermRequired = errors.New("administrators group must keep the admin permission")
	ErrLastAdministrator = errors.New("cannot remove the last enabled administrator")
	ErrUnknownGroup      = errors.New("unknown group")
	ErrUnknownUser       = errors.New("unknown user")
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

// groupColumns is table-qualified so it stays unambiguous in queries that
// join user_groups (which also has a created_at column). Qualified names are
// valid in plain SELECTs and in INSERT/UPDATE ... RETURNING alike.
const groupColumns = `groups.id, groups.slug, groups.name, groups.description,
	groups.built_in, groups.permissions, groups.library_ids, groups.max_streams,
	groups.max_transcodes, groups.max_profiles, groups.max_playback_quality,
	groups.download_allowed, groups.download_transcode_allowed,
	groups.created_at, groups.updated_at`

func scanGroupFields(row pgx.Row, g *models.Group, extra ...any) error {
	dest := []any{
		&g.ID, &g.Slug, &g.Name, &g.Description, &g.BuiltIn, &g.Permissions,
		&g.LibraryIDs, &g.MaxStreams, &g.MaxTranscodes, &g.MaxProfiles,
		&g.MaxPlaybackQuality, &g.DownloadAllowed, &g.DownloadTranscodeAllowed,
		&g.CreatedAt, &g.UpdatedAt,
	}
	dest = append(dest, extra...)
	return row.Scan(dest...)
}

func scanGroup(row pgx.Row) (*models.Group, error) {
	var g models.Group
	if err := scanGroupFields(row, &g); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGroupNotFound
		}
		return nil, fmt.Errorf("scanning group: %w", err)
	}
	return &g, nil
}

var slugStripRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugifyGroupName derives a stable slug from a display name.
// Accented characters are decomposed via NFD and their combining marks are
// stripped so that e.g. "ï" → "i" before the ASCII-only filter is applied.
func slugifyGroupName(name string) string {
	// NFD-decompose so accented letters split into base + combining mark.
	decomposed := norm.NFD.String(strings.ToLower(strings.TrimSpace(name)))
	// Drop combining (diacritic) runes; keep everything else for the regex.
	var b strings.Builder
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	s := slugStripRe.ReplaceAllString(b.String(), "-")
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
		if err := scanGroupFields(rows, &g.Group, &g.MemberCount); err != nil {
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

	var out []models.Group
	for rows.Next() {
		var g models.Group
		if err := scanGroupFields(rows, &g); err != nil {
			return nil, fmt.Errorf("scanning group row: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// GroupsForUsers returns group memberships for many users in one query.
func (r *GroupRepository) GroupsForUsers(ctx context.Context, userIDs []int) (map[int][]models.Group, error) {
	out := make(map[int][]models.Group, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+groupColumns+`, ug.user_id
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
		if err := scanGroupFields(rows, &g, &userID); err != nil {
			return nil, fmt.Errorf("scanning user group row: %w", err)
		}
		out[userID] = append(out[userID], g)
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
	type optionalCol struct {
		col string
		val any
		set bool
	}
	optional := []optionalCol{
		{"max_streams", derefAny(input.MaxStreams), input.MaxStreams != nil},
		{"max_transcodes", derefAny(input.MaxTranscodes), input.MaxTranscodes != nil},
		{"max_profiles", derefAny(input.MaxProfiles), input.MaxProfiles != nil},
		{"max_playback_quality", derefAny(input.MaxPlaybackQuality), input.MaxPlaybackQuality != nil},
		{"download_allowed", derefAny(input.DownloadAllowed), input.DownloadAllowed != nil},
		{"download_transcode_allowed", derefAny(input.DownloadTranscodeAllowed), input.DownloadTranscodeAllowed != nil},
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

func derefAny[T any](p *T) any {
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

	var normalizedPerms []string
	if input.Permissions != nil {
		var err error
		normalizedPerms, err = NormalizePermissions(*input.Permissions)
		if err != nil {
			return nil, err
		}
		if current.Slug == models.GroupSlugAdministrators {
			hasAdmin := false
			for _, p := range normalizedPerms {
				if p == string(PermissionAdmin) {
					hasAdmin = true
					break
				}
			}
			if !hasAdmin {
				return nil, ErrAdminPermRequired
			}
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
		add("permissions", normalizedPerms, true)
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

// MemberCount returns the number of users in a group.
func (r *GroupRepository) MemberCount(ctx context.Context, groupID int) (int, error) {
	var count int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_groups WHERE group_id = $1`, groupID).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting group members: %w", err)
	}
	return count, nil
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
	if offset < 0 {
		offset = 0
	}
	total, err := r.MemberCount(ctx, groupID)
	if err != nil {
		return nil, 0, err
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
		if isForeignKeyError(err) {
			return ErrUnknownUser
		}
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

// lastEnabledAdministratorGuard returns ErrLastAdministrator when userID is an
// enabled member of the administrators group and no other enabled member
// exists, i.e. when removing, disabling, or deleting userID would leave the
// server without an enabled administrator. It locks the administrators group
// row (FOR UPDATE) so concurrent membership/disable/delete mutations
// serialize; re-locking the same row later in the same transaction is a no-op.
func lastEnabledAdministratorGuard(ctx context.Context, tx pgx.Tx, excludingUserID int) error {
	var adminGroupID int
	if err := tx.QueryRow(ctx, `
		SELECT id FROM groups WHERE slug = $1 FOR UPDATE`,
		models.GroupSlugAdministrators).Scan(&adminGroupID); err != nil {
		return fmt.Errorf("locking administrators group: %w", err)
	}

	// Only an enabled administrator can be the last enabled administrator;
	// mutating a non-member or disabled member never breaks the invariant.
	var enabledMember bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_groups ug
			JOIN users u ON u.id = ug.user_id
			WHERE ug.group_id = $1 AND ug.user_id = $2 AND u.enabled)`,
		adminGroupID, excludingUserID).Scan(&enabledMember); err != nil {
		return fmt.Errorf("checking administrators membership: %w", err)
	}
	if !enabledMember {
		return nil
	}

	var remaining int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM user_groups ug
		JOIN users u ON u.id = ug.user_id
		WHERE ug.group_id = $1 AND u.enabled AND ug.user_id <> $2`,
		adminGroupID, excludingUserID).Scan(&remaining); err != nil {
		return fmt.Errorf("counting administrators: %w", err)
	}
	if remaining == 0 {
		return ErrLastAdministrator
	}
	return nil
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
		if err := lastEnabledAdministratorGuard(ctx, tx, userID); err != nil {
			return err
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

	if err := r.replaceUserGroupsTx(ctx, tx, userID, groupIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// replaceUserGroupsTx performs the membership replacement inside the caller's
// transaction so it can be composed with other writes atomically.
func (r *GroupRepository) replaceUserGroupsTx(ctx context.Context, tx pgx.Tx, userID int, groupIDs []int) error {
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
		if err := lastEnabledAdministratorGuard(ctx, tx, userID); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_groups WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clearing memberships: %w", err)
	}
	if len(groupIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_groups (user_id, group_id)
			SELECT $1, unnest($2::int[])
			ON CONFLICT DO NOTHING`, userID, groupIDs); err != nil {
			if isForeignKeyError(err) {
				return fmt.Errorf("%w", ErrUnknownGroup)
			}
			return fmt.Errorf("adding memberships: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users SET access_policy_revision = access_policy_revision + 1
		WHERE id = $1`, userID); err != nil {
		return fmt.Errorf("bumping user revision: %w", err)
	}
	return nil
}
