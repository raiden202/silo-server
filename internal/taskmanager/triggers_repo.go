package taskmanager

import "context"

// TriggerRepository persists task trigger configuration.
type TriggerRepository interface {
	GetTriggers(ctx context.Context, taskKey string) ([]TriggerConfig, error)
	SetTriggers(ctx context.Context, taskKey string, triggers []TriggerConfig) error
}
