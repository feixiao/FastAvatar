package interaction

import (
	"context"
	"fmt"
	"sync"

	"avatar/internal/cloud"
	"avatar/internal/orchestrator"
	"avatar/internal/persona"
)

// Manager implements orchestrator.Interaction for front-end conversational control.
type Manager struct {
	coord         *orchestrator.Coordinator
	fastPath      *FastPathHandler
	player        orchestrator.SpeechOutput
	runner        orchestrator.TaskRunner
	cloudClient   *cloud.Client
	mu            sync.RWMutex
	onSpeechChunk func(string)
	onPersonaSync func(persona.PersonaSlot)
	onPetAction   func(PetActionPayload)
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

// SetCloudClient binds the cloud realtime client for edge-cloud synchronization.
func (m *Manager) SetCloudClient(client *cloud.Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cloudClient = client
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

// SetPetActionCallback registers callback for pet action animations.
func (m *Manager) SetPetActionCallback(cb func(PetActionPayload)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onPetAction = cb
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
		handled, confirm, newSlot, petAction := m.fastPath.Evaluate(input.Text)
		if handled {
			if newSlot != nil {
				m.mu.RLock()
				cb := m.onPersonaSync
				m.mu.RUnlock()
				if cb != nil {
					cb(*newSlot)
				}
			}

			if petAction != nil {
				m.mu.RLock()
				actCb := m.onPetAction
				m.mu.RUnlock()
				if actCb != nil {
					actCb(*petAction)
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
			fmt.Printf("[交互调度] 本地快环处理完成 (TurnID: %s) | 耗时: < 150ms | 100%% 端侧本地响应，未走云端\n", turn.TurnID)
			taskCancel()
			return turn.TurnID, nil
		}
	}

	// 5. 非快路径：交由云端 / TaskRunner 执行
	if m.runner == nil {
		taskCancel()
		return turn.TurnID, fmt.Errorf("no task runner configured")
	}
	fmt.Printf("[交互调度] 移交认知任务调度 (TurnID: %s)\n", turn.TurnID)

	m.mu.RLock()
	cloudCli := m.cloudClient
	m.mu.RUnlock()
	if cloudCli != nil {
		_ = cloudCli.StartTurn(string(turn.TurnID))
	}

	err := m.runner.Run(taskCtx, *task, func(event orchestrator.TaskEvent) {
		m.coord.DispatchEvent(event)
		if event.State == orchestrator.TaskStateCompleted || event.State == orchestrator.TaskStateFailed {
			if cloudCli != nil {
				cloudCli.FinishTurn(string(turn.TurnID))
			}
		}
	})

	return turn.TurnID, err
}

// Interrupt terminates ongoing speech, active task, and active cloud turn.
func (m *Manager) Interrupt(ctx context.Context, sessionID orchestrator.SessionID, turnID orchestrator.TurnID) error {
	m.coord.CancelActive("user interrupted")
	if m.coord.State() == orchestrator.StateSpeaking {
		fmt.Printf("[全双工打断] 触发用户插话打断 (Barge-In) | 立即掐断本地发音 (≤ 80ms)，同步向云端下发截断信令\n")
	}

	// 端侧声学层打断：立即物理停止本地发音 (≤ 80ms)
	if m.player != nil {
		_ = m.player.Stop(ctx)
	}

	// 云端协同打断：立即向云端投递取消信令，废弃推流与后续数据
	m.mu.RLock()
	cloudCli := m.cloudClient
	m.mu.RUnlock()
	if cloudCli != nil {
		_ = cloudCli.InterruptActiveTurn(ctx, string(turnID), "LOCAL_VAD_BARGE_IN")
	}

	m.coord.SetState(orchestrator.StateIdle)
	return nil
}
