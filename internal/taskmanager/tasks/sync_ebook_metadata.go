package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Silo-Server/silo-server/internal/ebooks"
	"github.com/Silo-Server/silo-server/internal/taskmanager"
)

const ebookMetadataExecutionBudget = 4 * time.Minute

type ebookMetadataEnricher interface {
	Run(ctx context.Context, scope ebooks.EnrichmentScope) (ebooks.EnrichmentRunResult, error)
}

type ebookMetadataTask struct {
	enricher    ebookMetadataEnricher
	scope       ebooks.EnrichmentScope
	key         string
	name        string
	description string
	triggers    []taskmanager.TriggerConfig
	budget      time.Duration
	now         func() time.Time
	errorPrefix string
}

// SyncEbookMetadataTask drains new and recurring ebook metadata work.
type SyncEbookMetadataTask struct {
	*ebookMetadataTask
}

// BackfillEbookMetadataTask drains the legacy ebook backlog only when manually run.
type BackfillEbookMetadataTask struct {
	*ebookMetadataTask
}

func NewSyncEbookMetadataTask(enricher ebookMetadataEnricher) *SyncEbookMetadataTask {
	return &SyncEbookMetadataTask{ebookMetadataTask: &ebookMetadataTask{
		enricher:    enricher,
		scope:       ebooks.EnrichmentScopeIncremental,
		key:         "sync_ebook_metadata",
		name:        "Sync Ebook Metadata",
		description: "Fetches metadata for new ebooks and ebooks due for a recurring refresh",
		triggers:    []taskmanager.TriggerConfig{{Type: taskmanager.TriggerTypeInterval, IntervalMs: 5 * 60 * 1000}},
		budget:      ebookMetadataExecutionBudget,
		now:         time.Now,
		errorPrefix: "ebook metadata sync",
	}}
}

func NewBackfillEbookMetadataTask(enricher ebookMetadataEnricher) *BackfillEbookMetadataTask {
	return &BackfillEbookMetadataTask{ebookMetadataTask: &ebookMetadataTask{
		enricher:    enricher,
		scope:       ebooks.EnrichmentScopeLegacy,
		key:         "backfill_ebook_metadata",
		name:        "Backfill Ebook Metadata",
		description: "Manually enriches the legacy ebook backlog without competing with scheduled metadata sync",
		budget:      ebookMetadataExecutionBudget,
		now:         time.Now,
		errorPrefix: "ebook metadata backfill",
	}}
}

func (t *ebookMetadataTask) Key() string         { return t.key }
func (t *ebookMetadataTask) Name() string        { return t.name }
func (t *ebookMetadataTask) Description() string { return t.description }
func (t *ebookMetadataTask) Category() taskmanager.TaskCategory {
	return taskmanager.TaskCategoryMetadata
}
func (t *ebookMetadataTask) IsHidden() bool { return false }

func (t *ebookMetadataTask) DefaultTriggers() []taskmanager.TriggerConfig {
	return append([]taskmanager.TriggerConfig(nil), t.triggers...)
}

func (t *ebookMetadataTask) Execute(ctx context.Context, progress taskmanager.ProgressReporter) error {
	progress.Report(0, fmt.Sprintf("%s started", t.name))
	started := t.now()
	total := ebooks.EnrichmentRunResult{}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		batch, err := t.enricher.Run(ctx, t.scope)
		if err != nil {
			return fmt.Errorf("%s: %w", t.errorPrefix, err)
		}
		addEbookEnrichmentResult(&total, batch)
		reportEbookEnrichmentProgress(progress, total, false)

		if err := ctx.Err(); err != nil {
			return err
		}
		if batch.Remaining == 0 {
			reportEbookEnrichmentProgress(progress, total, true)
			return nil
		}
		if batch.Claimed == 0 {
			return nil
		}
		if t.now().Sub(started) >= t.budget {
			total.Deferred += total.Remaining
			reportEbookEnrichmentProgress(progress, total, false)
			return nil
		}
	}
}

func addEbookEnrichmentResult(total *ebooks.EnrichmentRunResult, batch ebooks.EnrichmentRunResult) {
	total.Claimed += batch.Claimed
	total.Enriched += batch.Enriched
	total.NoMatch += batch.NoMatch
	total.Failed += batch.Failed
	total.Deferred += batch.Deferred
	total.Remaining = batch.Remaining
}

func reportEbookEnrichmentProgress(
	progress taskmanager.ProgressReporter,
	result ebooks.EnrichmentRunResult,
	complete bool,
) {
	data, _ := json.Marshal(result)
	progress.SetResultData(data)

	percent := ebookEnrichmentPercent(result)
	if complete && result.Remaining == 0 {
		percent = 100
	}
	progress.Report(percent, fmt.Sprintf(
		"Claimed %d, enriched %d, no match %d, failed %d, deferred %d, remaining %d",
		result.Claimed,
		result.Enriched,
		result.NoMatch,
		result.Failed,
		result.Deferred,
		result.Remaining,
	))
}

func ebookEnrichmentPercent(result ebooks.EnrichmentRunResult) float64 {
	if result.Remaining == 0 {
		return 100
	}
	total := result.Claimed + result.Remaining
	if total <= 0 {
		return 0
	}
	percent := float64(result.Claimed) * 100 / float64(total)
	if percent >= 100 {
		return 99
	}
	return percent
}
