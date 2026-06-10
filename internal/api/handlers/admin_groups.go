package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Silo-Server/silo-server/internal/auth"
	"github.com/Silo-Server/silo-server/internal/models"
)

// GroupStore defines the group repository operations the AdminGroupsHandler
// needs, so tests can substitute a fake.
type GroupStore interface {
	List(ctx context.Context) ([]auth.GroupWithMemberCount, error)
	GetByID(ctx context.Context, id int) (*models.Group, error)
	MemberCount(ctx context.Context, groupID int) (int, error)
	Create(ctx context.Context, input models.CreateGroupInput) (*models.Group, error)
	Update(ctx context.Context, id int, input models.UpdateGroupInput) (*models.Group, error)
	Delete(ctx context.Context, id int) error
	ListMembers(ctx context.Context, groupID, offset, limit int) ([]auth.GroupMember, int, error)
	AddMember(ctx context.Context, groupID, userID int) error
	RemoveMember(ctx context.Context, groupID, userID int) error
}

// AdminGroupsHandler handles admin-only group management endpoints.
type AdminGroupsHandler struct {
	store GroupStore
}

// NewAdminGroupsHandler creates an AdminGroupsHandler backed by the given
// group store.
func NewAdminGroupsHandler(store GroupStore) *AdminGroupsHandler {
	return &AdminGroupsHandler{store: store}
}

// --- JSON field wrappers ---

// optionalIntSliceField distinguishes an absent JSON field from an explicit
// null or array value. Absent = don't update; null = nil slice ("all
// libraries"); [] = empty slice ("none").
type optionalIntSliceField struct {
	Set   bool
	Value []int
}

func (f *optionalIntSliceField) UnmarshalJSON(data []byte) error {
	f.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		f.Value = nil
		return nil
	}
	return json.Unmarshal(data, &f.Value)
}

// Ptr returns nil when the field was absent, otherwise a pointer to the
// decoded slice (which may itself be nil for JSON null).
func (f optionalIntSliceField) Ptr() *[]int {
	if !f.Set {
		return nil
	}
	value := f.Value
	return &value
}

// optionalStringSliceField distinguishes an absent JSON field from an
// explicit null or array value. JSON null decodes to an empty slice.
type optionalStringSliceField struct {
	Set   bool
	Value []string
}

func (f *optionalStringSliceField) UnmarshalJSON(data []byte) error {
	f.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		f.Value = []string{}
		return nil
	}
	return json.Unmarshal(data, &f.Value)
}

// --- Request/Response types ---

// createAdminGroupRequest represents the JSON body for POST /admin/groups.
type createAdminGroupRequest struct {
	Name                     string   `json:"name"`
	Description              string   `json:"description"`
	Permissions              []string `json:"permissions"`
	LibraryIDs               []int    `json:"library_ids"` // null/absent = all libraries
	MaxStreams               *int     `json:"max_streams"`
	MaxTranscodes            *int     `json:"max_transcodes"`
	MaxProfiles              *int     `json:"max_profiles"`
	MaxPlaybackQuality       *string  `json:"max_playback_quality"`
	DownloadAllowed          *bool    `json:"download_allowed"`
	DownloadTranscodeAllowed *bool    `json:"download_transcode_allowed"`
}

// updateAdminGroupRequest represents the JSON body for PATCH /admin/groups/{id}.
// Absent fields are left unchanged.
type updateAdminGroupRequest struct {
	Name                     *string                  `json:"name,omitempty"`
	Description              *string                  `json:"description,omitempty"`
	Permissions              optionalStringSliceField `json:"permissions,omitempty"`
	LibraryIDs               optionalIntSliceField    `json:"library_ids,omitempty"`
	MaxStreams               *int                     `json:"max_streams,omitempty"`
	MaxTranscodes            *int                     `json:"max_transcodes,omitempty"`
	MaxProfiles              *int                     `json:"max_profiles,omitempty"`
	MaxPlaybackQuality       *string                  `json:"max_playback_quality,omitempty"`
	DownloadAllowed          *bool                    `json:"download_allowed,omitempty"`
	DownloadTranscodeAllowed *bool                    `json:"download_transcode_allowed,omitempty"`
}

// adminGroupResponse represents a group in admin JSON responses.
type adminGroupResponse struct {
	ID                       int       `json:"id"`
	Slug                     string    `json:"slug"`
	Name                     string    `json:"name"`
	Description              string    `json:"description"`
	BuiltIn                  bool      `json:"built_in"`
	Permissions              []string  `json:"permissions"`
	LibraryIDs               []int     `json:"library_ids"` // null = all libraries
	MaxStreams               int       `json:"max_streams"`
	MaxTranscodes            int       `json:"max_transcodes"`
	MaxProfiles              int       `json:"max_profiles"`
	MaxPlaybackQuality       string    `json:"max_playback_quality"`
	DownloadAllowed          bool      `json:"download_allowed"`
	DownloadTranscodeAllowed bool      `json:"download_transcode_allowed"`
	MemberCount              int       `json:"member_count"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type adminGroupListResponse struct {
	Groups []adminGroupResponse `json:"groups"`
}

type adminGroupMemberRow struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Enabled  bool   `json:"enabled"`
}

type adminGroupMembersResponse struct {
	Members []adminGroupMemberRow `json:"members"`
	Total   int                   `json:"total"`
	Offset  int                   `json:"offset"`
	Limit   int                   `json:"limit"`
}

func toAdminGroupResponse(g *models.Group, memberCount int) adminGroupResponse {
	return adminGroupResponse{
		ID:                       g.ID,
		Slug:                     g.Slug,
		Name:                     g.Name,
		Description:              g.Description,
		BuiltIn:                  g.BuiltIn,
		Permissions:              append([]string{}, g.Permissions...),
		LibraryIDs:               g.LibraryIDs,
		MaxStreams:               g.MaxStreams,
		MaxTranscodes:            g.MaxTranscodes,
		MaxProfiles:              g.MaxProfiles,
		MaxPlaybackQuality:       g.MaxPlaybackQuality,
		DownloadAllowed:          g.DownloadAllowed,
		DownloadTranscodeAllowed: g.DownloadTranscodeAllowed,
		MemberCount:              memberCount,
		CreatedAt:                g.CreatedAt,
		UpdatedAt:                g.UpdatedAt,
	}
}

// --- Helpers ---

// writeGroupError maps group repository sentinel errors to API responses.
func writeGroupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrGroupNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Group not found")
	case errors.Is(err, auth.ErrBuiltInGroup):
		writeError(w, http.StatusConflict, "built_in_group", "Built-in groups cannot be deleted")
	case errors.Is(err, auth.ErrAdminPermRequired):
		writeError(w, http.StatusConflict, "admin_permission_required", "The administrators group must keep the admin permission")
	case errors.Is(err, auth.ErrLastAdministrator):
		writeError(w, http.StatusConflict, "last_administrator", "Cannot remove the last enabled administrator")
	case errors.Is(err, auth.ErrDuplicate):
		writeError(w, http.StatusConflict, "duplicate", "A group with that name already exists")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Group operation failed")
	}
}

func parseGroupIDParam(w http.ResponseWriter, r *http.Request, name string) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, name))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid "+name)
		return 0, false
	}
	return id, true
}

// --- Handler methods ---

// HandleList handles GET /admin/groups.
func (h *AdminGroupsHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list groups")
		return
	}
	resp := adminGroupListResponse{Groups: make([]adminGroupResponse, 0, len(groups))}
	for i := range groups {
		resp.Groups = append(resp.Groups, toAdminGroupResponse(&groups[i].Group, groups[i].MemberCount))
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleCreate handles POST /admin/groups.
func (h *AdminGroupsHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req createAdminGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "Group name is required")
		return
	}
	permissions, err := auth.NormalizePermissions(req.Permissions)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	group, err := h.store.Create(r.Context(), models.CreateGroupInput{
		Name:                     name,
		Description:              req.Description,
		Permissions:              permissions,
		LibraryIDs:               req.LibraryIDs,
		MaxStreams:               req.MaxStreams,
		MaxTranscodes:            req.MaxTranscodes,
		MaxProfiles:              req.MaxProfiles,
		MaxPlaybackQuality:       req.MaxPlaybackQuality,
		DownloadAllowed:          req.DownloadAllowed,
		DownloadTranscodeAllowed: req.DownloadTranscodeAllowed,
	})
	if err != nil {
		writeGroupError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAdminGroupResponse(group, 0))
}

// HandleGet handles GET /admin/groups/{id}.
func (h *AdminGroupsHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	id, ok := parseGroupIDParam(w, r, "id")
	if !ok {
		return
	}
	group, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		writeGroupError(w, err)
		return
	}
	memberCount, err := h.store.MemberCount(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to count group members")
		return
	}
	writeJSON(w, http.StatusOK, toAdminGroupResponse(group, memberCount))
}

// HandleUpdate handles PATCH /admin/groups/{id}.
func (h *AdminGroupsHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseGroupIDParam(w, r, "id")
	if !ok {
		return
	}
	var req updateAdminGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "Group name cannot be empty")
		return
	}

	input := models.UpdateGroupInput{
		Name:                     req.Name,
		Description:              req.Description,
		LibraryIDs:               req.LibraryIDs.Ptr(),
		MaxStreams:               req.MaxStreams,
		MaxTranscodes:            req.MaxTranscodes,
		MaxProfiles:              req.MaxProfiles,
		MaxPlaybackQuality:       req.MaxPlaybackQuality,
		DownloadAllowed:          req.DownloadAllowed,
		DownloadTranscodeAllowed: req.DownloadTranscodeAllowed,
	}
	if req.Permissions.Set {
		permissions, err := auth.NormalizePermissions(req.Permissions.Value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		input.Permissions = &permissions
	}

	group, err := h.store.Update(r.Context(), id, input)
	if err != nil {
		writeGroupError(w, err)
		return
	}
	memberCount, err := h.store.MemberCount(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to count group members")
		return
	}
	writeJSON(w, http.StatusOK, toAdminGroupResponse(group, memberCount))
}

// HandleDelete handles DELETE /admin/groups/{id}.
func (h *AdminGroupsHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseGroupIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.store.Delete(r.Context(), id); err != nil {
		writeGroupError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleListMembers handles GET /admin/groups/{id}/members.
func (h *AdminGroupsHandler) HandleListMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := parseGroupIDParam(w, r, "id")
	if !ok {
		return
	}

	q := r.URL.Query()
	offset := 0
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	limit := 50
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	members, total, err := h.store.ListMembers(r.Context(), id, offset, limit)
	if err != nil {
		writeGroupError(w, err)
		return
	}
	resp := adminGroupMembersResponse{
		Members: make([]adminGroupMemberRow, 0, len(members)),
		Total:   total,
		Offset:  offset,
		Limit:   limit,
	}
	for _, m := range members {
		resp.Members = append(resp.Members, adminGroupMemberRow{
			UserID:   m.UserID,
			Username: m.Username,
			Email:    m.Email,
			Enabled:  m.Enabled,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleAddMember handles PUT /admin/groups/{id}/members/{userID}.
func (h *AdminGroupsHandler) HandleAddMember(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseGroupIDParam(w, r, "id")
	if !ok {
		return
	}
	userID, ok := parseGroupIDParam(w, r, "userID")
	if !ok {
		return
	}
	if err := h.store.AddMember(r.Context(), groupID, userID); err != nil {
		writeGroupError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleRemoveMember handles DELETE /admin/groups/{id}/members/{userID}.
func (h *AdminGroupsHandler) HandleRemoveMember(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseGroupIDParam(w, r, "id")
	if !ok {
		return
	}
	userID, ok := parseGroupIDParam(w, r, "userID")
	if !ok {
		return
	}
	if err := h.store.RemoveMember(r.Context(), groupID, userID); err != nil {
		writeGroupError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
