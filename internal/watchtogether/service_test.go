package watchtogether

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Silo-Server/silo-server/internal/models"
	"github.com/Silo-Server/silo-server/internal/playback"
)

type stubRepo struct {
	room Room
}

func (s *stubRepo) CreateRoom(_ context.Context, room Room) (*Room, error) {
	s.room = room
	copy := s.room
	return &copy, nil
}
func (s *stubRepo) GetRoomByID(context.Context, string) (*Room, error) {
	room := s.room
	return &room, nil
}
func (s *stubRepo) GetRoomByCode(context.Context, string) (*Room, error) {
	room := s.room
	return &room, nil
}
func (s *stubRepo) GetRoomByJoinToken(context.Context, string) (*Room, error) {
	room := s.room
	return &room, nil
}
func (s *stubRepo) UpdatePolicy(_ context.Context, _ string, policy GuestControlPolicy, generation int64, expectedGeneration int64) (*Room, error) {
	if s.room.Generation != expectedGeneration {
		return nil, ErrRoomStateConflict
	}
	s.room.GuestControlPolicy = policy
	s.room.Generation = generation
	room := s.room
	return &room, nil
}
func (s *stubRepo) UpdateAnchor(
	_ context.Context,
	_ string,
	positionSeconds float64,
	isPaused bool,
	playbackState RoomPlaybackState,
	resumeOnReady bool,
	updatedAt time.Time,
	generation int64,
	expectedGeneration int64,
) (*Room, error) {
	if s.room.Generation != expectedGeneration {
		return nil, ErrRoomStateConflict
	}
	s.room.AnchorPositionSeconds = positionSeconds
	s.room.IsPaused = isPaused
	s.room.PlaybackState = playbackState
	s.room.ResumeOnReady = resumeOnReady
	s.room.AnchorUpdatedAt = updatedAt
	s.room.Generation = generation
	room := s.room
	return &room, nil
}
func (s *stubRepo) CloseRoom(_ context.Context, _ string, closedAt time.Time) (*Room, error) {
	s.room.Phase = RoomPhaseEnded
	s.room.ClosedAt = &closedAt
	room := s.room
	return &room, nil
}
func (s *stubRepo) UpdateSelection(
	_ context.Context,
	_ string,
	selection SelectItemInput,
	phase RoomPhase,
	playbackState RoomPlaybackState,
	resumeOnReady bool,
	anchorPosition float64,
	isPaused bool,
	anchorUpdatedAt time.Time,
	selectionRevision int64,
	generation int64,
	expectedGeneration int64,
) (*Room, error) {
	if s.room.Generation != expectedGeneration {
		return nil, ErrRoomStateConflict
	}
	s.room.Phase = phase
	s.room.PlaybackState = playbackState
	s.room.ResumeOnReady = resumeOnReady
	s.room.SelectedContentID = &selection.ContentID
	s.room.SelectedFileID = selection.FileID
	s.room.SelectedLibraryID = selection.LibraryID
	s.room.AnchorPositionSeconds = anchorPosition
	s.room.IsPaused = isPaused
	s.room.AnchorUpdatedAt = anchorUpdatedAt
	s.room.SelectionRevision = selectionRevision
	s.room.Generation = generation
	room := s.room
	return &room, nil
}

type stubSessions struct {
	session *playback.Session
}

func (s *stubSessions) GetSession(string) (*playback.Session, error) {
	if s.session == nil {
		return nil, playback.ErrSessionNotFound
	}
	cp := *s.session
	return &cp, nil
}

type stubFiles struct {
	file *models.MediaFile
}

func (s *stubFiles) GetByID(context.Context, int) (*models.MediaFile, error) {
	if s.file == nil {
		return nil, errors.New("missing file")
	}
	cp := *s.file
	return &cp, nil
}

type dispatchedCommand struct {
	sessionID string
	name      playback.CommandName
}

type stubDispatcher struct {
	commands []dispatchedCommand
}

func (s *stubDispatcher) DispatchToSession(
	command playback.CommandEnvelope,
	_ time.Duration,
	_ func(),
) playback.CommandDispatchResult {
	s.commands = append(s.commands, dispatchedCommand{
		sessionID: command.SessionID,
		name:      command.Name,
	})
	return playback.CommandDispatchResult{}
}

type stubConn struct{}

func (stubConn) WriteJSON(any) error { return nil }
func (stubConn) Close() error        { return nil }

type recordingConn struct {
	payloads []map[string]any
}

func (c *recordingConn) WriteJSON(v any) error {
	payload, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	copyPayload := make(map[string]any, len(payload))
	for key, value := range payload {
		copyPayload[key] = value
	}
	c.payloads = append(c.payloads, copyPayload)
	return nil
}

func (c *recordingConn) Close() error { return nil }

type stubSelectionResolver struct {
	resolved *ResolvedSelection
	err      error
}

func (s *stubSelectionResolver) ResolveSelection(context.Context, int, string, SelectItemInput) (*ResolvedSelection, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.resolved, nil
}

func baseRoom(now time.Time) Room {
	return Room{
		ID:                    "room-1",
		Code:                  "ROOM1234",
		JoinToken:             "TOKEN1234",
		HostUserID:            7,
		HostProfileID:         "host",
		Phase:                 RoomPhasePlaying,
		PlaybackState:         RoomPlaybackStatePlaying,
		ResumeOnReady:         false,
		SelectionMode:         RoomSelectionModeHostPick,
		SelectionRevision:     1,
		SelectedContentID:     stringPtr("movie-1"),
		GuestControlPolicy:    GuestControlPolicyHostOnly,
		AnchorPositionSeconds: 10,
		IsPaused:              false,
		AnchorUpdatedAt:       now.Add(-10 * time.Second),
		Generation:            1,
		CreatedAt:             now.Add(-20 * time.Second),
	}
}

func newServiceForTest(now time.Time, repo *stubRepo, sessions *stubSessions, files *stubFiles, dispatcher *stubDispatcher, resolver WatchTogetherSelectionResolver) *Service {
	service := NewService(repo, sessions, files, dispatcher, resolver, nil)
	service.hostDisconnectTTL = time.Hour
	service.now = func() time.Time { return now }
	service.rooms[repo.room.ID] = &liveRoom{
		room:    repo.room,
		members: make(map[string]*memberState),
	}
	return service
}

func stringPtr(value string) *string {
	return &value
}

func TestGuestPlayPausePolicyStillRejectsGuestSeek(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 0, 20, 0, time.UTC)
	repo := &stubRepo{room: baseRoom(now)}
	repo.room.GuestControlPolicy = GuestControlPolicyGuestPlayPause
	dispatcher := &stubDispatcher{}
	conn := &recordingConn{}
	service := newServiceForTest(
		now,
		repo,
		&stubSessions{},
		&stubFiles{file: &models.MediaFile{ID: 42, ContentID: "movie-1"}},
		dispatcher,
		nil,
	)
	service.rooms[repo.room.ID].members[buildMemberKey(8, "guest")] = &memberState{
		userID:     8,
		profileID:  "guest",
		sessionID:  "session-1",
		connection: conn,
	}

	position := 120.0
	_, err := service.HandleTransportRequest(context.Background(), repo.room.ID, 8, "guest", TransportRequest{
		Action:          TransportActionSeek,
		PositionSeconds: &position,
		IsPaused:        false,
	})
	if !errors.Is(err, ErrTransportNotAllowed) {
		t.Fatalf("HandleTransportRequest(guest seek) error = %v, want ErrTransportNotAllowed", err)
	}
}

func TestGuestDriftTriggersCorrection(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 0, 20, 0, time.UTC)
	repo := &stubRepo{room: baseRoom(now)}
	dispatcher := &stubDispatcher{}
	conn := &recordingConn{}
	service := newServiceForTest(
		now,
		repo,
		&stubSessions{session: &playback.Session{
			ID:          "session-1",
			UserID:      8,
			ProfileID:   "guest",
			MediaFileID: 42,
		}},
		&stubFiles{file: &models.MediaFile{ID: 42, ContentID: "movie-1"}},
		dispatcher,
		nil,
	)
	service.rooms[repo.room.ID].members[buildMemberKey(8, "guest")] = &memberState{
		userID:     8,
		profileID:  "guest",
		sessionID:  "session-1",
		connection: conn,
	}

	_, err := service.HandleStateReport(context.Background(), repo.room.ID, 8, "guest", StateReport{
		SessionID:       "session-1",
		PositionSeconds: 2,
		IsPaused:        false,
	})
	if err != nil {
		t.Fatalf("HandleStateReport() error = %v", err)
	}

	if len(conn.payloads) == 0 {
		t.Fatal("expected correction commands to be dispatched")
	}
}

func TestHostAttachKeepsRoomSelectionAnchor(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 0, 20, 0, time.UTC)
	repo := &stubRepo{room: baseRoom(now)}
	repo.room.AnchorPositionSeconds = 0
	repo.room.IsPaused = true
	repo.room.AnchorUpdatedAt = now
	repo.room.Generation = 1
	dispatcher := &stubDispatcher{}
	conn := &recordingConn{}
	service := newServiceForTest(
		now,
		repo,
		&stubSessions{session: &playback.Session{
			ID:          "session-1",
			UserID:      7,
			ProfileID:   "host",
			MediaFileID: 42,
			Position:    318,
			IsPaused:    false,
		}},
		&stubFiles{file: &models.MediaFile{ID: 42, ContentID: "movie-1"}},
		dispatcher,
		nil,
	)
	service.rooms[repo.room.ID].members[buildMemberKey(7, "host")] = &memberState{
		userID:     7,
		profileID:  "host",
		connection: conn,
	}

	snapshot, err := service.AttachSession(context.Background(), repo.room.ID, 7, "host", "session-1")
	if err != nil {
		t.Fatalf("AttachSession() error = %v", err)
	}

	if snapshot.AnchorPositionSeconds != 0 {
		t.Fatalf("snapshot anchor = %v, want 0", snapshot.AnchorPositionSeconds)
	}
	if !snapshot.IsPaused {
		t.Fatal("snapshot should remain paused")
	}
	if repo.room.Generation != 1 {
		t.Fatalf("generation = %d, want 1", repo.room.Generation)
	}
	if len(conn.payloads) == 0 {
		t.Fatal("expected room sync commands to be dispatched")
	}
}

func TestHostAttachKeepsRoomSelectionAnchorEvenWhenGuestAttached(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 0, 20, 0, time.UTC)
	repo := &stubRepo{room: baseRoom(now)}
	repo.room.AnchorPositionSeconds = 0
	repo.room.IsPaused = true
	repo.room.AnchorUpdatedAt = now
	repo.room.Generation = 1
	dispatcher := &stubDispatcher{}
	service := newServiceForTest(
		now,
		repo,
		&stubSessions{session: &playback.Session{
			ID:          "host-session",
			UserID:      7,
			ProfileID:   "host",
			MediaFileID: 42,
			Position:    318,
			IsPaused:    false,
		}},
		&stubFiles{file: &models.MediaFile{ID: 42, ContentID: "movie-1"}},
		dispatcher,
		nil,
	)
	service.rooms[repo.room.ID].members[buildMemberKey(8, "guest")] = &memberState{
		userID:     8,
		profileID:  "guest",
		sessionID:  "guest-session",
		connection: stubConn{},
	}
	service.rooms[repo.room.ID].members[buildMemberKey(7, "host")] = &memberState{
		userID:     7,
		profileID:  "host",
		connection: stubConn{},
	}

	snapshot, err := service.AttachSession(context.Background(), repo.room.ID, 7, "host", "host-session")
	if err != nil {
		t.Fatalf("AttachSession() error = %v", err)
	}

	if snapshot.AnchorPositionSeconds != 0 {
		t.Fatalf("snapshot anchor = %v, want 0", snapshot.AnchorPositionSeconds)
	}
	if !snapshot.IsPaused {
		t.Fatal("snapshot should remain paused")
	}
}

func TestAttachSessionAcceptsEpisodeContentID(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 20, 0, time.UTC)
	repo := &stubRepo{room: baseRoom(now)}
	repo.room.SelectedContentID = stringPtr("episode-19")
	repo.room.AnchorPositionSeconds = 0
	repo.room.IsPaused = true
	repo.room.AnchorUpdatedAt = now
	repo.room.Generation = 1
	dispatcher := &stubDispatcher{}
	service := newServiceForTest(
		now,
		repo,
		&stubSessions{session: &playback.Session{
			ID:          "host-session",
			UserID:      7,
			ProfileID:   "host",
			MediaFileID: 42,
			Position:    75,
			IsPaused:    false,
		}},
		&stubFiles{file: &models.MediaFile{
			ID:        42,
			ContentID: "series-1",
			EpisodeID: "episode-19",
		}},
		dispatcher,
		nil,
	)
	service.rooms[repo.room.ID].members[buildMemberKey(7, "host")] = &memberState{
		userID:     7,
		profileID:  "host",
		connection: stubConn{},
	}

	snapshot, err := service.AttachSession(context.Background(), repo.room.ID, 7, "host", "host-session")
	if err != nil {
		t.Fatalf("AttachSession() error = %v", err)
	}

	if snapshot.AttachedSessionID != "host-session" {
		t.Fatalf("attached session = %q, want host-session", snapshot.AttachedSessionID)
	}
	if snapshot.AnchorPositionSeconds != 0 {
		t.Fatalf("snapshot anchor = %v, want 0", snapshot.AnchorPositionSeconds)
	}
}

func TestCreateRoomStartsInLobbyWithoutSelection(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 20, 0, time.UTC)
	repo := &stubRepo{}
	service := NewService(repo, &stubSessions{}, &stubFiles{}, &stubDispatcher{}, nil, nil)
	service.now = func() time.Time { return now }

	room, err := service.CreateRoom(context.Background(), CreateRoomInput{
		HostUserID:    7,
		HostProfileID: "host",
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	if room.Phase != RoomPhaseLobby {
		t.Fatalf("phase = %q, want %q", room.Phase, RoomPhaseLobby)
	}
	if room.SelectionRevision != 0 {
		t.Fatalf("selection revision = %d, want 0", room.SelectionRevision)
	}
	if room.SelectedContentID != nil {
		t.Fatalf("selected content = %v, want nil", *room.SelectedContentID)
	}
}

func TestHostCanSelectItemFromLobby(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 20, 0, time.UTC)
	repo := &stubRepo{room: Room{
		ID:                 "room-1",
		Code:               "ROOM1234",
		JoinToken:          "TOKEN1234",
		HostUserID:         7,
		HostProfileID:      "host",
		Phase:              RoomPhaseLobby,
		SelectionMode:      RoomSelectionModeHostPick,
		GuestControlPolicy: GuestControlPolicyHostOnly,
		IsPaused:           true,
		AnchorUpdatedAt:    now,
		Generation:         1,
		CreatedAt:          now,
	}}
	service := newServiceForTest(
		now,
		repo,
		&stubSessions{},
		&stubFiles{},
		&stubDispatcher{},
		&stubSelectionResolver{resolved: &ResolvedSelection{
			ContentID: "movie-2",
			FileID:    intPtr(55),
			LibraryID: intPtr(6),
		}},
	)

	snapshot, err := service.SelectItem(context.Background(), "room-1", 7, "host", SelectItemInput{
		ContentID: "movie-2",
	})
	if err != nil {
		t.Fatalf("SelectItem() error = %v", err)
	}

	if snapshot.Phase != RoomPhasePlaying {
		t.Fatalf("phase = %q, want %q", snapshot.Phase, RoomPhasePlaying)
	}
	if snapshot.SelectionRevision != 1 {
		t.Fatalf("selection revision = %d, want 1", snapshot.SelectionRevision)
	}
	if snapshot.SelectedContentID == nil || *snapshot.SelectedContentID != "movie-2" {
		t.Fatalf("selected content = %v, want movie-2", snapshot.SelectedContentID)
	}
	if snapshot.AnchorPositionSeconds != 0 {
		t.Fatalf("anchor = %v, want 0", snapshot.AnchorPositionSeconds)
	}
	if !snapshot.IsPaused {
		t.Fatal("room should stay paused while waiting for participants to get ready")
	}
	if snapshot.PlaybackState != RoomPlaybackStateWaiting {
		t.Fatalf("playback state = %q, want %q", snapshot.PlaybackState, RoomPlaybackStateWaiting)
	}
}

func TestGuestCannotSelectItem(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 20, 0, time.UTC)
	repo := &stubRepo{room: baseRoom(now)}
	service := newServiceForTest(
		now,
		repo,
		&stubSessions{},
		&stubFiles{},
		&stubDispatcher{},
		&stubSelectionResolver{resolved: &ResolvedSelection{ContentID: "movie-2"}},
	)

	_, err := service.SelectItem(context.Background(), "room-1", 8, "guest", SelectItemInput{
		ContentID: "movie-2",
	})
	if !errors.Is(err, ErrRoomForbidden) {
		t.Fatalf("SelectItem() error = %v, want ErrRoomForbidden", err)
	}
}

func TestSelectItemRejectsInvalidSelection(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 20, 0, time.UTC)
	repo := &stubRepo{room: baseRoom(now)}
	service := newServiceForTest(
		now,
		repo,
		&stubSessions{},
		&stubFiles{},
		&stubDispatcher{},
		&stubSelectionResolver{err: ErrInvalidSelection},
	)

	_, err := service.SelectItem(context.Background(), "room-1", 7, "host", SelectItemInput{
		ContentID: "series-1",
	})
	if !errors.Is(err, ErrInvalidSelection) {
		t.Fatalf("SelectItem() error = %v, want ErrInvalidSelection", err)
	}
}

func TestAttachSessionEnforcesSelectedFileID(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 20, 0, time.UTC)
	repo := &stubRepo{room: baseRoom(now)}
	repo.room.SelectedFileID = intPtr(99)
	dispatcher := &stubDispatcher{}
	service := newServiceForTest(
		now,
		repo,
		&stubSessions{session: &playback.Session{
			ID:          "host-session",
			UserID:      7,
			ProfileID:   "host",
			MediaFileID: 42,
		}},
		&stubFiles{file: &models.MediaFile{ID: 42, ContentID: "movie-1"}},
		dispatcher,
		nil,
	)
	service.rooms[repo.room.ID].members[buildMemberKey(7, "host")] = &memberState{
		userID:     7,
		profileID:  "host",
		connection: stubConn{},
	}

	_, err := service.AttachSession(context.Background(), repo.room.ID, 7, "host", "host-session")
	if !errors.Is(err, ErrSessionMismatch) {
		t.Fatalf("AttachSession() error = %v, want ErrSessionMismatch", err)
	}
}

func intPtr(value int) *int {
	return &value
}
