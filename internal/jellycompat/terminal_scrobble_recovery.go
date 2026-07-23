package jellycompat

import (
	"context"
	"log/slog"
	"time"
)

const compatTerminalRecoveryBatchSize = 100

// StartTerminalScrobbleRecovery resumes terminal provider events that survived
// a server exit after staging but before delivery. The first scan runs at
// startup; periodic scans also recover expired leases from crashed peers.
func StartTerminalScrobbleRecovery(
	ctx context.Context,
	store CompatPlaybackStore,
	scrobbler PlaybackWatchScrobbler,
	interval time.Duration,
) {
	if store == nil || scrobbler == nil {
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	handler := &PlaybackHandler{playbackStore: store, WatchScrobbler: scrobbler}
	run := func() {
		if err := recoverPendingTerminalScrobbles(ctx, handler); err != nil {
			slog.WarnContext(ctx, "recover jellycompat terminal scrobbles failed",
				"component", "jellycompat", "error", err)
		}
	}
	go func() {
		run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

func recoverPendingTerminalScrobbles(ctx context.Context, handler *PlaybackHandler) error {
	pending, err := handler.playbackStore.ListPendingTerminals(ctx, compatTerminalRecoveryBatchSize)
	if err != nil {
		return err
	}
	for _, session := range pending {
		handler.deliverCompatTerminal(
			ctx,
			session.ID,
			session.CompatToken,
			session.TerminalAuthoritative,
			session.ExpiresAt,
			0,
			false,
		)
	}
	return nil
}
