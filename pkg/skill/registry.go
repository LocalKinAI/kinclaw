package skill

import (
	"encoding/json"
	"fmt"
	"strings"
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
	ID, Name string
	Params   map[string]string
}
type ToolResult struct {
	ToolCallID, Name, Output string
	// Images are file paths to attach to the tool message when the
	// brain supports vision. Skills opt in by emitting `image://<path>`
	// markers in their text output; ExecuteToolCalls strips those
	// markers and populates this field. The brain adapter reads the
	// files at send time and inlines them as image_url / image source
	// blocks. Brains without vision support ignore the field, so
	// adding markers in a skill is always safe.
	Images []string
	Err    error
}

// extractImageMarkers strips lines matching `image://<path>` from the
// tool output and returns the cleaned text + the list of paths
// encountered (de-duped, in order of first appearance). Skills can
// emit any number of markers anywhere in their output; the markers
// don't reach the LLM as text — they get rerouted into the message's
// Images list, which becomes inline image content in the next
// vision-capable API call.
func extractImageMarkers(out string) (cleaned string, images []string) {
	if !strings.Contains(out, "image://") {
		return out, nil
	}
	seen := make(map[string]bool)
	var keep []string
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "image://") {
			path := strings.TrimSpace(strings.TrimPrefix(t, "image://"))
			if path != "" && !seen[path] {
				seen[path] = true
				images = append(images, path)
			}
			continue
		}
		keep = append(keep, line)
	}
	return strings.TrimRight(strings.Join(keep, "\n"), "\n"), images
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
		cleaned, imgs := extractImageMarkers(out)
		return []ToolResult{{ToolCallID: c.ID, Name: c.Name, Output: cleaned, Images: imgs}}
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
			cleaned, imgs := extractImageMarkers(out)
			results[idx] = ToolResult{ToolCallID: ci.ID, Name: ci.Name, Output: cleaned, Images: imgs}
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
