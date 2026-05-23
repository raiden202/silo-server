package playback

import (
	"errors"
	"time"
)

var ErrCommandDispatchUnavailable = errors.New("command dispatch unavailable")

type SessionLookup interface {
	GetSession(sessionID string) (*Session, error)
	GetUserSessions(userID int) []*Session
	AllSessions() []*Session
}

// CommandDispatchResult describes a single attempted delivery.
type CommandDispatchResult struct {
	SessionID   string
	CommandID   string
	Delivered   bool
	Tracked     bool
	Deadline    time.Duration
	DispatchErr error
}

// CommandDispatcher routes realtime commands through the active hub and tracks
// commands that require deadline-enforced fallbacks.
type CommandDispatcher struct {
	sessions SessionLookup
	hub      *RealtimeHub
	tracker  *CommandTracker
}

func NewCommandDispatcher(sessions SessionLookup, hub *RealtimeHub, tracker *CommandTracker) *CommandDispatcher {
	return &CommandDispatcher{
		sessions: sessions,
		hub:      hub,
		tracker:  tracker,
	}
}

func (d *CommandDispatcher) DispatchToSession(
	command CommandEnvelope,
	deadline time.Duration,
	fallback func(),
) CommandDispatchResult {
	result := CommandDispatchResult{
		SessionID: command.SessionID,
		CommandID: command.CommandID,
		Deadline:  deadline,
	}
	if d == nil || d.hub == nil || d.sessions == nil {
		result.DispatchErr = ErrCommandDispatchUnavailable
		return result
	}
	if err := command.Validate(); err != nil {
		result.DispatchErr = err
		return result
	}
	if _, err := d.sessions.GetSession(command.SessionID); err != nil {
		result.DispatchErr = err
		return result
	}
	if deadline > 0 && d.tracker != nil {
		d.tracker.Track(command.CommandID, deadline, fallback)
		result.Tracked = true
	}
	if err := d.hub.Send(command.SessionID, command); err != nil {
		if result.Tracked && d.tracker != nil {
			d.tracker.Result(command.CommandID)
		}
		result.DispatchErr = err
		return result
	}
	result.Delivered = true
	return result
}

func (d *CommandDispatcher) DispatchToUser(
	userID int,
	build func(session *Session) (CommandEnvelope, time.Duration, func(), error),
) []CommandDispatchResult {
	if d == nil || d.sessions == nil || build == nil {
		return nil
	}

	sessions := d.sessions.GetUserSessions(userID)
	results := make([]CommandDispatchResult, 0, len(sessions))
	for _, session := range sessions {
		command, deadline, fallback, err := build(session)
		if err != nil {
			results = append(results, CommandDispatchResult{
				SessionID:   session.ID,
				CommandID:   command.CommandID,
				Deadline:    deadline,
				DispatchErr: err,
			})
			continue
		}
		results = append(results, d.DispatchToSession(command, deadline, fallback))
	}
	return results
}

func (d *CommandDispatcher) DispatchToAll(
	build func(session *Session) (CommandEnvelope, time.Duration, func(), error),
) []CommandDispatchResult {
	if d == nil || d.sessions == nil || build == nil {
		return nil
	}

	sessions := d.sessions.AllSessions()
	results := make([]CommandDispatchResult, 0, len(sessions))
	for _, session := range sessions {
		command, deadline, fallback, err := build(session)
		if err != nil {
			results = append(results, CommandDispatchResult{
				SessionID:   session.ID,
				CommandID:   command.CommandID,
				Deadline:    deadline,
				DispatchErr: err,
			})
			continue
		}
		results = append(results, d.DispatchToSession(command, deadline, fallback))
	}
	return results
}
