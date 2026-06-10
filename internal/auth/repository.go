package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/Silo-Server/silo-server/internal/models"
)

// Sentinel errors for repository operations.
var (
	ErrNotFound  = errors.New("user not found")
	ErrDuplicate = errors.New("duplicate user")
)

// IsNotFound returns true if the error is a "not found" error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsDuplicate returns true if the error is a "duplicate" error.
func IsDuplicate(err error) bool {
	return errors.Is(err, ErrDuplicate)
}

// CheckPassword verifies a plaintext password against the user's bcrypt hash.
// This is a standalone function, not a repository method.
func CheckPassword(user *models.User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// UserRepository provides CRUD operations for the users table. Effective
// access policy is derived from group memberships when users are loaded.
type UserRepository struct {
	pool   *pgxpool.Pool
	groups *GroupRepository
}

// NewUserRepository creates a new UserRepository backed by the given pool.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool, groups: NewGroupRepository(pool)}
}

// Groups exposes the group repository sharing this repository's pool.
func (r *UserRepository) Groups() *GroupRepository { return r.groups }

// allColumns is the list of columns returned by all SELECT queries.
// Kept in one place so scanUser stays in sync.
const allColumns = `id, email, username, password_hash, local_password_login_enabled, enabled,
	access_policy_revision, created_at, updated_at`

// scanUser scans a single row into a *models.User.
func scanUser(row pgx.Row) (*models.User, error) {
	var u models.User
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.Username,
		&u.PasswordHash,
		&u.LocalPasswordLoginEnabled,
		&u.Enabled,
		&u.AccessPolicyRevision,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning user: %w", err)
	}
	return &u, nil
}

// scanUsers scans multiple rows into a []*models.User slice.
func scanUsers(rows pgx.Rows) ([]*models.User, error) {
	var users []*models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning user row: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user rows: %w", err)
	}
	return users, nil
}

// hydrate loads the user's group memberships and applies the effective policy.
func (r *UserRepository) hydrate(ctx context.Context, u *models.User) error {
	groups, err := r.groups.GroupsForUser(ctx, u.ID)
	if err != nil {
		return err
	}
	ApplyEffectivePolicy(u, groups)
	return nil
}

// Create inserts a new user with a bcrypt-hashed password and the given group
// memberships, then returns the created user with its effective policy.
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

	var id int
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, username, password_hash, local_password_login_enabled)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		NormalizeEmail(input.Email),
		NormalizeUsername(input.Username),
		string(hash),
		localPasswordLoginEnabled,
	).Scan(&id)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, fmt.Errorf("%w: %s", ErrDuplicate, extractConstraint(err))
		}
		return nil, fmt.Errorf("creating user: %w", err)
	}

	for _, groupID := range input.GroupIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_groups (user_id, group_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, id, groupID); err != nil {
			if isForeignKeyError(err) {
				return nil, fmt.Errorf("%w: group %d", ErrUnknownGroup, groupID)
			}
			return nil, fmt.Errorf("adding membership %d: %w", groupID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing user create: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a user by their numeric ID.
func (r *UserRepository) GetByID(ctx context.Context, id int) (*models.User, error) {
	query := `SELECT ` + allColumns + ` FROM users WHERE id = $1`
	user, err := scanUser(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		return nil, err
	}
	if err := r.hydrate(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetByUsername retrieves a user by their username (case-insensitive).
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `SELECT ` + allColumns + ` FROM users WHERE username = $1`
	user, err := scanUser(r.pool.QueryRow(ctx, query, NormalizeUsername(username)))
	if err != nil {
		return nil, err
	}
	if err := r.hydrate(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetByEmail retrieves a user by their email address (case-insensitive).
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT ` + allColumns + ` FROM users WHERE email = $1`
	user, err := scanUser(r.pool.QueryRow(ctx, query, NormalizeEmail(email)))
	if err != nil {
		return nil, err
	}
	if err := r.hydrate(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// Update modifies a user's fields. Only non-nil fields in the input are updated.
// If the input contains a Password, it is bcrypt-hashed before storage. A
// non-nil GroupIDs replaces the user's group memberships.
func (r *UserRepository) Update(ctx context.Context, id int, input models.UpdateUserInput) error {
	setClauses := []string{}
	accessPolicyPredicates := []string{}
	args := []any{}
	argIndex := 1

	if input.Email != nil {
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", argIndex))
		args = append(args, NormalizeEmail(*input.Email))
		argIndex++
	}
	if input.Username != nil {
		setClauses = append(setClauses, fmt.Sprintf("username = $%d", argIndex))
		args = append(args, NormalizeUsername(*input.Username))
		argIndex++
	}
	if input.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hashing password: %w", err)
		}
		setClauses = append(setClauses, fmt.Sprintf("password_hash = $%d", argIndex))
		args = append(args, string(hash))
		argIndex++
	}
	if input.LocalPasswordLoginEnabled != nil {
		setClauses = append(setClauses, fmt.Sprintf("local_password_login_enabled = $%d", argIndex))
		args = append(args, *input.LocalPasswordLoginEnabled)
		argIndex++
	}
	if input.Enabled != nil {
		setClauses = append(setClauses, fmt.Sprintf("enabled = $%d", argIndex))
		accessPolicyPredicates = append(accessPolicyPredicates, fmt.Sprintf("enabled IS DISTINCT FROM $%d", argIndex))
		args = append(args, *input.Enabled)
		argIndex++
	}

	// Identity and membership writes share one transaction so a failure in
	// either (e.g. ErrLastAdministrator) leaves the user fully unchanged.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning user update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if len(setClauses) == 0 {
		// Nothing to update on the users row; still verify the user exists.
		var exists int
		if err := tx.QueryRow(ctx, `SELECT 1 FROM users WHERE id = $1`, id).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("checking user exists: %w", err)
		}
	} else {
		if len(accessPolicyPredicates) > 0 {
			setClauses = append(setClauses, fmt.Sprintf(
				"access_policy_revision = CASE WHEN %s THEN access_policy_revision + 1 ELSE access_policy_revision END",
				strings.Join(accessPolicyPredicates, " OR "),
			))
		}

		// Always bump updated_at.
		setClauses = append(setClauses, "updated_at = NOW()")

		query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d",
			strings.Join(setClauses, ", "), argIndex)
		args = append(args, id)

		tag, err := tx.Exec(ctx, query, args...)
		if err != nil {
			if isDuplicateKeyError(err) {
				return fmt.Errorf("%w: %s", ErrDuplicate, extractConstraint(err))
			}
			return fmt.Errorf("updating user: %w", err)
		}

		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
	}

	if input.GroupIDs != nil {
		if err := r.groups.replaceUserGroupsTx(ctx, tx, id, *input.GroupIDs); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing user update: %w", err)
	}
	return nil
}

// Delete removes a user by their ID.
func (r *UserRepository) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// List returns all users ordered by ID ascending.
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

	// Bulk-hydrate effective policy with a single membership query.
	userIDs := make([]int, 0, len(users))
	for _, u := range users {
		userIDs = append(userIDs, u.ID)
	}
	groupsByUser, err := r.groups.GroupsForUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		ApplyEffectivePolicy(u, groupsByUser[u.ID])
	}

	return users, nil
}

// Count returns the number of users in the database.
func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return count, nil
}

// isDuplicateKeyError checks if the error is a PostgreSQL unique_violation (code 23505).
func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// isForeignKeyError checks if the error is a PostgreSQL foreign_key_violation (code 23503).
func isForeignKeyError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	return false
}

// extractConstraint extracts the constraint name from a PgError for diagnostic messages.
func extractConstraint(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return "unknown"
}
