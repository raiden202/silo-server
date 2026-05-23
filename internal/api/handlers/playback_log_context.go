package handlers

import (
	"net/http"

	"github.com/Silo-Server/silo-server/internal/activitylog"
)

func setPlaybackSessionLogContext(r *http.Request, sessionID string) {
	if sessionID == "" {
		return
	}
	if lc := activitylog.GetPlaybackLogContext(r.Context()); lc != nil {
		lc.PlaybackSessionID = sessionID
	}
}
