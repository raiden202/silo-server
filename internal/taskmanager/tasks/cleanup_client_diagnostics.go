package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Silo-Server/silo-server/internal/diagnostics"
	"github.com/Silo-Server/silo-server/internal/opslog"
	"github.com/Silo-Server/silo-server/internal/taskmanager"
)

// ClientDiagnosticsCleanupTask prunes expired client diagnostics and reconciles
// object-store leftovers from interrupted uploads.
type ClientDiagnosticsCleanupTask struct {
	repo     diagnostics.CleanupRepository
	settings diagnostics.SettingsStore
	store    diagnostics.ObjectStore
	logger   *slog.Logger
}

func NewClientDiagnosticsCleanupTask(
	repo diagnostics.CleanupRepository,
	settings diagnostics.SettingsStore,
	store diagnostics.ObjectStore,
) *ClientDiagnosticsCleanupTask {
	return &ClientDiagnosticsCleanupTask{
		repo:     repo,
		settings: settings,
		store:    store,
		logger:   slog.Default(),
	}
}

func (t *ClientDiagnosticsCleanupTask) Key() string { return "cleanup_client_diagnostics" }
func (t *ClientDiagnosticsCleanupTask) Name() string {
	return "Cleanup Client Diagnostics"
}
func (t *ClientDiagnosticsCleanupTask) Description() string {
	return "Prunes expired client diagnostic reports and orphaned bundles"
}
func (t *ClientDiagnosticsCleanupTask) Category() taskmanager.TaskCategory {
	return taskmanager.TaskCategorySystem
}
func (t *ClientDiagnosticsCleanupTask) IsHidden() bool { return false }

func (t *ClientDiagnosticsCleanupTask) DefaultTriggers() []taskmanager.TriggerConfig {
	return []taskmanager.TriggerConfig{
		{Type: taskmanager.TriggerTypeStartup},
		{
			Type:       taskmanager.TriggerTypeInterval,
			IntervalMs: int64(opslog.LoadCleanupInterval(context.Background(), t.settings) / time.Millisecond),
		},
	}
}

func (t *ClientDiagnosticsCleanupTask) Execute(ctx context.Context, progress taskmanager.ProgressReporter) error {
	progress.Report(0, "Pruning client diagnostic reports")
	result, err := diagnostics.CleanupOnce(ctx, t.repo, t.settings, t.store, t.logger)
	if err != nil {
		return err
	}
	progress.Report(100, fmt.Sprintf(
		"Pruned %d client diagnostic reports and %d orphaned bundles",
		result.ReportsDeleted(),
		result.OrphanObjectsDeleted,
	))
	return nil
}
