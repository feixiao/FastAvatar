package engine

import (
	"context"

	"avatar/internal/cloud"
	"avatar/internal/config"
	"avatar/internal/interaction"
	"avatar/internal/llm"
	"avatar/internal/memory"
	"avatar/internal/orchestrator"
	"avatar/internal/persona"
	"avatar/internal/task"
	"avatar/internal/tools"
	"avatar/internal/tts"
)

// State re-exports orchestrator.State for backward compatibility.
type State = orchestrator.State

const (
	StateIdle      = orchestrator.StateIdle
	StateListening = orchestrator.StateListening
	StateThinking  = orchestrator.StateThinking
	StateSpeaking  = orchestrator.StateSpeaking
)

// Pipeline integrates session coordination, front-end interaction, tools, and background cognitive execution.
type Pipeline struct {
	coordinator *orchestrator.Coordinator
	interaction *interaction.Manager
	fastPath    *interaction.FastPathHandler
	taskRunner  *task.Runner
	toolsReg    *tools.Registry
}

// NewPipeline creates a new modular Pipeline composed of coordinator, interaction manager, task runner and tools.
func NewPipeline(
	cfg config.AppConfig,
	synthesizer tts.Synthesizer,
	vault *memory.Vault,
	personaMgr *persona.Manager,
	bondMgr *persona.BondManager,
) *Pipeline {
	if synthesizer == nil {
		synthesizer = tts.NewMacSaySynthesizer(cfg.TTS.Voice, cfg.TTS.Rate)
	}

	coord := orchestrator.NewCoordinator("")

	toolsReg := tools.NewRegistry()
	tools.RegisterMemoryTools(toolsReg, vault)
	tools.RegisterPersonaTools(toolsReg, personaMgr)

	llmClient := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey)
	taskRunner := task.NewRunner(cfg.LLM, llmClient, synthesizer, vault, personaMgr, bondMgr, coord)

	fastPath := interaction.NewFastPathHandler(vault, personaMgr)
	speechPlayer := interaction.NewSpeechPlayer(synthesizer)
	interactionMgr := interaction.NewManager(coord, fastPath, speechPlayer, taskRunner)

	return &Pipeline{
		coordinator: coord,
		interaction: interactionMgr,
		fastPath:    fastPath,
		taskRunner:  taskRunner,
		toolsReg:    toolsReg,
	}
}

// SetCallbacks registers lifecycle and event callbacks for UI synchronization.
func (p *Pipeline) SetCallbacks(
	onState func(State),
	onSpeech func(string),
	onPersona func(persona.PersonaSlot),
	onBond func(persona.BondData),
) {
	if onState != nil {
		p.coordinator.SetStateChangeCallback(onState)
	}
	if onSpeech != nil {
		p.interaction.SetSpeechChunkCallback(onSpeech)
		p.taskRunner.SetSpeechChunkCallback(onSpeech)
	}
	if onPersona != nil {
		p.interaction.SetPersonaSyncCallback(onPersona)
	}
	if onBond != nil {
		p.taskRunner.SetBondSyncCallback(onBond)
	}
}

// State returns current coordinator state.
func (p *Pipeline) State() State {
	return p.coordinator.State()
}

// Interrupt cancels active background task and stops audio playback.
func (p *Pipeline) Interrupt() {
	_ = p.interaction.Interrupt(context.Background(), p.coordinator.SessionID(), "")
}

// Chat processes user input by delegating to interaction manager.
func (p *Pipeline) Chat(parentCtx context.Context, userInput string) error {
	input := orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      userInput,
		Stability: orchestrator.StabilityFinal,
	}
	_, err := p.interaction.Submit(parentCtx, input)
	return err
}

// Coordinator returns the underlying session coordinator.
func (p *Pipeline) Coordinator() *orchestrator.Coordinator {
	return p.coordinator
}

// Interaction returns the interaction manager.
func (p *Pipeline) Interaction() *interaction.Manager {
	return p.interaction
}

// TaskRunner returns the background cognitive task runner.
func (p *Pipeline) TaskRunner() *task.Runner {
	return p.taskRunner
}

// Tools returns the tool registry.
func (p *Pipeline) Tools() *tools.Registry {
	return p.toolsReg
}

// FastPath returns the fast path handler.
func (p *Pipeline) FastPath() *interaction.FastPathHandler {
	return p.fastPath
}

// SetCloudClient configures cloud realtime client for edge-cloud synchronization.
func (p *Pipeline) SetCloudClient(client *cloud.Client) {
	p.interaction.SetCloudClient(client)
}
