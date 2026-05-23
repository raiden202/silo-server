package plugins

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Silo-Server/silo-server/internal/pluginhost"
)

// ConfigValidationError indicates that the caller supplied an invalid key or
// a value that did not match the plugin's manifest global_config_schema.
// Callers (the admin HTTP handler) should map this to a 400-equivalent
// response.
type ConfigValidationError struct {
	Message string
	Cause   error
}

func (e *ConfigValidationError) Error() string { return e.Message }
func (e *ConfigValidationError) Unwrap() error { return e.Cause }

// SetGlobalConfig validates a single global-config entry against the plugin's
// manifest schema, persists it, and stops the running plugin so the next
// invocation rebinds with the new configuration. Used by the admin HTTP
// handler at PUT /api/admin/plugins/installations/{id}/config.
func (s *Service) SetGlobalConfig(
	ctx context.Context,
	installationID int,
	key string,
	value map[string]any,
) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return &ConfigValidationError{Message: "config key is required"}
	}
	if s.configs == nil {
		return fmt.Errorf("plugin config store not configured")
	}

	_, manifest, err := s.ensureInstallationCache(ctx, installationID, false)
	if err != nil {
		return err
	}

	if value == nil {
		value = map[string]any{}
	}
	if err := ValidateGlobalConfigValue(manifest, key, value); err != nil {
		return &ConfigValidationError{Message: err.Error(), Cause: err}
	}

	if err := s.configs.PutGlobalConfig(ctx, installationID, key, value); err != nil {
		return fmt.Errorf("persist plugin config: %w", err)
	}

	if s.host != nil {
		if err := s.host.Stop(installationID); err != nil && !errors.Is(err, pluginhost.ErrClientNotFound) {
			return fmt.Errorf("reload plugin after config update: %w", err)
		}
	}
	return nil
}
