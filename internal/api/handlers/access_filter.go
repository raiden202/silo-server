package handlers

import (
	"net/http"

	"github.com/Silo-Server/silo-server/internal/access"
	apimw "github.com/Silo-Server/silo-server/internal/api/middleware"
	"github.com/Silo-Server/silo-server/internal/catalog"
)

func requestAccessFilter(r *http.Request) catalog.AccessFilter {
	if scope, ok := access.GetScope(r.Context()); ok {
		return catalog.AccessFilter{
			AllowedLibraryIDs:  scope.AllowedLibraryIDs,
			DisabledLibraryIDs: scope.DisabledLibraryIDs,
			MaxContentRating:   scope.MaxContentRating,
			MaxPlaybackQuality: scope.MaxPlaybackQuality,
			UserID:             apimw.GetUserID(r.Context()),
			ProfileID:          apimw.GetProfileID(r.Context()),
		}
	}
	return catalog.AccessFilter{
		UserID:    apimw.GetUserID(r.Context()),
		ProfileID: apimw.GetProfileID(r.Context()),
	}
}
