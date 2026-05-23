package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrAuthBindingNotFound = errors.New("plugin auth binding not found")
	ErrTaskBindingNotFound = errors.New("plugin task binding not found")
)

type RuntimeConfig struct {
	InstallationID int
	Key            string
	Value          map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AuthBinding struct {
	InstallationID int
	CapabilityID   string
	Enabled        bool
	DisplayOrder   int
	AutoProvision  bool
	DefaultLogin   bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type TaskBinding struct {
	InstallationID int
	CapabilityID   string
	Enabled        bool
	Trigger        map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type RuntimeConfigStore struct {
	pool *pgxpool.Pool
}

func NewRuntimeConfigStore(pool *pgxpool.Pool) *RuntimeConfigStore {
	return &RuntimeConfigStore{pool: pool}
}

func (s *RuntimeConfigStore) PutGlobalConfig(
	ctx context.Context,
	installationID int,
	key string,
	value map[string]any,
) error {
	if value == nil {
		value = map[string]any{}
	}
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling plugin runtime config: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO plugin_runtime_configs (plugin_installation_id, config_key, config_value)
		VALUES ($1, $2, $3)
		ON CONFLICT (plugin_installation_id, config_key) DO UPDATE SET
			config_value = EXCLUDED.config_value,
			updated_at = NOW()
	`, installationID, key, valueJSON)
	if err != nil {
		return fmt.Errorf("upserting plugin runtime config: %w", err)
	}
	return nil
}

func (s *RuntimeConfigStore) ListGlobalConfigs(ctx context.Context, installationID int) ([]*RuntimeConfig, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT plugin_installation_id, config_key, config_value, created_at, updated_at
		FROM plugin_runtime_configs
		WHERE plugin_installation_id = $1
		ORDER BY config_key ASC
	`, installationID)
	if err != nil {
		return nil, fmt.Errorf("listing plugin runtime configs: %w", err)
	}
	defer rows.Close()

	var configs []*RuntimeConfig
	for rows.Next() {
		var config RuntimeConfig
		var valueJSON []byte
		if err := rows.Scan(
			&config.InstallationID,
			&config.Key,
			&valueJSON,
			&config.CreatedAt,
			&config.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning plugin runtime config: %w", err)
		}
		config.Value = map[string]any{}
		if len(valueJSON) > 0 {
			if err := json.Unmarshal(valueJSON, &config.Value); err != nil {
				return nil, fmt.Errorf("unmarshaling plugin runtime config: %w", err)
			}
		}
		configs = append(configs, &config)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating plugin runtime configs: %w", err)
	}
	return configs, nil
}

func (s *RuntimeConfigStore) UpsertAuthBinding(ctx context.Context, binding AuthBinding) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO plugin_auth_bindings (
			plugin_installation_id, capability_id, enabled, display_order, auto_provision, default_login
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (plugin_installation_id, capability_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			display_order = EXCLUDED.display_order,
			auto_provision = EXCLUDED.auto_provision,
			default_login = EXCLUDED.default_login,
			updated_at = NOW()
	`,
		binding.InstallationID,
		binding.CapabilityID,
		binding.Enabled,
		binding.DisplayOrder,
		binding.AutoProvision,
		binding.DefaultLogin,
	)
	if err != nil {
		return fmt.Errorf("upserting plugin auth binding: %w", err)
	}
	return nil
}

func (s *RuntimeConfigStore) GetAuthBinding(
	ctx context.Context,
	installationID int,
	capabilityID string,
) (*AuthBinding, error) {
	var binding AuthBinding
	err := s.pool.QueryRow(ctx, `
		SELECT plugin_installation_id, capability_id, enabled, display_order, auto_provision, default_login, created_at, updated_at
		FROM plugin_auth_bindings
		WHERE plugin_installation_id = $1 AND capability_id = $2
	`, installationID, capabilityID).Scan(
		&binding.InstallationID,
		&binding.CapabilityID,
		&binding.Enabled,
		&binding.DisplayOrder,
		&binding.AutoProvision,
		&binding.DefaultLogin,
		&binding.CreatedAt,
		&binding.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAuthBindingNotFound
		}
		return nil, fmt.Errorf("getting plugin auth binding: %w", err)
	}
	return &binding, nil
}

func (s *RuntimeConfigStore) ListAuthBindings(ctx context.Context) ([]*AuthBinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT plugin_installation_id, capability_id, enabled, display_order, auto_provision, default_login, created_at, updated_at
		FROM plugin_auth_bindings
		ORDER BY display_order ASC, plugin_installation_id ASC, capability_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing plugin auth bindings: %w", err)
	}
	defer rows.Close()

	var bindings []*AuthBinding
	for rows.Next() {
		var binding AuthBinding
		if err := rows.Scan(
			&binding.InstallationID,
			&binding.CapabilityID,
			&binding.Enabled,
			&binding.DisplayOrder,
			&binding.AutoProvision,
			&binding.DefaultLogin,
			&binding.CreatedAt,
			&binding.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning plugin auth binding: %w", err)
		}
		bindings = append(bindings, &binding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating plugin auth bindings: %w", err)
	}
	return bindings, nil
}

func (s *RuntimeConfigStore) UpsertTaskBinding(ctx context.Context, binding TaskBinding) error {
	trigger := binding.Trigger
	if trigger == nil {
		trigger = map[string]any{}
	}
	triggerJSON, err := json.Marshal(trigger)
	if err != nil {
		return fmt.Errorf("marshaling plugin task binding trigger: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO plugin_task_bindings (plugin_installation_id, capability_id, enabled, trigger)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (plugin_installation_id, capability_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			trigger = EXCLUDED.trigger,
			updated_at = NOW()
	`,
		binding.InstallationID,
		binding.CapabilityID,
		binding.Enabled,
		triggerJSON,
	)
	if err != nil {
		return fmt.Errorf("upserting plugin task binding: %w", err)
	}
	return nil
}

func (s *RuntimeConfigStore) GetTaskBinding(
	ctx context.Context,
	installationID int,
	capabilityID string,
) (*TaskBinding, error) {
	var binding TaskBinding
	var triggerJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT plugin_installation_id, capability_id, enabled, trigger, created_at, updated_at
		FROM plugin_task_bindings
		WHERE plugin_installation_id = $1 AND capability_id = $2
	`, installationID, capabilityID).Scan(
		&binding.InstallationID,
		&binding.CapabilityID,
		&binding.Enabled,
		&triggerJSON,
		&binding.CreatedAt,
		&binding.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTaskBindingNotFound
		}
		return nil, fmt.Errorf("getting plugin task binding: %w", err)
	}
	binding.Trigger = map[string]any{}
	if len(triggerJSON) > 0 {
		if err := json.Unmarshal(triggerJSON, &binding.Trigger); err != nil {
			return nil, fmt.Errorf("unmarshaling plugin task binding trigger: %w", err)
		}
	}
	return &binding, nil
}

func (s *RuntimeConfigStore) ListTaskBindings(ctx context.Context) ([]*TaskBinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT plugin_installation_id, capability_id, enabled, trigger, created_at, updated_at
		FROM plugin_task_bindings
		ORDER BY plugin_installation_id ASC, capability_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing plugin task bindings: %w", err)
	}
	defer rows.Close()

	var bindings []*TaskBinding
	for rows.Next() {
		var binding TaskBinding
		var triggerJSON []byte
		if err := rows.Scan(
			&binding.InstallationID,
			&binding.CapabilityID,
			&binding.Enabled,
			&triggerJSON,
			&binding.CreatedAt,
			&binding.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning plugin task binding: %w", err)
		}
		binding.Trigger = map[string]any{}
		if len(triggerJSON) > 0 {
			if err := json.Unmarshal(triggerJSON, &binding.Trigger); err != nil {
				return nil, fmt.Errorf("unmarshaling plugin task binding trigger: %w", err)
			}
		}
		bindings = append(bindings, &binding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating plugin task bindings: %w", err)
	}
	return bindings, nil
}
