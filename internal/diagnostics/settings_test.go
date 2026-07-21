package diagnostics

import (
	"context"
	"testing"
	"time"
)

func TestSeedDefaultsPreservesExistingServerInstanceID(t *testing.T) {
	store := newMemorySettingsStore(map[string]string{
		KeyServerInstanceID: "existing-instance",
	})
	if err := SeedDefaults(context.Background(), store); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	if got := store.values[KeyServerInstanceID]; got != "existing-instance" {
		t.Fatalf("server instance id = %q, want existing-instance", got)
	}
	if got := store.values[KeyUploadsEnabled]; got != defaultUploadsEnabledStr {
		t.Fatalf("uploads enabled default = %q, want %q", got, defaultUploadsEnabledStr)
	}
}

func TestSeedDefaultsGeneratesDedicatedServerInstanceIDWhenMissing(t *testing.T) {
	const jellyfinServerIDKey = "jellyfin_compat.server_id"
	store := newMemorySettingsStore(map[string]string{
		jellyfinServerIDKey: "constant-jellyfin-id",
	})
	if err := SeedDefaults(context.Background(), store); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	got := store.values[KeyServerInstanceID]
	if len(got) != 36 {
		t.Fatalf("server instance id length = %d, want UUID string", len(got))
	}
	if got == store.values[jellyfinServerIDKey] {
		t.Fatal("diagnostics server instance id reused jellyfin_compat.server_id")
	}
	if store.values[jellyfinServerIDKey] != "constant-jellyfin-id" {
		t.Fatalf("jellyfin server id changed to %q", store.values[jellyfinServerIDKey])
	}
}

func TestSeedDefaultsAdoptsConcurrentInstanceIDWinner(t *testing.T) {
	// A store that supports insert-if-absent seeds the instance ID atomically:
	// when another node wins the race, this node adopts the winning value
	// instead of overwriting it.
	store := &racingSettingsStore{
		memorySettingsStore: memorySettingsStore{values: map[string]string{}},
		concurrentWinner:    "winner-instance",
	}
	if err := SeedDefaults(context.Background(), store); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	if got := store.values[KeyServerInstanceID]; got != "winner-instance" {
		t.Fatalf("server instance id = %q, want winner-instance", got)
	}
	if store.setIfAbsentCalls != 1 {
		t.Fatalf("SetIfAbsent calls = %d, want 1", store.setIfAbsentCalls)
	}
}

func TestLoadCleanupIntervalUsesDiagnosticsKey(t *testing.T) {
	store := newMemorySettingsStore(map[string]string{
		KeyCleanupIntervalMinutes: "45",
	})
	if got := LoadCleanupInterval(context.Background(), store); got != 45*time.Minute {
		t.Fatalf("LoadCleanupInterval = %v, want 45m", got)
	}
	if got := LoadCleanupInterval(context.Background(), newMemorySettingsStore(nil)); got != DefaultCleanupIntervalMinutes*time.Minute {
		t.Fatalf("LoadCleanupInterval default = %v, want %d minutes", got, DefaultCleanupIntervalMinutes)
	}
}

func TestLoadSettingsParsesTypedValues(t *testing.T) {
	store := newMemorySettingsStore(map[string]string{
		KeyUploadsEnabled:       "true",
		KeyMaxBundleBytes:       "123",
		KeyMaxUncompressedBytes: "456",
		KeyMaxReportsPerUserDay: "7",
		KeyRetentionDays:        "8",
		KeyMaxBytesPerUser:      "900",
		KeyConsentNoticeVersion: "2",
		KeyServerInstanceID:     "server-1",
	})

	got, err := LoadSettings(context.Background(), store)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if !got.UploadsEnabled ||
		got.MaxBundleBytes != 123 ||
		got.MaxUncompressedBytes != 456 ||
		got.MaxReportsPerUserDay != 7 ||
		got.RetentionDays != 8 ||
		got.MaxBytesPerUser != 900 ||
		got.ConsentNoticeVersion != 2 ||
		got.ServerInstanceID != "server-1" {
		t.Fatalf("LoadSettings parsed unexpected values: %+v", got)
	}
}

func TestLoadSettingsFallsBackForInvalidNumbers(t *testing.T) {
	store := newMemorySettingsStore(map[string]string{
		KeyMaxBundleBytes:       "-1",
		KeyMaxUncompressedBytes: "not-a-number",
		KeyMaxReportsPerUserDay: "0",
		KeyRetentionDays:        "0",
		KeyMaxBytesPerUser:      "-2",
		KeyConsentNoticeVersion: "0",
	})

	got, err := LoadSettings(context.Background(), store)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	want := DefaultSettings()
	if got.MaxBundleBytes != want.MaxBundleBytes ||
		got.MaxUncompressedBytes != want.MaxUncompressedBytes ||
		got.MaxReportsPerUserDay != want.MaxReportsPerUserDay ||
		got.RetentionDays != want.RetentionDays ||
		got.MaxBytesPerUser != want.MaxBytesPerUser ||
		got.ConsentNoticeVersion != want.ConsentNoticeVersion {
		t.Fatalf("LoadSettings = %+v, want defaults %+v", got, want)
	}
}

type memorySettingsStore struct {
	values map[string]string
}

func newMemorySettingsStore(values map[string]string) *memorySettingsStore {
	store := &memorySettingsStore{values: map[string]string{}}
	for k, v := range values {
		store.values[k] = v
	}
	return store
}

func (s *memorySettingsStore) Get(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *memorySettingsStore) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

// racingSettingsStore adds insert-if-absent semantics and simulates another node
// winning the instance-ID seed race via concurrentWinner.
type racingSettingsStore struct {
	memorySettingsStore
	concurrentWinner string
	setIfAbsentCalls int
}

func (s *racingSettingsStore) SetIfAbsent(_ context.Context, key, value string) (bool, error) {
	s.setIfAbsentCalls++
	if existing := s.values[key]; existing != "" {
		return false, nil
	}
	if s.concurrentWinner != "" {
		s.values[key] = s.concurrentWinner
		return false, nil
	}
	s.values[key] = value
	return true, nil
}
