package interaction

import (
	"context"
	"fmt"
	"sync"

	"avatar/internal/orchestrator"
	"avatar/internal/persona"
)

// Manager implements orchestrator.Interaction for front-end conversational control.
type Manager struct {
	coord         *orchestrator.Coordinator
	fastPath      *FastPathHandler
	player        orchestrator.SpeechOutput
	runner        orchestrator.TaskRunner
	mu            sync.RWMutex
	onSpeechChunk func(string)
	onPersonaSync func(persona.PersonaSlot)
}

// NewManager creates a new interaction manager.
func NewManager(
	coord *orchestrator.Coordinator,
	fastPath *FastPathHandler,
	player orchestrator.SpeechOutput,
	runner orchestrator.TaskRunner,
) *Manager {
	return &Manager{
		coord:    coord,
		fastPath: fastPath,
		player:   player,
		runner:   runner,
	}
}

// SetSpeechChunkCallback registers callback for speech playback chunks.
func (m *Manager) SetSpeechChunkCallback(cb func(string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onSpeechChunk = cb
}

// SetPersonaSyncCallback registers callback for persona updates.
func (m *Manager) SetPersonaSyncCallback(cb func(persona.PersonaSlot)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onPersonaSync = cb
}

// Submit handles user input: checks fast-path first, then falls back to background task runner.
func (m *Manager) Submit(ctx context.Context, input orchestrator.UserInput) (orchestrator.TurnID, error) {
	// 1. 打断任何正在进行中的活动
	_ = m.Interrupt(ctx, m.coord.SessionID(), "")

	// 2. 建立新轮次与新任务
	turn, task, taskCtx, taskCancel := m.coord.NewTurn(ctx, input)

	// 3. 检查时段流转
	if m.fastPath != nil {
		if changed, newSlot := m.fastPath.CheckSchedule(); changed {
			m.mu.RLock()
			cb := m.onPersonaSync
			m.mu.RUnlock()
			if cb != nil {
				cb(newSlot)
			}
		}
	}

	// 4. 0.1ms 极速快路径处理
	if m.fastPath != nil {
		handled, confirm, newSlot := m.fastPath.Evaluate(input.Text)
		if handled {
			if newSlot != nil {
				m.mu.RLock()
				cb := m.onPersonaSync
				m.mu.RUnlock()
				if cb != nil {
					cb(*newSlot)
				}
			}

			m.coord.SetState(orchestrator.StateSpeaking)
			m.mu.RLock()
			speechCb := m.onSpeechChunk
			m.mu.RUnlock()
			if speechCb != nil {
				speechCb(confirm)
			}

			if m.player != nil {
				_, _ = m.player.Speak(taskCtx, confirm)
			}

			m.coord.SetState(orchestrator.StateIdle)
			m.coord.FinishTask(task.TaskID, orchestrator.TaskStateCompleted, confirm, nil)
			taskCancel()
			return turn.TurnID, nil
		}
	}

	// 5. 非快路径：交由 TaskRunner 执行
	if m.runner == nil {
		taskCancel()
		return turn.TurnID, fmt.Errorf("no task runner configured")
	}

	err := m.runner.Run(taskCtx, *task, func(event orchestrator.TaskEvent) {
		m.coord.DispatchEvent(event)
	})

	return turn.TurnID, err
}

// Interrupt terminates ongoing speech and active task.
func (m *Manager) Interrupt(ctx context.Context, sessionID orchestrator.SessionID, turnID orchestrator.TurnID) error {
	m.coord.CancelActive("user interrupted")
	if m.player != nil {
		_ = m.player.Stop(ctx)
	}
	m.coord.SetState(orchestrator.StateIdle)
	return nil
}
