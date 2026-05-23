package taskmanager_test

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Silo-Server/silo-server/internal/taskmanager"
	taskdefs "github.com/Silo-Server/silo-server/internal/taskmanager/tasks"
)

type fakeTriggerRepository struct {
	mu       sync.Mutex
	triggers map[string][]taskmanager.TriggerConfig
	setCalls map[string][]taskmanager.TriggerConfig
}

func (r *fakeTriggerRepository) GetTriggers(_ context.Context, taskKey string) ([]taskmanager.TriggerConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]taskmanager.TriggerConfig(nil), r.triggers[taskKey]...), nil
}

func (r *fakeTriggerRepository) SetTriggers(_ context.Context, taskKey string, triggers []taskmanager.TriggerConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.triggers == nil {
		r.triggers = map[string][]taskmanager.TriggerConfig{}
	}
	if r.setCalls == nil {
		r.setCalls = map[string][]taskmanager.TriggerConfig{}
	}
	copied := append([]taskmanager.TriggerConfig(nil), triggers...)
	r.triggers[taskKey] = copied
	r.setCalls[taskKey] = copied
	return nil
}

type fakeExecutionRepository struct{}

func (fakeExecutionRepository) Insert(context.Context, taskmanager.ExecutionResult) error { return nil }
func (fakeExecutionRepository) GetLatest(context.Context, string) (*taskmanager.ExecutionResult, error) {
	return nil, nil
}
func (fakeExecutionRepository) List(context.Context, string, int) ([]taskmanager.ExecutionResult, error) {
	return nil, nil
}

type fakeTrigger struct {
	cfg    taskmanager.TriggerConfig
	ch     chan struct{}
	next   time.Time
	stopCh chan struct{}
}

func (t *fakeTrigger) Start(_ *taskmanager.ExecutionResult) {
	t.next = time.Now().Add(time.Minute)
}

func (t *fakeTrigger) Stop() {
	if t.stopCh != nil {
		select {
		case <-t.stopCh:
		default:
			close(t.stopCh)
		}
	}
}

func (t *fakeTrigger) NextRunTime() time.Time            { return t.next }
func (t *fakeTrigger) Config() taskmanager.TriggerConfig { return t.cfg }
func (t *fakeTrigger) C() <-chan struct{}                { return t.ch }

type fakeServerSettings struct {
	values map[string]string
}

func (s *fakeServerSettings) Get(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *fakeServerSettings) Set(_ context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

type stubTask struct {
	key      string
	triggers []taskmanager.TriggerConfig
}

func (t stubTask) Key() string                        { return t.key }
func (t stubTask) Name() string                       { return t.key }
func (t stubTask) Description() string                { return t.key }
func (t stubTask) Category() taskmanager.TaskCategory { return taskmanager.TaskCategorySystem }
func (t stubTask) IsHidden() bool                     { return false }
func (t stubTask) Execute(context.Context, taskmanager.ProgressReporter) error {
	return nil
}

func (t stubTask) DefaultTriggers() []taskmanager.TriggerConfig {
	return append([]taskmanager.TriggerConfig(nil), t.triggers...)
}

func newFakeTrigger(cfg taskmanager.TriggerConfig) taskmanager.Trigger {
	return &fakeTrigger{
		cfg:    cfg,
		ch:     make(chan struct{}),
		stopCh: make(chan struct{}),
	}
}

func TestTaskManagerStartSeedsCleanupTaskDefaults(t *testing.T) {
	triggerRepo := &fakeTriggerRepository{triggers: map[string][]taskmanager.TriggerConfig{}}
	settings := &fakeServerSettings{
		values: map[string]string{"opslog.cleanup_interval_minutes": "42"},
	}
	manager := taskmanager.New(
		triggerRepo,
		fakeExecutionRepository{},
		newFakeTrigger,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	manager.Register(taskdefs.NewActivityLogCleanupTask(nil, settings, nil))
	manager.Register(taskdefs.NewOperationalLogCleanupTask(nil, settings, nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer manager.Stop()
	defer cancel()
	manager.Start(ctx)

	wantActivity := []taskmanager.TriggerConfig{
		{Type: taskmanager.TriggerTypeStartup},
		{Type: taskmanager.TriggerTypeInterval, IntervalMs: int64((24 * time.Hour) / time.Millisecond)},
	}
	if got := triggerRepo.setCalls["cleanup_activity_log"]; !reflect.DeepEqual(got, wantActivity) {
		t.Fatalf("cleanup_activity_log triggers = %#v, want %#v", got, wantActivity)
	}

	wantOps := []taskmanager.TriggerConfig{
		{Type: taskmanager.TriggerTypeStartup},
		{Type: taskmanager.TriggerTypeInterval, IntervalMs: int64((42 * time.Minute) / time.Millisecond)},
	}
	if got := triggerRepo.setCalls["cleanup_operational_log"]; !reflect.DeepEqual(got, wantOps) {
		t.Fatalf("cleanup_operational_log triggers = %#v, want %#v", got, wantOps)
	}
}

func TestTaskManagerStartPreservesExistingTriggers(t *testing.T) {
	existing := []taskmanager.TriggerConfig{
		{Type: taskmanager.TriggerTypeDaily, TimeOfDay: "03:15"},
	}
	triggerRepo := &fakeTriggerRepository{
		triggers: map[string][]taskmanager.TriggerConfig{
			"cleanup_activity_log": existing,
		},
	}
	manager := taskmanager.New(
		triggerRepo,
		fakeExecutionRepository{},
		newFakeTrigger,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	manager.Register(stubTask{
		key: "cleanup_activity_log",
		triggers: []taskmanager.TriggerConfig{
			{Type: taskmanager.TriggerTypeStartup},
			{Type: taskmanager.TriggerTypeInterval, IntervalMs: int64((24 * time.Hour) / time.Millisecond)},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer manager.Stop()
	defer cancel()
	manager.Start(ctx)

	if _, ok := triggerRepo.setCalls["cleanup_activity_log"]; ok {
		t.Fatalf("expected existing triggers to be preserved without SetTriggers call")
	}

	if got := manager.GetTaskInfo("cleanup_activity_log").Triggers; !reflect.DeepEqual(got, existing) {
		t.Fatalf("worker triggers = %#v, want %#v", got, existing)
	}
}
