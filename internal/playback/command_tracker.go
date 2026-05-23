package playback

import (
	"sync"
	"time"
)

// CommandState describes the lifecycle state of a tracked realtime command.
type CommandState string

const (
	CommandStatePending   CommandState = "pending"
	CommandStateAccepted  CommandState = "accepted"
	CommandStateCompleted CommandState = "completed"
)

type commandTrackerEntry struct {
	state    CommandState
	timer    *time.Timer
	fallback func()
}

// CommandTracker tracks in-flight realtime commands by command_id.
type CommandTracker struct {
	mu       sync.Mutex
	closed   bool
	commands map[string]*commandTrackerEntry
}

// NewCommandTracker creates an empty command tracker.
func NewCommandTracker() *CommandTracker {
	return &CommandTracker{
		commands: make(map[string]*commandTrackerEntry),
	}
}

// Track records a new command as pending and arms its fallback deadline.
func (t *CommandTracker) Track(commandID string, deadline time.Duration, fallback func()) {
	if t == nil || commandID == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	if _, exists := t.commands[commandID]; exists {
		return
	}

	entry := &commandTrackerEntry{
		state:    CommandStatePending,
		fallback: fallback,
	}
	t.commands[commandID] = entry
	entry.timer = time.AfterFunc(deadline, func() {
		t.fireDeadline(commandID)
	})
}

// Ack marks a tracked command as accepted.
func (t *CommandTracker) Ack(commandID string) {
	t.setState(commandID, CommandStateAccepted)
}

// Result marks a tracked command as completed.
func (t *CommandTracker) Result(commandID string) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	entry, ok := t.commands[commandID]
	if !ok {
		t.mu.Unlock()
		return
	}
	entry.state = CommandStateCompleted
	timer := entry.timer
	delete(t.commands, commandID)
	t.mu.Unlock()

	if timer != nil {
		timer.Stop()
	}
}

// Status reports the current tracked state for a command.
func (t *CommandTracker) Status(commandID string) (CommandState, bool) {
	if t == nil || commandID == "" {
		return "", false
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	entry, ok := t.commands[commandID]
	if !ok {
		return "", false
	}
	return entry.state, true
}

// Close stops all deadline timers and prevents future tracking.
func (t *CommandTracker) Close() {
	if t == nil {
		return
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	commands := t.commands
	t.commands = make(map[string]*commandTrackerEntry)
	t.mu.Unlock()

	for _, entry := range commands {
		if entry.timer != nil {
			entry.timer.Stop()
		}
	}
}

func (t *CommandTracker) setState(commandID string, state CommandState) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	entry, ok := t.commands[commandID]
	if !ok {
		t.mu.Unlock()
		return
	}
	if entry.state != CommandStateCompleted {
		entry.state = state
	}
	t.mu.Unlock()
}

func (t *CommandTracker) fireDeadline(commandID string) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	entry, ok := t.commands[commandID]
	if !ok {
		t.mu.Unlock()
		return
	}
	if entry.state == CommandStateCompleted {
		delete(t.commands, commandID)
		t.mu.Unlock()
		return
	}
	fallback := entry.fallback
	delete(t.commands, commandID)
	t.mu.Unlock()

	if fallback != nil {
		fallback()
	}
}
