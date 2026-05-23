package watchstate

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Silo-Server/silo-server/internal/userstore"
)

type LeafWatchTarget struct {
	MediaItemID     string
	DurationSeconds float64
}

type Service struct {
	storeProvider userstore.UserStoreProvider
	identity      *StableIdentityResolver
}

type PlaybackStopResult struct {
	MediaItemID           string
	DurationSeconds       float64
	FinalPositionSeconds  float64
	Completed             bool
	SkippedBelowMinResume bool
	HistoryID             string
}

type ManualMarkResult struct {
	Entries []userstore.WatchHistoryEntry
}

func NewService(storeProvider userstore.UserStoreProvider) *Service {
	return &Service{storeProvider: storeProvider}
}

func (s *Service) WithStableIdentityResolver(identity *StableIdentityResolver) *Service {
	if s == nil {
		return nil
	}
	s.identity = identity
	return s
}

func (s *Service) RecordManualMarkWatched(ctx context.Context, userID int, profileID string, targets []LeafWatchTarget, watchedAt time.Time) error {
	_, err := s.RecordManualMarkWatchedWithResult(ctx, userID, profileID, targets, watchedAt)
	return err
}

func (s *Service) RecordManualMarkWatchedWithResult(ctx context.Context, userID int, profileID string, targets []LeafWatchTarget, watchedAt time.Time) (ManualMarkResult, error) {
	return s.recordMarkWatched(ctx, userID, profileID, targets, watchedAt, userstore.WatchHistorySourceManual)
}

func (s *Service) RecordManualMarkUnwatched(ctx context.Context, userID int, profileID string, targetIDs []string) error {
	_, err := s.RecordManualMarkUnwatchedWithResult(ctx, userID, profileID, targetIDs)
	return err
}

func (s *Service) RecordManualMarkUnwatchedWithResult(ctx context.Context, userID int, profileID string, targetIDs []string) (ManualMarkResult, error) {
	return s.recordMarkUnwatched(ctx, userID, profileID, targetIDs, userstore.WatchHistorySourceManual)
}

func (s *Service) RecordPlaybackStop(
	ctx context.Context,
	userID int,
	profileID, targetID string,
	duration, position float64,
	watchedAt time.Time,
	hints userstore.VersionHints,
	thresholds userstore.ProgressThresholds,
) (PlaybackStopResult, error) {
	result := PlaybackStopResult{
		MediaItemID:          targetID,
		DurationSeconds:      duration,
		FinalPositionSeconds: position,
	}
	// Below minimum resume threshold — skip both progress and history.
	if duration > 0 && position > 0 && position/duration < userstore.MinResumeFraction(thresholds.MinResumePct) {
		result.SkippedBelowMinResume = true
		return result, nil
	}
	store, err := s.storeForUser(ctx, userID)
	if err != nil {
		return result, err
	}
	if err := store.SetProgress(ctx, profileID, targetID, position, duration, thresholds); err != nil {
		return result, err
	}
	if hints.FileID > 0 {
		if err := store.UpdateProgressHints(ctx, profileID, targetID, hints); err != nil {
			return result, err
		}
	}
	historyID := uuid.NewString()
	entry := userstore.WatchHistoryEntry{
		ID:              historyID,
		ProfileID:       profileID,
		MediaItemID:     targetID,
		WatchedAt:       formatWatchedAt(watchedAt),
		DurationSeconds: duration,
		Completed:       duration > 0 && position/duration > userstore.WatchedFraction(thresholds.WatchedPct),
		Source:          userstore.WatchHistorySourcePlayback,
	}
	s.applyStableIdentity(ctx, &entry)
	if err := store.AddHistory(ctx, entry); err != nil {
		return result, err
	}
	result.Completed = entry.Completed
	result.HistoryID = historyID
	return result, nil
}

func (s *Service) RecordImportedWatch(
	ctx context.Context,
	userID int,
	profileID, targetID string,
	duration, position float64,
	completed bool,
	updatedAt time.Time,
	watchedAt *time.Time,
) (bool, error) {
	return s.RecordImportedWatchWithSource(ctx, userID, profileID, targetID, duration, position, completed, updatedAt, watchedAt, userstore.WatchHistorySourceImport)
}

func (s *Service) RecordImportedWatchWithSource(
	ctx context.Context,
	userID int,
	profileID, targetID string,
	duration, position float64,
	completed bool,
	updatedAt time.Time,
	watchedAt *time.Time,
	source userstore.WatchHistorySource,
) (bool, error) {
	store, err := s.storeForUser(ctx, userID)
	if err != nil {
		return false, err
	}
	if err := store.SetProgressAt(ctx, profileID, targetID, position, duration, completed, updatedAt); err != nil {
		return false, err
	}
	return s.addImportedHistoryIfMissingWithSource(ctx, store, profileID, targetID, duration, completed, watchedAt, source)
}

func (s *Service) RecordImportedHistory(
	ctx context.Context,
	userID int,
	profileID, targetID string,
	duration float64,
	completed bool,
	watchedAt *time.Time,
) (bool, error) {
	return s.RecordImportedHistoryWithSource(ctx, userID, profileID, targetID, duration, completed, watchedAt, userstore.WatchHistorySourceImport)
}

func (s *Service) RecordImportedHistoryWithSource(
	ctx context.Context,
	userID int,
	profileID, targetID string,
	duration float64,
	completed bool,
	watchedAt *time.Time,
	source userstore.WatchHistorySource,
) (bool, error) {
	store, err := s.storeForUser(ctx, userID)
	if err != nil {
		return false, err
	}
	return s.addImportedHistoryIfMissingWithSource(ctx, store, profileID, targetID, duration, completed, watchedAt, source)
}

func (s *Service) RecordImportedMarkUnplayed(
	ctx context.Context,
	userID int,
	profileID, targetID string,
	updatedAt time.Time,
) error {
	store, err := s.storeForUser(ctx, userID)
	if err != nil {
		return err
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	return store.RemoveHistoryItems(ctx, profileID, []string{targetID}, updatedAt)
}

func (s *Service) SetFavorite(
	ctx context.Context,
	userID int,
	profileID, targetID string,
	favorite bool,
) error {
	store, err := s.storeForUser(ctx, userID)
	if err != nil {
		return err
	}
	if favorite {
		return store.AddFavorite(ctx, profileID, targetID)
	}
	return store.RemoveFavorite(ctx, profileID, targetID)
}

func (s *Service) ToggleFavorite(ctx context.Context, userID int, profileID, targetID string) (bool, error) {
	store, err := s.storeForUser(ctx, userID)
	if err != nil {
		return false, err
	}
	current, err := store.IsFavorite(ctx, profileID, targetID)
	if err != nil {
		return false, err
	}
	next := !current
	if next {
		return next, store.AddFavorite(ctx, profileID, targetID)
	}
	return next, store.RemoveFavorite(ctx, profileID, targetID)
}

func (s *Service) RecordJellycompatMarkPlayed(ctx context.Context, userID int, profileID, targetID string, watchedAt time.Time) error {
	_, err := s.recordMarkWatched(ctx, userID, profileID, []LeafWatchTarget{{MediaItemID: targetID}}, watchedAt, userstore.WatchHistorySourceJellycompat)
	return err
}

func (s *Service) RecordJellycompatMarkUnplayed(ctx context.Context, userID int, profileID, targetID string) error {
	_, err := s.recordMarkUnwatched(ctx, userID, profileID, []string{targetID}, userstore.WatchHistorySourceJellycompat)
	return err
}

// RecordJellycompatMarkPlayedBatch marks all the given media items as played in
// a single batch upsert and writes corresponding history entries. Used by
// jellycompat's series-mark-played path to collapse a per-episode loop into
// one progress upsert plus per-episode history inserts (audit 2026-05-01 §2.7).
func (s *Service) RecordJellycompatMarkPlayedBatch(ctx context.Context, userID int, profileID string, targetIDs []string, watchedAt time.Time) error {
	return s.recordMarkWatchedBatch(ctx, userID, profileID, targetIDs, watchedAt, userstore.WatchHistorySourceJellycompat)
}

// RecordJellycompatMarkUnplayedBatch clears progress and deletes
// jellycompat-sourced history entries for all targets in a single statement
// each (audit 2026-05-01 §2.7).
func (s *Service) RecordJellycompatMarkUnplayedBatch(ctx context.Context, userID int, profileID string, targetIDs []string) error {
	return s.recordMarkUnwatchedBatch(ctx, userID, profileID, targetIDs, userstore.WatchHistorySourceJellycompat)
}

func (s *Service) storeForUser(ctx context.Context, userID int) (userstore.UserStore, error) {
	if s == nil || s.storeProvider == nil {
		return nil, fmt.Errorf("watch state store provider is not configured")
	}
	store, err := s.storeProvider.ForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("open user store: %w", err)
	}
	if store == nil {
		return nil, fmt.Errorf("user store not found")
	}
	return store, nil
}

func (s *Service) recordMarkWatched(
	ctx context.Context,
	userID int,
	profileID string,
	targets []LeafWatchTarget,
	watchedAt time.Time,
	source userstore.WatchHistorySource,
) (ManualMarkResult, error) {
	store, err := s.storeForUser(ctx, userID)
	if err != nil {
		return ManualMarkResult{}, err
	}
	entryTime := formatWatchedAt(watchedAt)
	result := ManualMarkResult{Entries: make([]userstore.WatchHistoryEntry, 0, len(targets))}
	for _, target := range targets {
		if err := store.MarkWatched(ctx, profileID, target.MediaItemID, target.DurationSeconds); err != nil {
			return result, err
		}
		histEntry := userstore.WatchHistoryEntry{
			ID:              uuid.NewString(),
			ProfileID:       profileID,
			MediaItemID:     target.MediaItemID,
			WatchedAt:       entryTime,
			DurationSeconds: target.DurationSeconds,
			Completed:       true,
			Source:          source,
		}
		s.applyStableIdentity(ctx, &histEntry)
		if err := store.AddHistory(ctx, histEntry); err != nil {
			return result, err
		}
		result.Entries = append(result.Entries, histEntry)
	}
	return result, nil
}

func (s *Service) recordMarkUnwatched(
	ctx context.Context,
	userID int,
	profileID string,
	targetIDs []string,
	source userstore.WatchHistorySource,
) (ManualMarkResult, error) {
	store, err := s.storeForUser(ctx, userID)
	if err != nil {
		return ManualMarkResult{}, err
	}
	result, err := s.completedHistoryForTargets(ctx, store, profileID, targetIDs, source)
	if err != nil {
		return ManualMarkResult{}, err
	}
	for _, targetID := range targetIDs {
		if err := store.ClearProgress(ctx, profileID, targetID); err != nil {
			return result, err
		}
	}
	return result, store.DeleteHistoryBySource(ctx, profileID, targetIDs, source)
}

func (s *Service) completedHistoryForTargets(
	ctx context.Context,
	store userstore.UserStore,
	profileID string,
	targetIDs []string,
	source userstore.WatchHistorySource,
) (ManualMarkResult, error) {
	if len(targetIDs) == 0 {
		return ManualMarkResult{}, nil
	}
	entries, err := store.ListCompletedHistory(ctx, userstore.CompletedHistoryQuery{
		ProfileID:    profileID,
		MediaItemIDs: targetIDs,
		IncludeSources: []userstore.WatchHistorySource{
			source,
		},
		Limit: len(targetIDs) * 20,
	})
	if err != nil {
		return ManualMarkResult{}, err
	}
	return ManualMarkResult{Entries: entries}, nil
}

func (s *Service) recordMarkWatchedBatch(
	ctx context.Context,
	userID int,
	profileID string,
	targetIDs []string,
	watchedAt time.Time,
	source userstore.WatchHistorySource,
) error {
	if len(targetIDs) == 0 {
		return nil
	}
	store, err := s.storeForUser(ctx, userID)
	if err != nil {
		return err
	}
	if watchedAt.IsZero() {
		watchedAt = time.Now().UTC()
	}
	if err := store.MarkProgressBatch(ctx, profileID, targetIDs, watchedAt); err != nil {
		return err
	}
	// Strategy A (audit 2026-05-01 §2.7): batch the progress upsert because it
	// powers hot Continue-Watching queries. History inserts stay per-target so
	// per-episode stable-identity resolution still applies.
	entryTime := formatWatchedAt(watchedAt)
	for _, targetID := range targetIDs {
		histEntry := userstore.WatchHistoryEntry{
			ProfileID:   profileID,
			MediaItemID: targetID,
			WatchedAt:   entryTime,
			Completed:   true,
			Source:      source,
		}
		s.applyStableIdentity(ctx, &histEntry)
		if err := store.AddHistory(ctx, histEntry); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) recordMarkUnwatchedBatch(
	ctx context.Context,
	userID int,
	profileID string,
	targetIDs []string,
	source userstore.WatchHistorySource,
) error {
	if len(targetIDs) == 0 {
		return nil
	}
	store, err := s.storeForUser(ctx, userID)
	if err != nil {
		return err
	}
	if err := store.ClearProgressBatch(ctx, profileID, targetIDs, time.Now().UTC()); err != nil {
		return err
	}
	return store.DeleteHistoryBySource(ctx, profileID, targetIDs, source)
}

// buildMarkPlayedBatchSQL returns the upsert that marks every media_item_id in
// the unnest($3) array as completed for a given (user, profile). Extracted into
// a helper so a SQL-shape unit test can pin the structure without standing up
// Postgres.
func buildMarkPlayedBatchSQL() (string, []any) {
	return `
        INSERT INTO user_watch_progress
            (user_id, profile_id, media_item_id, completed, position_seconds, duration_seconds, updated_at)
        SELECT $1, $2, mid, TRUE, 0, 0, $4
        FROM unnest($3::text[]) AS mid
        ON CONFLICT (user_id, profile_id, media_item_id) DO UPDATE
        SET completed = TRUE,
            updated_at = EXCLUDED.updated_at
        WHERE user_watch_progress.completed IS DISTINCT FROM TRUE
           OR user_watch_progress.updated_at < EXCLUDED.updated_at`, nil
}

// buildMarkUnplayedBatchSQL returns the update that clears the completed flag
// and resets position to 0 for every media_item_id in $3 for a given
// (user, profile). Pairs with the jellycompat unplayed-batch path; the matching
// history-row deletion uses DeleteHistoryBySource which already takes a slice.
//
// The `completed = TRUE OR position_seconds <> 0` predicate clears partially-
// watched rows in addition to fully-completed ones — the prior single-item
// ClearProgress path DELETE-d unconditionally, so any non-default state must
// be cleared (otherwise "mark unplayed" leaves resume position untouched).
// Skip rows already in the target state to avoid pointless writes.
func buildMarkUnplayedBatchSQL() (string, []any) {
	return `
        UPDATE user_watch_progress
        SET completed = FALSE, position_seconds = 0, updated_at = $4
        WHERE user_id = $1 AND profile_id = $2
          AND media_item_id = ANY($3::text[])
          AND (completed = TRUE OR position_seconds <> 0)`, nil
}

func (s *Service) addImportedHistoryIfMissing(
	ctx context.Context,
	store userstore.UserStore,
	profileID, targetID string,
	duration float64,
	completed bool,
	watchedAt *time.Time,
) (bool, error) {
	return s.addImportedHistoryIfMissingWithSource(ctx, store, profileID, targetID, duration, completed, watchedAt, userstore.WatchHistorySourceImport)
}

func (s *Service) addImportedHistoryIfMissingWithSource(
	ctx context.Context,
	store userstore.UserStore,
	profileID, targetID string,
	duration float64,
	completed bool,
	watchedAt *time.Time,
	source userstore.WatchHistorySource,
) (bool, error) {
	if watchedAt == nil || watchedAt.IsZero() {
		return false, nil
	}
	entry := userstore.WatchHistoryEntry{
		ProfileID:       profileID,
		MediaItemID:     targetID,
		WatchedAt:       watchedAt.UTC().Format(time.RFC3339),
		DurationSeconds: duration,
		Completed:       completed,
		Source:          source,
	}
	s.applyStableIdentity(ctx, &entry)
	return store.AddHistoryIfMissing(ctx, entry)
}

func (s *Service) applyStableIdentity(ctx context.Context, entry *userstore.WatchHistoryEntry) {
	if s == nil || s.identity == nil || entry == nil {
		return
	}
	entry.Identity = s.identity.ResolveHistoryIdentity(ctx, entry.MediaItemID)
}

func formatWatchedAt(watchedAt time.Time) string {
	if watchedAt.IsZero() {
		return ""
	}
	return watchedAt.UTC().Format(time.RFC3339)
}
