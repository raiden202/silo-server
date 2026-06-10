package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Silo-Server/silo-server/internal/models"
	"github.com/Silo-Server/silo-server/internal/userstore"
)

// DefaultGroupSlugsSettingKey is the server setting holding the JSON array of
// group slugs assigned to newly provisioned accounts.
const DefaultGroupSlugsSettingKey = "users.default_group_slugs"

type AccountUserRepository interface {
	Create(ctx context.Context, input models.CreateUserInput) (*models.User, error)
	Delete(ctx context.Context, id int) error
}

// GroupResolver resolves group slugs to groups. Implemented by GroupRepository.
type GroupResolver interface {
	GetBySlug(ctx context.Context, slug string) (*models.Group, error)
}

type DefaultProfileOptions struct {
	Enabled bool
	Name    string
}

type CreateAccountInput struct {
	User           models.CreateUserInput
	DefaultProfile DefaultProfileOptions
}

type AccountProvisioner struct {
	users         AccountUserRepository
	storeProvider userstore.UserStoreProvider
	settings      SettingsGetter
	groups        GroupResolver
}

func NewAccountProvisioner(
	users AccountUserRepository,
	storeProvider userstore.UserStoreProvider,
	settings SettingsGetter,
	groups GroupResolver,
) *AccountProvisioner {
	return &AccountProvisioner{
		users:         users,
		storeProvider: storeProvider,
		settings:      settings,
		groups:        groups,
	}
}

func (p *AccountProvisioner) CreateAccount(
	ctx context.Context,
	input CreateAccountInput,
) (*models.User, error) {
	if input.User.GroupIDs == nil && p.groups != nil {
		ids, err := p.defaultGroupIDs(ctx)
		if err != nil {
			return nil, err
		}
		input.User.GroupIDs = ids
	}

	user, err := p.users.Create(ctx, input.User)
	if err != nil {
		return nil, err
	}

	if !input.DefaultProfile.Enabled {
		return user, nil
	}

	if err := p.createDefaultProfile(ctx, user.ID, input); err != nil {
		if deleteErr := p.users.Delete(ctx, user.ID); deleteErr != nil {
			return nil, fmt.Errorf(
				"create default profile: %w (cleanup user: %v)",
				err,
				deleteErr,
			)
		}
		return nil, fmt.Errorf("create default profile: %w", err)
	}

	return user, nil
}

// defaultGroupIDs resolves the configured default group slugs (falling back to
// the built-in users group) to group IDs. Unknown slugs are skipped so a stale
// setting never blocks signups; if no configured slug resolves, the built-in
// users group is used so accounts never start with an empty policy.
func (p *AccountProvisioner) defaultGroupIDs(ctx context.Context) ([]int, error) {
	slugs := []string{models.GroupSlugUsers}
	if p.settings != nil {
		raw, err := p.settings.Get(ctx, DefaultGroupSlugsSettingKey)
		if err != nil {
			slog.Warn("reading default group slugs setting failed", "key", DefaultGroupSlugsSettingKey, "error", err)
		} else if strings.TrimSpace(raw) != "" {
			var configured []string
			if jsonErr := json.Unmarshal([]byte(raw), &configured); jsonErr != nil {
				slog.Warn("parsing default group slugs setting failed", "key", DefaultGroupSlugsSettingKey, "error", jsonErr)
			} else if len(configured) > 0 {
				slugs = configured
			}
		}
	}
	ids := make([]int, 0, len(slugs))
	for _, slug := range slugs {
		group, err := p.groups.GetBySlug(ctx, slug)
		if err != nil {
			if errors.Is(err, ErrGroupNotFound) {
				// A stale setting must not block signups.
				slog.Warn("skipping unknown default group slug", "slug", slug)
				continue
			}
			return nil, fmt.Errorf("resolving default group %q: %w", slug, err)
		}
		ids = append(ids, group.ID)
	}
	if len(ids) == 0 {
		group, err := p.groups.GetBySlug(ctx, models.GroupSlugUsers)
		if err != nil {
			return nil, fmt.Errorf("resolving fallback group %q: %w", models.GroupSlugUsers, err)
		}
		ids = append(ids, group.ID)
	}
	return ids, nil
}

func (p *AccountProvisioner) createDefaultProfile(
	ctx context.Context,
	userID int,
	input CreateAccountInput,
) error {
	if p.storeProvider == nil {
		return fmt.Errorf("user store provider unavailable")
	}

	store, err := p.storeProvider.ForUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("open user store: %w", err)
	}

	name := strings.TrimSpace(input.DefaultProfile.Name)
	if name == "" {
		name = strings.TrimSpace(input.User.Username)
	}
	if name == "" {
		return fmt.Errorf("default profile name is required")
	}

	if err := store.CreateProfile(ctx, userstore.Profile{
		Name:                name,
		ShowForcedSubtitles: true,
	}); err != nil {
		return fmt.Errorf("store profile: %w", err)
	}

	return nil
}
