package skill

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// todoSkill maintains a structured task list across rounds within
// one kinclaw session. Mirrors kincode's todo_write tool exactly —
// same param shape, same status enum, same overwrite-in-full
// semantics — so the Mac shell's TodoChecklistView component
// renders both surfaces identically.
//
// State is in-process per session. The desktop shell observes the
// tool_call event on the SSE stream — `params.todos` is the JSON-
// encoded list — and renders a live checklist. Critical for Cowork
// mode where Pilot's plan ("open Numbers, click cell A1, type 42,
// take a screenshot") is invisible to the user without this surface;
// they have to trust the agent or ⌘. abort blindly.
//
// Status enum: pending / in_progress / completed. Convention is
// single-tasking — exactly ONE in_progress at a time. The kernel
// soft-warns past 1 but doesn't reject (soul-prompt-level rule).
type todoSkill struct {
	mu    sync.Mutex
	items []TodoItem
}

// TodoItem matches Claude Code + kincode's shape so prompts that
// reference TodoWrite produce the right JSON for kinclaw too.
//
//	content    — imperative form: "Open Numbers"
//	activeForm — present-continuous: "Opening Numbers"
//	status     — pending | in_progress | completed
type TodoItem struct {
	Content    string `json:"content"`
	ActiveForm string `json:"activeForm"`
	Status     string `json:"status"`
}

// NewTodoSkill registers a fresh todo list. Per-session — kinclaw
// builds a new registry on each soul load, so todos don't leak
// across soul switches.
func NewTodoSkill() Skill {
	return &todoSkill{}
}

func (s *todoSkill) Name() string { return "todo_write" }

func (s *todoSkill) Description() string {
	return "Maintain a structured task list across rounds. Pass the COMPLETE current list each call (the existing list is replaced — not a delta). Use for any task with 3+ steps:\n" +
		"  1. Plan: emit the full todo list with all items pending\n" +
		"  2. Execute: before each step, mark exactly ONE item as in_progress\n" +
		"  3. Tick: mark completed when done, then move to the next\n\n" +
		"Single-tasking discipline — only ONE item is in_progress at a time. The desktop shell renders this list as an inline checklist so the user sees your plan + progress without you re-narrating.\n\n" +
		"Skip for trivial 1-2 step requests (overhead > value). Use for: multi-app workflows, anything with a verify-step (\"open X, find Y, do Z, take screenshot to confirm\"), file edits across 3+ files."
}

// ToolDef builds the OpenAI function-calling schema by hand —
// MakeToolDef only handles flat string→string properties, but
// todo_write needs a nested array-of-object schema (todos →
// [{content, activeForm, status}]).
func (s *todoSkill) ToolDef() json.RawMessage {
	schema := map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "todo_write",
			"description": s.Description(),
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"todos": map[string]interface{}{
						"type":        "array",
						"description": "The complete current todo list (overwrites prior state).",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"content": map[string]interface{}{
									"type":        "string",
									"description": "Imperative description, e.g. \"Open Numbers\".",
								},
								"activeForm": map[string]interface{}{
									"type":        "string",
									"description": "Present-continuous form, e.g. \"Opening Numbers\".",
								},
								"status": map[string]interface{}{
									"type":        "string",
									"enum":        []string{"pending", "in_progress", "completed"},
									"description": "pending / in_progress / completed.",
								},
							},
							"required": []string{"content", "activeForm", "status"},
						},
					},
				},
				"required": []string{"todos"},
			},
		},
	}
	blob, _ := json.Marshal(schema)
	return blob
}

// Execute. params["todos"] is a JSON-encoded array — kinclaw's
// brain.ParseArguments JSON-encodes complex types (arrays, objects)
// so we decode it back here. Validates each item, replaces the
// list atomically, returns a human-readable summary the model
// can read in subsequent rounds for context.
func (s *todoSkill) Execute(params map[string]string) (string, error) {
	raw, ok := params["todos"]
	if !ok || raw == "" {
		return "", fmt.Errorf("todos is required (JSON array of {content, activeForm, status})")
	}
	var parsed []TodoItem
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", fmt.Errorf("todos must be a JSON array of {content, activeForm, status}: %w", err)
	}

	// Validate the whole batch before committing — partial accept
	// would leave the list in an inconsistent state.
	inProgress := 0
	for i, it := range parsed {
		if strings.TrimSpace(it.Content) == "" {
			return "", fmt.Errorf("todo %d: content is required", i+1)
		}
		if strings.TrimSpace(it.ActiveForm) == "" {
			return "", fmt.Errorf("todo %d: activeForm is required", i+1)
		}
		switch it.Status {
		case "pending", "in_progress", "completed":
		default:
			return "", fmt.Errorf("todo %d: status must be pending|in_progress|completed (got %q)",
				i+1, it.Status)
		}
		if it.Status == "in_progress" {
			inProgress++
		}
	}

	s.mu.Lock()
	s.items = parsed
	s.mu.Unlock()

	// Compact view for the model's next-round reasoning. Format:
	//   todo list updated (5 items, 1 in_progress, 2 completed):
	//     1. [x] Open Numbers
	//     2. [x] Click cell A1
	//     3. [~] Typing 42         ← active form when in_progress
	//     4. [ ] Take screenshot
	//     5. [ ] Confirm with user
	var sb strings.Builder
	completedCount := 0
	for _, it := range parsed {
		if it.Status == "completed" {
			completedCount++
		}
	}
	fmt.Fprintf(&sb, "todo list updated (%d items, %d in_progress, %d completed):\n",
		len(parsed), inProgress, completedCount)
	for i, it := range parsed {
		mark := "[ ]"
		label := it.Content
		switch it.Status {
		case "in_progress":
			mark = "[~]"
			label = it.ActiveForm
		case "completed":
			mark = "[x]"
		}
		fmt.Fprintf(&sb, "  %d. %s %s\n", i+1, mark, label)
	}
	if inProgress > 1 {
		sb.WriteString("\nWarning: more than one item is in_progress. Convention is single-tasking — keep one active at a time.\n")
	}
	return sb.String(), nil
}

// Items returns a snapshot of the current todo list. Useful for an
// /api/todos endpoint or tests inspecting state without going
// through Execute.
func (s *todoSkill) Items() []TodoItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TodoItem, len(s.items))
	copy(out, s.items)
	return out
}
