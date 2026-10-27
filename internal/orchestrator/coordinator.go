package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Coordinator manages session, turn, task lifecycle, and state transitions.
type Coordinator struct {
	sessionID      SessionID
	turnCounter    int64
	taskCounter    int64
	eventSeq       int64
	state          State
	activeTurn     *Turn
	activeTask     *Task
	activeCancel   context.CancelFunc
	mu             sync.RWMutex
	onStateChange  func(State)
	onTaskEvent    func(TaskEvent)
}

// NewCoordinator creates a new session coordinator.
func NewCoordinator(sessionID SessionID) *Coordinator {
	if sessionID == "" {
		sessionID = SessionID(fmt.Sprintf("sess-%d", time.Now().UnixNano()))
	}
	return &Coordinator{
		sessionID: sessionID,
		state:     StateIdle,
	}
}

// SessionID returns the session ID.
func (c *Coordinator) SessionID() SessionID {
	return c.sessionID
}

// State returns the current interaction state.
func (c *Coordinator) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// SetState transitions the current state and triggers callback if changed.
func (c *Coordinator) SetState(s State) {
	c.mu.Lock()
	if c.state == s {
		c.mu.Unlock()
		return
	}
	c.state = s
	cb := c.onStateChange
	c.mu.Unlock()

	if cb != nil {
		cb(s)
	}
}

// SetStateChangeCallback registers state transition listener.
func (c *Coordinator) SetStateChangeCallback(cb func(State)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onStateChange = cb
}

// SetEventCallback registers task event listener.
func (c *Coordinator) SetEventCallback(cb func(TaskEvent)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onTaskEvent = cb
}

// NewTurn starts a new interaction turn and associated background task.
// Any previous active task is cancelled.
func (c *Coordinator) NewTurn(parentCtx context.Context, input UserInput) (*Turn, *Task, context.Context, context.CancelFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeCancel != nil {
		c.activeCancel()
		c.activeCancel = nil
	}

	turnNum := atomic.AddInt64(&c.turnCounter, 1)
	taskNum := atomic.AddInt64(&c.taskCounter, 1)
	now := time.Now()

	turnID := TurnID(fmt.Sprintf("%s-turn-%d", c.sessionID, turnNum))
	taskID := TaskID(fmt.Sprintf("%s-task-%d", c.sessionID, taskNum))

	turn := &Turn{
		TurnID:    turnID,
		SessionID: c.sessionID,
		UserGoal:  input.Text,
		Revision:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	ctx, cancel := context.WithCancel(parentCtx)

	task := &Task{
		TaskID:       taskID,
		TurnID:       turnID,
		SessionID:    c.sessionID,
		GoalSnapshot: input.Text,
		CancelFunc:   cancel,
	}

	c.activeTurn = turn
	c.activeTask = task
	c.activeCancel = cancel

	return turn, task, ctx, cancel
}

// ActiveTask returns the currently active task, or nil.
func (c *Coordinator) ActiveTask() *Task {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeTask
}

// ActiveTurn returns the currently active turn, or nil.
func (c *Coordinator) ActiveTurn() *Turn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeTurn
}

// CancelActive cancels the active task and clears active pointers.
func (c *Coordinator) CancelActive(reason string) {
	c.mu.Lock()
	cancel := c.activeCancel
	task := c.activeTask
	turn := c.activeTurn
	c.activeCancel = nil
	c.activeTask = nil
	cb := c.onTaskEvent
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if task != nil && cb != nil {
		seq := atomic.AddInt64(&c.eventSeq, 1)
		cb(TaskEvent{
			Seq:       seq,
			TaskID:    task.TaskID,
			TurnID:    turn.TurnID,
			SessionID: c.sessionID,
			State:     TaskStateCancelled,
			Result:    reason,
			Timestamp: time.Now(),
		})
	}
}

// DispatchEvent validates and dispatches a task event.
// Events for stale or superseded tasks are filtered out.
func (c *Coordinator) DispatchEvent(event TaskEvent) bool {
	c.mu.RLock()
	if c.activeTask == nil || event.TaskID != c.activeTask.TaskID {
		c.mu.RUnlock()
		return false
	}
	cb := c.onTaskEvent
	c.mu.RUnlock()

	if event.Seq == 0 {
		event.Seq = atomic.AddInt64(&c.eventSeq, 1)
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	if cb != nil {
		cb(event)
	}
	return true
}

// FinishTask marks an active task as finished (completed or failed).
func (c *Coordinator) FinishTask(taskID TaskID, state TaskState, result string, err error) {
	c.mu.Lock()
	if c.activeTask != nil && c.activeTask.TaskID == taskID {
		c.activeCancel = nil
		c.activeTask = nil
	}
	c.mu.Unlock()

	c.DispatchEvent(TaskEvent{
		TaskID:    taskID,
		SessionID: c.sessionID,
		State:     state,
		Result:    result,
		Error:     err,
		Timestamp: time.Now(),
	})
}
