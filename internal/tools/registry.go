package tools

import (
	"context"
	"fmt"
	"sync"

	"avatar/internal/memory"
	"avatar/internal/orchestrator"
	"avatar/internal/persona"
)

type ToolHandler func(ctx context.Context, params map[string]any) (any, error)

type ToolDefinition struct {
	ID                   string
	Version              string
	Description          string
	PermissionLevel      string // low, medium, high
	RequiresConfirmation bool
	Handler              ToolHandler
}

// Registry manages tool definitions, checks permissions/idempotency, and executes tools.
type Registry struct {
	mu           sync.RWMutex
	tools        map[string]ToolDefinition
	idempotency  map[string]orchestrator.ToolResult
}

func NewRegistry() *Registry {
	return &Registry{
		tools:       make(map[string]ToolDefinition),
		idempotency: make(map[string]orchestrator.ToolResult),
	}
}

// Register adds a tool definition to the registry.
func (r *Registry) Register(def ToolDefinition) error {
	if def.ID == "" {
		return fmt.Errorf("tool ID cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[def.ID] = def
	return nil
}

// Get returns the tool definition for the given ID.
func (r *Registry) Get(id string) (ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.tools[id]
	return def, ok
}

// List returns all registered tool definitions.
func (r *Registry) List() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]ToolDefinition, 0, len(r.tools))
	for _, def := range r.tools {
		list = append(list, def)
	}
	return list
}

// Execute implements orchestrator.ToolExecutor.
func (r *Registry) Execute(ctx context.Context, call orchestrator.ToolCall, idempotencyKey string) (orchestrator.ToolResult, error) {
	if idempotencyKey == "" {
		idempotencyKey = call.IdempotencyKey
	}

	if idempotencyKey != "" {
		r.mu.RLock()
		if cached, exists := r.idempotency[idempotencyKey]; exists {
			r.mu.RUnlock()
			return cached, nil
		}
		r.mu.RUnlock()
	}

	r.mu.RLock()
	def, exists := r.tools[call.ToolID]
	r.mu.RUnlock()

	if !exists {
		res := orchestrator.ToolResult{
			ToolID:         call.ToolID,
			Success:        false,
			Error:          fmt.Sprintf("tool '%s' not found", call.ToolID),
			IdempotencyKey: idempotencyKey,
		}
		return res, fmt.Errorf("tool '%s' not found", call.ToolID)
	}

	data, err := def.Handler(ctx, call.Params)
	res := orchestrator.ToolResult{
		ToolID:         call.ToolID,
		IdempotencyKey: idempotencyKey,
	}

	if err != nil {
		res.Success = false
		res.Error = err.Error()
	} else {
		res.Success = true
		res.Data = data
	}

	if idempotencyKey != "" {
		r.mu.Lock()
		r.idempotency[idempotencyKey] = res
		r.mu.Unlock()
	}

	return res, err
}

// RegisterMemoryTools registers tools that operate on memory vault.
func RegisterMemoryTools(reg *Registry, vault *memory.Vault) {
	if reg == nil || vault == nil {
		return
	}

	_ = reg.Register(ToolDefinition{
		ID:          "memory_store",
		Version:     "1.0",
		Description: "Store user profile fact or preference",
		PermissionLevel: "low",
		Handler: func(ctx context.Context, params map[string]any) (any, error) {
			category, _ := params["category"].(string)
			if category == "" {
				category = "preference"
			}
			key, _ := params["key"].(string)
			value, _ := params["value"].(string)
			if key == "" || value == "" {
				return nil, fmt.Errorf("key and value are required")
			}
			item := vault.AddOrUpdateProfile(category, key, value)
			return item, nil
		},
	})

	_ = reg.Register(ToolDefinition{
		ID:          "memory_forget",
		Version:     "1.0",
		Description: "Delete user profile or episode by ID",
		PermissionLevel: "low",
		Handler: func(ctx context.Context, params map[string]any) (any, error) {
			id, _ := params["id"].(string)
			if id == "" {
				return nil, fmt.Errorf("id is required")
			}
			ok := vault.DeleteProfile(id)
			if !ok {
				ok = vault.DeleteEpisode(id)
			}
			return map[string]bool{"deleted": ok}, nil
		},
	})
}

// RegisterPersonaTools registers tools that operate on persona manager.
func RegisterPersonaTools(reg *Registry, mgr *persona.Manager) {
	if reg == nil || mgr == nil {
		return
	}

	_ = reg.Register(ToolDefinition{
		ID:          "persona_switch",
		Version:     "1.0",
		Description: "Switch the active persona slot",
		PermissionLevel: "low",
		Handler: func(ctx context.Context, params map[string]any) (any, error) {
			personaID, _ := params["persona_id"].(string)
			if personaID == "" {
				return nil, fmt.Errorf("persona_id is required")
			}
			slot, err := mgr.SetActiveSlot(personaID)
			if err != nil {
				return nil, err
			}
			return slot, nil
		},
	})

	_ = reg.Register(ToolDefinition{
		ID:          "persona_get",
		Version:     "1.0",
		Description: "Get the current active persona slot",
		PermissionLevel: "low",
		Handler: func(ctx context.Context, params map[string]any) (any, error) {
			slot := mgr.GetActiveSlot()
			return slot, nil
		},
	})
}
