package orchestrator

import (
	"context"
	"testing"
	"time"
)

func TestCoordinatorStateTransitions(t *testing.T) {
	c := NewCoordinator("session-1")
	if c.State() != StateIdle {
		t.Fatalf("expected initial state %s, got %s", StateIdle, c.State())
	}

	var stateHistory []State
	c.SetStateChangeCallback(func(s State) {
		stateHistory = append(stateHistory, s)
	})

	c.SetState(StateListening)
	c.SetState(StateThinking)
	c.SetState(StateSpeaking)
	c.SetState(StateIdle)

	if len(stateHistory) != 4 {
		t.Fatalf("expected 4 transitions, got %d", len(stateHistory))
	}
	if stateHistory[0] != StateListening || stateHistory[3] != StateIdle {
		t.Fatalf("unexpected state history: %v", stateHistory)
	}
}

func TestCoordinatorNewTurnAndCancellation(t *testing.T) {
	c := NewCoordinator("session-1")

	input := UserInput{
		InputType: InputTypeText,
		Text:      "测试问题",
		Stability: StabilityFinal,
	}

	turn, task, ctx, cancel := c.NewTurn(context.Background(), input)
	if turn == nil || task == nil {
		t.Fatal("expected non-nil turn and task")
	}
	if turn.SessionID != "session-1" || task.SessionID != "session-1" {
		t.Fatalf("unexpected session ID in turn/task: %s", turn.SessionID)
	}
	if task.GoalSnapshot != "测试问题" {
		t.Fatalf("expected goal snapshot '测试问题', got '%s'", task.GoalSnapshot)
	}

	activeTask := c.ActiveTask()
	if activeTask == nil || activeTask.TaskID != task.TaskID {
		t.Fatal("expected active task to match created task")
	}

	// Cancel active task
	c.CancelActive("user interrupted")

	// Context should be cancelled
	select {
	case <-ctx.Done():
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected task context to be cancelled after CancelActive")
	}
	cancel() // clean up
}

func TestCoordinatorLateEventFiltering(t *testing.T) {
	c := NewCoordinator("session-1")

	var dispatched []TaskEvent
	c.SetEventCallback(func(e TaskEvent) {
		dispatched = append(dispatched, e)
	})

	input1 := UserInput{InputType: InputTypeText, Text: "任务一", Stability: StabilityFinal}
	_, task1, _, _ := c.NewTurn(context.Background(), input1)

	// Valid event for task1
	e1 := TaskEvent{
		TaskID:    task1.TaskID,
		TurnID:    task1.TurnID,
		SessionID: "session-1",
		State:     TaskStateExecuting,
		Progress:  0.5,
	}
	if !c.DispatchEvent(e1) {
		t.Fatal("expected e1 to be dispatched")
	}

	// Create task2, which supersedes task1
	input2 := UserInput{InputType: InputTypeText, Text: "任务二", Stability: StabilityFinal}
	_, task2, _, _ := c.NewTurn(context.Background(), input2)

	// Late event for task1 should be rejected
	lateEvent := TaskEvent{
		TaskID:    task1.TaskID,
		TurnID:    task1.TurnID,
		SessionID: "session-1",
		State:     TaskStateCompleted,
		Result:    "迟到结果",
	}
	if c.DispatchEvent(lateEvent) {
		t.Fatal("expected late event for task1 to be rejected after task2 started")
	}

	// Valid event for task2
	e2 := TaskEvent{
		TaskID:    task2.TaskID,
		TurnID:    task2.TurnID,
		SessionID: "session-1",
		State:     TaskStateExecuting,
		Progress:  0.8,
	}
	if !c.DispatchEvent(e2) {
		t.Fatal("expected e2 to be dispatched")
	}

	if len(dispatched) != 2 {
		t.Fatalf("expected exactly 2 dispatched events, got %d", len(dispatched))
	}
	if dispatched[0].TaskID != task1.TaskID || dispatched[1].TaskID != task2.TaskID {
		t.Fatalf("unexpected dispatched events: %+v", dispatched)
	}
}
