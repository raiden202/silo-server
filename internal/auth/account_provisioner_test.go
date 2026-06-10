package auth

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Silo-Server/silo-server/internal/models"
	"github.com/Silo-Server/silo-server/internal/userstore"
)

type stubAccountUsers struct {
	createFn func(context.Context, models.CreateUserInput) (*models.User, error)
	deleteFn func(context.Context, int) error
}

func (s stubAccountUsers) Create(ctx context.Context, input models.CreateUserInput) (*models.User, error) {
	return s.createFn(ctx, input)
}

func (s stubAccountUsers) Delete(ctx context.Context, id int) error {
	if s.deleteFn == nil {
		return nil
	}
	return s.deleteFn(ctx, id)
}

type stubStoreProvider struct {
	store userstore.UserStore
	err   error
}

func (s stubStoreProvider) ForUser(context.Context, int) (userstore.UserStore, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.store, nil
}

func (s stubStoreProvider) Close() error { return nil }

type stubUserStore struct {
	userstore.UserStore
	createProfileFn func(context.Context, userstore.Profile) error
}

func (s stubUserStore) CreateProfile(ctx context.Context, p userstore.Profile) error {
	return s.createProfileFn(ctx, p)
}

func TestAccountProvisionerCreateAccount_SkipsProfileByDefault(t *testing.T) {
	var createCalls int
	provisioner := NewAccountProvisioner(
		stubAccountUsers{
			createFn: func(context.Context, models.CreateUserInput) (*models.User, error) {
				createCalls++
				return &models.User{ID: 42, Username: "alex"}, nil
			},
		},
		nil,
		nil,
		nil,
	)

	user, err := provisioner.CreateAccount(context.Background(), CreateAccountInput{
		User: models.CreateUserInput{Username: "alex"},
	})
	if err != nil {
		t.Fatalf("CreateAccount returned error: %v", err)
	}
	if user == nil || user.ID != 42 {
		t.Fatalf("CreateAccount user = %#v, want ID 42", user)
	}
	if createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", createCalls)
	}
}

func TestAccountProvisionerCreateAccount_CreatesDefaultProfile(t *testing.T) {
	var createdProfile userstore.Profile
	provisioner := NewAccountProvisioner(
		stubAccountUsers{
			createFn: func(context.Context, models.CreateUserInput) (*models.User, error) {
				return &models.User{ID: 7, Username: "alex"}, nil
			},
		},
		stubStoreProvider{
			store: stubUserStore{
				createProfileFn: func(_ context.Context, p userstore.Profile) error {
					createdProfile = p
					return nil
				},
			},
		},
		nil,
		nil,
	)

	_, err := provisioner.CreateAccount(context.Background(), CreateAccountInput{
		User: models.CreateUserInput{Username: "alex"},
		DefaultProfile: DefaultProfileOptions{
			Enabled: true,
		},
	})
	if err != nil {
		t.Fatalf("CreateAccount returned error: %v", err)
	}
	if createdProfile.Name != "alex" {
		t.Fatalf("created profile name = %q, want alex", createdProfile.Name)
	}
	if !createdProfile.ShowForcedSubtitles {
		t.Fatal("created profile should enable forced subtitles by default")
	}
}

func TestAccountProvisionerCreateAccount_UsesExplicitProfileName(t *testing.T) {
	var createdProfile userstore.Profile
	provisioner := NewAccountProvisioner(
		stubAccountUsers{
			createFn: func(context.Context, models.CreateUserInput) (*models.User, error) {
				return &models.User{ID: 7, Username: "alex"}, nil
			},
		},
		stubStoreProvider{
			store: stubUserStore{
				createProfileFn: func(_ context.Context, p userstore.Profile) error {
					createdProfile = p
					return nil
				},
			},
		},
		nil,
		nil,
	)

	_, err := provisioner.CreateAccount(context.Background(), CreateAccountInput{
		User: models.CreateUserInput{Username: "alex"},
		DefaultProfile: DefaultProfileOptions{
			Enabled: true,
			Name:    "  Living Room  ",
		},
	})
	if err != nil {
		t.Fatalf("CreateAccount returned error: %v", err)
	}
	if createdProfile.Name != "Living Room" {
		t.Fatalf("created profile name = %q, want Living Room", createdProfile.Name)
	}
}

func TestAccountProvisionerCreateAccount_BlankExplicitProfileNameFallsBackToUsername(t *testing.T) {
	var createdProfile userstore.Profile
	provisioner := NewAccountProvisioner(
		stubAccountUsers{
			createFn: func(context.Context, models.CreateUserInput) (*models.User, error) {
				return &models.User{ID: 7, Username: "alex"}, nil
			},
		},
		stubStoreProvider{
			store: stubUserStore{
				createProfileFn: func(_ context.Context, p userstore.Profile) error {
					createdProfile = p
					return nil
				},
			},
		},
		nil,
		nil,
	)

	_, err := provisioner.CreateAccount(context.Background(), CreateAccountInput{
		User: models.CreateUserInput{Username: "alex"},
		DefaultProfile: DefaultProfileOptions{
			Enabled: true,
			Name:    "   ",
		},
	})
	if err != nil {
		t.Fatalf("CreateAccount returned error: %v", err)
	}
	if createdProfile.Name != "alex" {
		t.Fatalf("created profile name = %q, want alex", createdProfile.Name)
	}
}

func TestAccountProvisionerCreateAccount_DeletesUserWhenProfileCreationFails(t *testing.T) {
	var deletedUserID int
	provisioner := NewAccountProvisioner(
		stubAccountUsers{
			createFn: func(context.Context, models.CreateUserInput) (*models.User, error) {
				return &models.User{ID: 9, Username: "alex"}, nil
			},
			deleteFn: func(_ context.Context, id int) error {
				deletedUserID = id
				return nil
			},
		},
		stubStoreProvider{
			store: stubUserStore{
				createProfileFn: func(context.Context, userstore.Profile) error {
					return errors.New("boom")
				},
			},
		},
		nil,
		nil,
	)

	_, err := provisioner.CreateAccount(context.Background(), CreateAccountInput{
		User: models.CreateUserInput{Username: "alex"},
		DefaultProfile: DefaultProfileOptions{
			Enabled: true,
		},
	})
	if err == nil {
		t.Fatal("CreateAccount returned nil error, want failure")
	}
	if deletedUserID != 9 {
		t.Fatalf("deletedUserID = %d, want 9", deletedUserID)
	}
}

type stubGroupResolver struct {
	groups map[string]*models.Group
}

func (s stubGroupResolver) GetBySlug(_ context.Context, slug string) (*models.Group, error) {
	if group, ok := s.groups[slug]; ok {
		return group, nil
	}
	return nil, ErrGroupNotFound
}

type stubSettings map[string]string

func (s stubSettings) Get(_ context.Context, key string) (string, error) {
	return s[key], nil
}

func TestAccountProvisionerCreateAccount_NilGroupIDsResolvesConfiguredDefaults(t *testing.T) {
	var gotInput models.CreateUserInput
	provisioner := NewAccountProvisioner(
		stubAccountUsers{
			createFn: func(_ context.Context, input models.CreateUserInput) (*models.User, error) {
				gotInput = input
				return &models.User{ID: 1, Username: "alex"}, nil
			},
		},
		nil,
		stubSettings{DefaultGroupSlugsSettingKey: `["family","kids"]`},
		stubGroupResolver{groups: map[string]*models.Group{
			"family": {ID: 11, Slug: "family"},
			"kids":   {ID: 12, Slug: "kids"},
		}},
	)

	if _, err := provisioner.CreateAccount(context.Background(), CreateAccountInput{
		User: models.CreateUserInput{Username: "alex"},
	}); err != nil {
		t.Fatalf("CreateAccount returned error: %v", err)
	}
	want := []int{11, 12}
	if !reflect.DeepEqual(gotInput.GroupIDs, want) {
		t.Fatalf("GroupIDs = %#v, want %#v", gotInput.GroupIDs, want)
	}
}

func TestAccountProvisionerCreateAccount_FallsBackToBuiltInUsersGroup(t *testing.T) {
	var gotInput models.CreateUserInput
	provisioner := NewAccountProvisioner(
		stubAccountUsers{
			createFn: func(_ context.Context, input models.CreateUserInput) (*models.User, error) {
				gotInput = input
				return &models.User{ID: 1, Username: "alex"}, nil
			},
		},
		nil,
		nil, // no settings configured
		stubGroupResolver{groups: map[string]*models.Group{
			models.GroupSlugUsers: {ID: 2, Slug: models.GroupSlugUsers},
		}},
	)

	if _, err := provisioner.CreateAccount(context.Background(), CreateAccountInput{
		User: models.CreateUserInput{Username: "alex"},
	}); err != nil {
		t.Fatalf("CreateAccount returned error: %v", err)
	}
	if !reflect.DeepEqual(gotInput.GroupIDs, []int{2}) {
		t.Fatalf("GroupIDs = %#v, want [2]", gotInput.GroupIDs)
	}
}

func TestAccountProvisionerCreateAccount_ExplicitGroupIDsPassThrough(t *testing.T) {
	var gotInput models.CreateUserInput
	provisioner := NewAccountProvisioner(
		stubAccountUsers{
			createFn: func(_ context.Context, input models.CreateUserInput) (*models.User, error) {
				gotInput = input
				return &models.User{ID: 1, Username: "alex"}, nil
			},
		},
		nil,
		stubSettings{DefaultGroupSlugsSettingKey: `["family"]`},
		stubGroupResolver{groups: map[string]*models.Group{
			"family": {ID: 11, Slug: "family"},
		}},
	)

	explicit := []int{99}
	if _, err := provisioner.CreateAccount(context.Background(), CreateAccountInput{
		User: models.CreateUserInput{Username: "alex", GroupIDs: explicit},
	}); err != nil {
		t.Fatalf("CreateAccount returned error: %v", err)
	}
	if !reflect.DeepEqual(gotInput.GroupIDs, explicit) {
		t.Fatalf("GroupIDs = %#v, want %#v", gotInput.GroupIDs, explicit)
	}
}

func TestAccountProvisionerCreateAccount_SkipsUnknownDefaultSlugs(t *testing.T) {
	var gotInput models.CreateUserInput
	provisioner := NewAccountProvisioner(
		stubAccountUsers{
			createFn: func(_ context.Context, input models.CreateUserInput) (*models.User, error) {
				gotInput = input
				return &models.User{ID: 1, Username: "alex"}, nil
			},
		},
		nil,
		stubSettings{DefaultGroupSlugsSettingKey: `["ghost","family"]`},
		stubGroupResolver{groups: map[string]*models.Group{
			"family": {ID: 11, Slug: "family"},
		}},
	)

	if _, err := provisioner.CreateAccount(context.Background(), CreateAccountInput{
		User: models.CreateUserInput{Username: "alex"},
	}); err != nil {
		t.Fatalf("CreateAccount returned error: %v", err)
	}
	if !reflect.DeepEqual(gotInput.GroupIDs, []int{11}) {
		t.Fatalf("GroupIDs = %#v, want [11]", gotInput.GroupIDs)
	}
}

func TestAccountProvisionerCreateAccount_AllUnknownSlugsFallBackToUsersGroup(t *testing.T) {
	var gotInput models.CreateUserInput
	provisioner := NewAccountProvisioner(
		stubAccountUsers{
			createFn: func(_ context.Context, input models.CreateUserInput) (*models.User, error) {
				gotInput = input
				return &models.User{ID: 1, Username: "alex"}, nil
			},
		},
		nil,
		stubSettings{DefaultGroupSlugsSettingKey: `["ghost","phantom"]`},
		stubGroupResolver{groups: map[string]*models.Group{
			models.GroupSlugUsers: {ID: 2, Slug: models.GroupSlugUsers},
		}},
	)

	if _, err := provisioner.CreateAccount(context.Background(), CreateAccountInput{
		User: models.CreateUserInput{Username: "alex"},
	}); err != nil {
		t.Fatalf("CreateAccount returned error: %v", err)
	}
	if !reflect.DeepEqual(gotInput.GroupIDs, []int{2}) {
		t.Fatalf("GroupIDs = %#v, want [2] (built-in users fallback)", gotInput.GroupIDs)
	}
}

func TestAccountProvisionerCreateAccount_FallbackGroupMissingReturnsError(t *testing.T) {
	provisioner := NewAccountProvisioner(
		stubAccountUsers{
			createFn: func(context.Context, models.CreateUserInput) (*models.User, error) {
				t.Fatal("Create should not be called when no default group resolves")
				return nil, nil
			},
		},
		nil,
		stubSettings{DefaultGroupSlugsSettingKey: `["ghost"]`},
		stubGroupResolver{groups: map[string]*models.Group{}},
	)

	if _, err := provisioner.CreateAccount(context.Background(), CreateAccountInput{
		User: models.CreateUserInput{Username: "alex"},
	}); err == nil {
		t.Fatal("CreateAccount returned nil error, want fallback resolution failure")
	}
}
