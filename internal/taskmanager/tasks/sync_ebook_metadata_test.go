package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Silo-Server/silo-server/internal/ebooks"
	"github.com/Silo-Server/silo-server/internal/taskmanager"
)

type fakeEbookMetadataEnricher struct {
	results []ebooks.EnrichmentRunResult
	err     error
	scopes  []ebooks.EnrichmentScope
	onRun   func(int)
}

func (f *fakeEbookMetadataEnricher) Run(_ context.Context, scope ebooks.EnrichmentScope) (ebooks.EnrichmentRunResult, error) {
	f.scopes = append(f.scopes, scope)
	call := len(f.scopes)
	if f.onRun != nil {
		f.onRun(call)
	}
	if f.err != nil {
		return ebooks.EnrichmentRunResult{}, f.err
	}
	if call > len(f.results) {
		return ebooks.EnrichmentRunResult{}, nil
	}
	return f.results[call-1], nil
}

type ebookMetadataProgressReporter struct {
	percents []float64
	messages []string
	results  []json.RawMessage
}

func (p *ebookMetadataProgressReporter) Report(percent float64, message string) {
	p.percents = append(p.percents, percent)
	p.messages = append(p.messages, message)
}

func (p *ebookMetadataProgressReporter) SetResultData(data json.RawMessage) {
	p.results = append(p.results, append(json.RawMessage(nil), data...))
}

func TestEbookMetadataTaskPropertiesAndScopes(t *testing.T) {
	enricher := &fakeEbookMetadataEnricher{}
	syncTask := NewSyncEbookMetadataTask(enricher)
	backfillTask := NewBackfillEbookMetadataTask(enricher)

	if syncTask.Key() != "sync_ebook_metadata" || syncTask.Name() != "Sync Ebook Metadata" {
		t.Fatalf("unexpected sync identity: %q %q", syncTask.Key(), syncTask.Name())
	}
	if backfillTask.Key() != "backfill_ebook_metadata" {
		t.Fatalf("backfill Key() = %q", backfillTask.Key())
	}
	if !strings.Contains(strings.ToLower(backfillTask.Description()), "legacy") {
		t.Fatalf("backfill description does not explain legacy work: %q", backfillTask.Description())
	}
	for _, task := range []taskmanager.Task{syncTask, backfillTask} {
		if task.Category() != taskmanager.TaskCategoryMetadata || task.IsHidden() {
			t.Fatalf("unexpected task properties for %q", task.Key())
		}
	}
	triggers := syncTask.DefaultTriggers()
	if len(triggers) != 1 || triggers[0].Type != taskmanager.TriggerTypeInterval || triggers[0].IntervalMs != 5*60*1000 {
		t.Fatalf("sync DefaultTriggers() = %#v", triggers)
	}
	if triggers := backfillTask.DefaultTriggers(); len(triggers) != 0 {
		t.Fatalf("backfill DefaultTriggers() = %#v, want none", triggers)
	}
}

func TestEbookMetadataTaskDrainsBatchesAndReportsHonestProgress(t *testing.T) {
	enricher := &fakeEbookMetadataEnricher{results: []ebooks.EnrichmentRunResult{
		{Claimed: 4, Enriched: 2, NoMatch: 1, Failed: 1, Remaining: 3},
		{Claimed: 3, Enriched: 1, Deferred: 2, Remaining: 0},
	}}
	task := NewSyncEbookMetadataTask(enricher)
	progress := &ebookMetadataProgressReporter{}

	if err := task.Execute(context.Background(), progress); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(enricher.scopes) != 2 {
		t.Fatalf("Run calls = %d, want 2", len(enricher.scopes))
	}
	for _, scope := range enricher.scopes {
		if scope != ebooks.EnrichmentScopeIncremental {
			t.Fatalf("sync scope = %q, want incremental", scope)
		}
	}
	var result ebooks.EnrichmentRunResult
	if err := json.Unmarshal(progress.results[len(progress.results)-1], &result); err != nil {
		t.Fatalf("result JSON error: %v", err)
	}
	want := ebooks.EnrichmentRunResult{Claimed: 7, Enriched: 3, NoMatch: 1, Failed: 1, Deferred: 2}
	if result != want {
		t.Fatalf("result = %+v, want %+v", result, want)
	}
	if progress.percents[1] >= 100 {
		t.Fatalf("first batch progress = %.1f, must be below 100 with remaining work", progress.percents[1])
	}
	if got := progress.percents[len(progress.percents)-1]; got != 100 {
		t.Fatalf("final progress = %.1f, want 100", got)
	}
}

func TestEbookMetadataBackfillUsesLegacyScope(t *testing.T) {
	enricher := &fakeEbookMetadataEnricher{results: []ebooks.EnrichmentRunResult{{Remaining: 0}}}
	task := NewBackfillEbookMetadataTask(enricher)
	if err := task.Execute(context.Background(), &ebookMetadataProgressReporter{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(enricher.scopes) != 1 || enricher.scopes[0] != ebooks.EnrichmentScopeLegacy {
		t.Fatalf("backfill scopes = %#v, want legacy", enricher.scopes)
	}
}

func TestEbookMetadataTaskStopsBetweenBatchesOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	enricher := &fakeEbookMetadataEnricher{
		results: []ebooks.EnrichmentRunResult{{Claimed: 1, Enriched: 1, Remaining: 2}},
		onRun: func(int) {
			cancel()
		},
	}
	err := NewSyncEbookMetadataTask(enricher).Execute(ctx, &ebookMetadataProgressReporter{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute() error = %v, want context.Canceled", err)
	}
	if len(enricher.scopes) != 1 {
		t.Fatalf("Run calls = %d, want 1", len(enricher.scopes))
	}
}

func TestEbookMetadataTaskTreatsBudgetAsCleanDeferredCompletion(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	enricher := &fakeEbookMetadataEnricher{
		results: []ebooks.EnrichmentRunResult{{Claimed: 2, Enriched: 2, Remaining: 5}},
		onRun: func(int) {
			now = now.Add(5 * time.Minute)
		},
	}
	task := NewSyncEbookMetadataTask(enricher)
	task.now = func() time.Time { return now }
	task.budget = 4 * time.Minute
	progress := &ebookMetadataProgressReporter{}

	if err := task.Execute(context.Background(), progress); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(enricher.scopes) != 1 {
		t.Fatalf("Run calls = %d, want 1", len(enricher.scopes))
	}
	var result ebooks.EnrichmentRunResult
	if err := json.Unmarshal(progress.results[len(progress.results)-1], &result); err != nil {
		t.Fatalf("result JSON error: %v", err)
	}
	if result.Remaining != 5 || result.Deferred != 5 {
		t.Fatalf("budget result = %+v, want remaining=5 deferred=5", result)
	}
	if got := progress.percents[len(progress.percents)-1]; got >= 100 {
		t.Fatalf("budget-limited progress = %.1f, must remain below 100", got)
	}
}

func TestEbookMetadataTaskWrapsRunError(t *testing.T) {
	enricher := &fakeEbookMetadataEnricher{err: errors.New("boom")}
	err := NewSyncEbookMetadataTask(enricher).Execute(context.Background(), &ebookMetadataProgressReporter{})
	if err == nil || !strings.Contains(err.Error(), "ebook metadata sync") {
		t.Fatalf("Execute() error = %v, want wrapped ebook metadata sync error", err)
	}
}
