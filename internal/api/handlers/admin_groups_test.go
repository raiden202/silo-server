package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Silo-Server/silo-server/internal/auth"
	"github.com/Silo-Server/silo-server/internal/models"
)

// fakeGroupStore is a configurable in-memory GroupStore that records the
// inputs handlers pass to it.
type fakeGroupStore struct {
	listGroups  []auth.GroupWithMemberCount
	listErr     error
	getGroup    *models.Group
	getErr      error
	memberCount int
	countErr    error

	createInput *models.CreateGroupInput
	createGroup *models.Group
	createErr   error

	updateID    int
	updateInput *models.UpdateGroupInput
	updateGroup *models.Group
	updateErr   error

	deleteID  int
	deleteErr error

	membersGroupID int
	membersOffset  int
	membersLimit   int
	members        []auth.GroupMember
	membersTotal   int
	membersErr     error

	addGroupID    int
	addUserID     int
	addErr        error
	removeGroupID int
	removeUserID  int
	removeErr     error
}

func (f *fakeGroupStore) List(context.Context) ([]auth.GroupWithMemberCount, error) {
	return f.listGroups, f.listErr
}

func (f *fakeGroupStore) GetByID(_ context.Context, id int) (*models.Group, error) {
	return f.getGroup, f.getErr
}

func (f *fakeGroupStore) MemberCount(_ context.Context, groupID int) (int, error) {
	return f.memberCount, f.countErr
}

func (f *fakeGroupStore) Create(_ context.Context, input models.CreateGroupInput) (*models.Group, error) {
	f.createInput = &input
	return f.createGroup, f.createErr
}

func (f *fakeGroupStore) Update(_ context.Context, id int, input models.UpdateGroupInput) (*models.Group, error) {
	f.updateID = id
	f.updateInput = &input
	return f.updateGroup, f.updateErr
}

func (f *fakeGroupStore) Delete(_ context.Context, id int) error {
	f.deleteID = id
	return f.deleteErr
}

func (f *fakeGroupStore) ListMembers(_ context.Context, groupID, offset, limit int) ([]auth.GroupMember, int, error) {
	f.membersGroupID = groupID
	f.membersOffset = offset
	f.membersLimit = limit
	return f.members, f.membersTotal, f.membersErr
}

func (f *fakeGroupStore) AddMember(_ context.Context, groupID, userID int) error {
	f.addGroupID = groupID
	f.addUserID = userID
	return f.addErr
}

func (f *fakeGroupStore) RemoveMember(_ context.Context, groupID, userID int) error {
	f.removeGroupID = groupID
	f.removeUserID = userID
	return f.removeErr
}

// newGroupsTestRouter mounts the handler routes the same way the API router
// does so tests exercise real chi URL params.
func newGroupsTestRouter(store *fakeGroupStore) chi.Router {
	h := NewAdminGroupsHandler(store)
	r := chi.NewRouter()
	r.Get("/groups", h.HandleList)
	r.Post("/groups", h.HandleCreate)
	r.Get("/groups/{id}", h.HandleGet)
	r.Patch("/groups/{id}", h.HandleUpdate)
	r.Delete("/groups/{id}", h.HandleDelete)
	r.Get("/groups/{id}/members", h.HandleListMembers)
	r.Put("/groups/{id}/members/{userID}", h.HandleAddMember)
	r.Delete("/groups/{id}/members/{userID}", h.HandleRemoveMember)
	return r
}

func doGroupsRequest(t *testing.T, router chi.Router, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshaling request body: %v", err)
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeGroupsJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decoding response %q: %v", rec.Body.String(), err)
	}
	return out
}

func assertGroupsError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, status, rec.Body.String())
	}
	body := decodeGroupsJSON(t, rec)
	if body["error"] != code {
		t.Fatalf("error code = %v, want %q (body %q)", body["error"], code, rec.Body.String())
	}
}

func testGroup() *models.Group {
	return &models.Group{
		ID:                 7,
		Slug:               "film-club",
		Name:               "Film Club",
		Description:        "Movie nights",
		Permissions:        []string{"marker_edit"},
		LibraryIDs:         nil,
		MaxStreams:         2,
		MaxTranscodes:      1,
		MaxProfiles:        4,
		MaxPlaybackQuality: "1080p",
		CreatedAt:          time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		UpdatedAt:          time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func TestAdminGroupsList(t *testing.T) {
	store := &fakeGroupStore{
		listGroups: []auth.GroupWithMemberCount{
			{Group: models.Group{ID: 1, Slug: "administrators", Name: "Administrators", BuiltIn: true, Permissions: []string{"admin"}}, MemberCount: 1},
			{Group: models.Group{ID: 2, Slug: "users", Name: "Users", BuiltIn: true, LibraryIDs: []int{3}}, MemberCount: 12},
		},
	}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodGet, "/groups", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	body := decodeGroupsJSON(t, rec)
	groups, ok := body["groups"].([]any)
	if !ok || len(groups) != 2 {
		t.Fatalf("groups = %v, want 2 entries", body["groups"])
	}
	first := groups[0].(map[string]any)
	if first["member_count"] != float64(1) {
		t.Fatalf("member_count = %v, want 1", first["member_count"])
	}
	if first["library_ids"] != nil {
		t.Fatalf("library_ids = %v, want null for all-libraries", first["library_ids"])
	}
	second := groups[1].(map[string]any)
	if second["member_count"] != float64(12) {
		t.Fatalf("member_count = %v, want 12", second["member_count"])
	}
	if ids, ok := second["library_ids"].([]any); !ok || len(ids) != 1 || ids[0] != float64(3) {
		t.Fatalf("library_ids = %v, want [3]", second["library_ids"])
	}
}

func TestAdminGroupsCreate(t *testing.T) {
	store := &fakeGroupStore{createGroup: testGroup()}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodPost, "/groups", map[string]any{
		"name":        " Film Club ",
		"description": "Movie nights",
		"permissions": []string{"marker_edit"},
		"library_ids": []int{1, 2},
		"max_streams": 2,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}
	if store.createInput == nil {
		t.Fatal("store.Create was not called")
	}
	if store.createInput.Name != "Film Club" {
		t.Fatalf("Name = %q, want trimmed name", store.createInput.Name)
	}
	if len(store.createInput.LibraryIDs) != 2 {
		t.Fatalf("LibraryIDs = %v, want [1 2]", store.createInput.LibraryIDs)
	}
	if store.createInput.MaxStreams == nil || *store.createInput.MaxStreams != 2 {
		t.Fatalf("MaxStreams = %v, want 2", store.createInput.MaxStreams)
	}
	if store.createInput.MaxTranscodes != nil {
		t.Fatalf("MaxTranscodes = %v, want nil (omitted)", store.createInput.MaxTranscodes)
	}
	body := decodeGroupsJSON(t, rec)
	if body["slug"] != "film-club" {
		t.Fatalf("slug = %v, want film-club", body["slug"])
	}
	if body["member_count"] != float64(0) {
		t.Fatalf("member_count = %v, want 0 for new group", body["member_count"])
	}
}

func TestAdminGroupsCreateEmptyName(t *testing.T) {
	store := &fakeGroupStore{}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodPost, "/groups", map[string]any{
		"name": "   ",
	})
	assertGroupsError(t, rec, http.StatusBadRequest, "bad_request")
	if store.createInput != nil {
		t.Fatal("store.Create should not be called for empty name")
	}
}

func TestAdminGroupsCreateUnknownPermission(t *testing.T) {
	store := &fakeGroupStore{}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodPost, "/groups", map[string]any{
		"name":        "Film Club",
		"permissions": []string{"server_owner"},
	})
	assertGroupsError(t, rec, http.StatusBadRequest, "bad_request")
	if store.createInput != nil {
		t.Fatal("store.Create should not be called for unknown permission")
	}
}

func TestAdminGroupsCreateDuplicate(t *testing.T) {
	store := &fakeGroupStore{createErr: auth.ErrDuplicate}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodPost, "/groups", map[string]any{
		"name": "Film Club",
	})
	assertGroupsError(t, rec, http.StatusConflict, "duplicate")
}

func TestAdminGroupsGet(t *testing.T) {
	store := &fakeGroupStore{getGroup: testGroup(), memberCount: 5}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodGet, "/groups/7", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	body := decodeGroupsJSON(t, rec)
	if body["id"] != float64(7) {
		t.Fatalf("id = %v, want 7", body["id"])
	}
	if body["member_count"] != float64(5) {
		t.Fatalf("member_count = %v, want 5", body["member_count"])
	}
}

func TestAdminGroupsGetNotFound(t *testing.T) {
	store := &fakeGroupStore{getErr: auth.ErrGroupNotFound}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodGet, "/groups/999", nil)
	assertGroupsError(t, rec, http.StatusNotFound, "not_found")
}

func TestAdminGroupsGetInvalidID(t *testing.T) {
	store := &fakeGroupStore{}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodGet, "/groups/abc", nil)
	assertGroupsError(t, rec, http.StatusBadRequest, "bad_request")
}

func TestAdminGroupsPatchStripAdminPermission(t *testing.T) {
	store := &fakeGroupStore{updateErr: auth.ErrAdminPermRequired}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodPatch, "/groups/1", map[string]any{
		"permissions": []string{"marker_edit"},
	})
	assertGroupsError(t, rec, http.StatusConflict, "admin_permission_required")
}

func TestAdminGroupsPatchUnknownPermission(t *testing.T) {
	store := &fakeGroupStore{}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodPatch, "/groups/7", map[string]any{
		"permissions": []string{"server_owner"},
	})
	assertGroupsError(t, rec, http.StatusBadRequest, "bad_request")
	if store.updateInput != nil {
		t.Fatal("store.Update should not be called for unknown permission")
	}
}

func TestAdminGroupsPatchLibraryIDsSemantics(t *testing.T) {
	tests := []struct {
		name string
		body string
		want func(t *testing.T, input *models.UpdateGroupInput)
	}{
		{
			name: "absent means no change",
			body: `{"name":"Renamed"}`,
			want: func(t *testing.T, input *models.UpdateGroupInput) {
				if input.LibraryIDs != nil {
					t.Fatalf("LibraryIDs = %v, want nil (no change)", input.LibraryIDs)
				}
			},
		},
		{
			name: "null means all libraries",
			body: `{"library_ids":null}`,
			want: func(t *testing.T, input *models.UpdateGroupInput) {
				if input.LibraryIDs == nil {
					t.Fatal("LibraryIDs = nil, want pointer to nil slice (all libraries)")
				}
				if *input.LibraryIDs != nil {
					t.Fatalf("*LibraryIDs = %v, want nil slice (all libraries)", *input.LibraryIDs)
				}
			},
		},
		{
			name: "empty array means no libraries",
			body: `{"library_ids":[]}`,
			want: func(t *testing.T, input *models.UpdateGroupInput) {
				if input.LibraryIDs == nil {
					t.Fatal("LibraryIDs = nil, want pointer to empty slice (no libraries)")
				}
				if *input.LibraryIDs == nil || len(*input.LibraryIDs) != 0 {
					t.Fatalf("*LibraryIDs = %v, want non-nil empty slice", *input.LibraryIDs)
				}
			},
		},
		{
			name: "explicit ids",
			body: `{"library_ids":[1,2]}`,
			want: func(t *testing.T, input *models.UpdateGroupInput) {
				if input.LibraryIDs == nil {
					t.Fatal("LibraryIDs = nil, want pointer to [1 2]")
				}
				ids := *input.LibraryIDs
				if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
					t.Fatalf("*LibraryIDs = %v, want [1 2]", ids)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeGroupStore{updateGroup: testGroup()}
			router := newGroupsTestRouter(store)
			req := httptest.NewRequest(http.MethodPatch, "/groups/7", bytes.NewReader([]byte(tt.body)))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
			}
			if store.updateInput == nil {
				t.Fatal("store.Update was not called")
			}
			tt.want(t, store.updateInput)
		})
	}
}

func TestAdminGroupsDeleteBuiltIn(t *testing.T) {
	store := &fakeGroupStore{deleteErr: auth.ErrBuiltInGroup}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodDelete, "/groups/1", nil)
	assertGroupsError(t, rec, http.StatusConflict, "built_in_group")
}

func TestAdminGroupsDelete(t *testing.T) {
	store := &fakeGroupStore{}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodDelete, "/groups/7", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body %q)", rec.Code, rec.Body.String())
	}
	if store.deleteID != 7 {
		t.Fatalf("deleted id = %d, want 7", store.deleteID)
	}
}

func TestAdminGroupsListMembersPagination(t *testing.T) {
	store := &fakeGroupStore{
		members: []auth.GroupMember{
			{UserID: 3, Username: "alice", Email: "alice@example.com", Enabled: true},
		},
		membersTotal: 42,
	}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodGet, "/groups/7/members?offset=10&limit=5", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if store.membersGroupID != 7 || store.membersOffset != 10 || store.membersLimit != 5 {
		t.Fatalf("ListMembers(%d, %d, %d), want (7, 10, 5)",
			store.membersGroupID, store.membersOffset, store.membersLimit)
	}
	body := decodeGroupsJSON(t, rec)
	if body["total"] != float64(42) || body["offset"] != float64(10) || body["limit"] != float64(5) {
		t.Fatalf("pagination = total %v offset %v limit %v, want 42/10/5",
			body["total"], body["offset"], body["limit"])
	}
	members, ok := body["members"].([]any)
	if !ok || len(members) != 1 {
		t.Fatalf("members = %v, want 1 entry", body["members"])
	}
	member := members[0].(map[string]any)
	if member["user_id"] != float64(3) || member["username"] != "alice" ||
		member["email"] != "alice@example.com" || member["enabled"] != true {
		t.Fatalf("member = %v, want alice row", member)
	}
}

func TestAdminGroupsListMembersDefaults(t *testing.T) {
	store := &fakeGroupStore{}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodGet, "/groups/7/members", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if store.membersOffset != 0 || store.membersLimit != 50 {
		t.Fatalf("ListMembers offset %d limit %d, want defaults 0/50",
			store.membersOffset, store.membersLimit)
	}
	body := decodeGroupsJSON(t, rec)
	if members, ok := body["members"].([]any); !ok || len(members) != 0 {
		t.Fatalf("members = %v, want empty array (not null)", body["members"])
	}
}

func TestAdminGroupsAddMember(t *testing.T) {
	store := &fakeGroupStore{}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodPut, "/groups/7/members/12", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body %q)", rec.Code, rec.Body.String())
	}
	if store.addGroupID != 7 || store.addUserID != 12 {
		t.Fatalf("AddMember(%d, %d), want (7, 12)", store.addGroupID, store.addUserID)
	}
}

func TestAdminGroupsRemoveLastAdministrator(t *testing.T) {
	store := &fakeGroupStore{removeErr: auth.ErrLastAdministrator}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodDelete, "/groups/1/members/12", nil)
	assertGroupsError(t, rec, http.StatusConflict, "last_administrator")
}

func TestAdminGroupsRemoveMember(t *testing.T) {
	store := &fakeGroupStore{}
	rec := doGroupsRequest(t, newGroupsTestRouter(store), http.MethodDelete, "/groups/7/members/12", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body %q)", rec.Code, rec.Body.String())
	}
	if store.removeGroupID != 7 || store.removeUserID != 12 {
		t.Fatalf("RemoveMember(%d, %d), want (7, 12)", store.removeGroupID, store.removeUserID)
	}
}
