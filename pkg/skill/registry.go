package skill

import (
	"encoding/json"
	"fmt"
	"sync"
)

type Skill interface {
	Name() string
	Description() string
	ToolDef() json.RawMessage // OpenAI function-calling format
	Execute(params map[string]string) (string, error)
}

type Registry struct {
	mu     sync.RWMutex
	skills map[string]Skill
}

func NewRegistry() *Registry {
	return &Registry{skills: make(map[string]Skill)}
}

func (r *Registry) Register(s Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[s.Name()] = s
}

func (r *Registry) Get(name string) (Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", name)
	}
	return s, nil
}

func (r *Registry) ToolDefs() []json.RawMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]json.RawMessage, 0, len(r.skills))
	for _, s := range r.skills {
		defs = append(defs, s.ToolDef())
	}
	return defs
}

func (r *Registry) FilteredToolDefs(allow []string) []json.RawMessage {
	if len(allow) == 0 {
		return r.ToolDefs()
	}
	enabled := make(map[string]bool, len(allow))
	for _, name := range allow {
		enabled[name] = true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var defs []json.RawMessage
	for _, s := range r.skills {
		if enabled[s.Name()] {
			defs = append(defs, s.ToolDef())
		}
	}
	return defs
}

type ToolCallInfo struct {
	ID     string
	Name   string
	Params map[string]string
}

type ToolResult struct {
	ToolCallID string
	Name       string
	Output     string
	Err        error
}

func ExecuteToolCalls(reg *Registry, calls []ToolCallInfo) []ToolResult {
	if len(calls) == 0 {
		return nil
	}
	if len(calls) == 1 {
		c := calls[0]
		s, err := reg.Get(c.Name)
		if err != nil {
			return []ToolResult{{ToolCallID: c.ID, Name: c.Name, Output: err.Error(), Err: err}}
		}
		out, err := s.Execute(c.Params)
		if err != nil {
			return []ToolResult{{ToolCallID: c.ID, Name: c.Name, Output: fmt.Sprintf("Error: %v", err), Err: err}}
		}
		return []ToolResult{{ToolCallID: c.ID, Name: c.Name, Output: out}}
	}
	results := make([]ToolResult, len(calls))
	var wg sync.WaitGroup
	for i, c := range calls {
		wg.Add(1)
		go func(idx int, ci ToolCallInfo) {
			defer wg.Done()
			s, err := reg.Get(ci.Name)
			if err != nil {
				results[idx] = ToolResult{ToolCallID: ci.ID, Name: ci.Name, Output: err.Error(), Err: err}
				return
			}
			out, err := s.Execute(ci.Params)
			if err != nil {
				results[idx] = ToolResult{ToolCallID: ci.ID, Name: ci.Name, Output: fmt.Sprintf("Error: %v", err), Err: err}
				return
			}
			results[idx] = ToolResult{ToolCallID: ci.ID, Name: ci.Name, Output: out}
		}(i, c)
	}
	wg.Wait()
	return results
}

func MakeToolDef(name, description string, properties map[string]map[string]string, required []string) json.RawMessage {
	props := make(map[string]interface{})
	for k, v := range properties {
		props[k] = v
	}
	schema := map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        name,
			"description": description,
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": props,
				"required":   required,
			},
		},
	}
	b, _ := json.Marshal(schema)
	return b
}
