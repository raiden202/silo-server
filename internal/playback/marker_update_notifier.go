package playback

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Silo-Server/silo-server/internal/models"
)

type markerUpdateSessionLookup interface {
	GetSessionsByMediaFileID(fileID int) []*Session
}

// MarkerUpdateNotifier publishes live marker updates to active playback sessions.
type MarkerUpdateNotifier struct {
	sessions markerUpdateSessionLookup
	hub      *RealtimeHub
}

func NewMarkerUpdateNotifier(sessions markerUpdateSessionLookup, hub *RealtimeHub) *MarkerUpdateNotifier {
	if sessions == nil || hub == nil {
		return nil
	}
	return &MarkerUpdateNotifier{
		sessions: sessions,
		hub:      hub,
	}
}

func (n *MarkerUpdateNotifier) MarkersUpdated(_ context.Context, file *models.MediaFile) {
	if n == nil || file == nil || file.ID <= 0 {
		return
	}

	var intro *TimeRangePayload
	if file.IntroStart != nil && file.IntroEnd != nil {
		intro = &TimeRangePayload{Start: *file.IntroStart, End: *file.IntroEnd}
	}
	var credits *TimeRangePayload
	if file.CreditsStart != nil && file.CreditsEnd != nil {
		credits = &TimeRangePayload{Start: *file.CreditsStart, End: *file.CreditsEnd}
	}
	if intro == nil && credits == nil {
		return
	}

	for _, session := range n.sessions.GetSessionsByMediaFileID(file.ID) {
		if session == nil || session.ID == "" || !session.HasRealtimeConnection {
			continue
		}
		event, err := NewMarkersUpdatedEvent(session.ID, file.ID, intro, credits)
		if err != nil {
			slog.Warn(
				"failed to encode markers updated realtime event",
				"session_id",
				session.ID,
				"file_id",
				file.ID,
				"error",
				err,
			)
			continue
		}
		if err := n.hub.Send(session.ID, event); err != nil && !errors.Is(err, ErrRealtimeConnectionNotFound) {
			slog.Warn(
				"failed to deliver markers updated realtime event",
				"session_id",
				session.ID,
				"file_id",
				file.ID,
				"error",
				err,
			)
		}
	}
}
