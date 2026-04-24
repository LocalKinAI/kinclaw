package memory

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LocalKinAI/kinclaw/pkg/brain"
)

func openTestDB(t *testing.T) *SQLiteStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	store, err := OpenMemory(path)
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestOpenMemory_CreatesDB(t *testing.T) {
	store := openTestDB(t)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestOpenMemory_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "test.db")
	store, err := OpenMemory(path)
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	store.Close()
}

func TestSaveMessage_And_LoadHistory(t *testing.T) {
	store := openTestDB(t)
	session := "test-session-1"

	msg1 := brain.Message{Role: brain.RoleUser, Content: "Hello"}
	msg2 := brain.Message{Role: brain.RoleAssistant, Content: "Hi there"}

	if err := store.SaveMessage(session, msg1); err != nil {
		t.Fatalf("SaveMessage failed: %v", err)
	}
	if err := store.SaveMessage(session, msg2); err != nil {
		t.Fatalf("SaveMessage failed: %v", err)
	}

	history := store.LoadHistory(session, 50)
	if len(history) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(history))
	}
	if history[0].Role != brain.RoleUser || history[0].Content != "Hello" {
		t.Errorf("first message: got role=%q content=%q", history[0].Role, history[0].Content)
	}
	if history[1].Role != brain.RoleAssistant || history[1].Content != "Hi there" {
		t.Errorf("second message: got role=%q content=%q", history[1].Role, history[1].Content)
	}
}

func TestLoadHistory_DefaultLimit(t *testing.T) {
	store := openTestDB(t)
	session := "limit-test"

	// Save more than default limit (0 => 50)
	for i := 0; i < 5; i++ {
		store.SaveMessage(session, brain.Message{Role: brain.RoleUser, Content: "msg"})
	}

	// Passing 0 should use default limit of 50
	history := store.LoadHistory(session, 0)
	if len(history) != 5 {
		t.Errorf("expected 5 messages, got %d", len(history))
	}
}

func TestLoadHistory_RespectLimit(t *testing.T) {
	store := openTestDB(t)
	session := "limit-test-2"

	for i := 0; i < 10; i++ {
		store.SaveMessage(session, brain.Message{Role: brain.RoleUser, Content: "msg"})
	}

	history := store.LoadHistory(session, 3)
	if len(history) != 3 {
		t.Errorf("expected 3 messages, got %d", len(history))
	}
}

func TestLoadHistory_OrderChronological(t *testing.T) {
	store := openTestDB(t)
	session := "order-test"

	store.SaveMessage(session, brain.Message{Role: brain.RoleUser, Content: "first"})
	store.SaveMessage(session, brain.Message{Role: brain.RoleUser, Content: "second"})
	store.SaveMessage(session, brain.Message{Role: brain.RoleUser, Content: "third"})

	history := store.LoadHistory(session, 10)
	if len(history) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(history))
	}
	if history[0].Content != "first" {
		t.Errorf("expected first message 'first', got %q", history[0].Content)
	}
	if history[2].Content != "third" {
		t.Errorf("expected last message 'third', got %q", history[2].Content)
	}
}

func TestLoadHistory_SessionIsolation(t *testing.T) {
	store := openTestDB(t)

	store.SaveMessage("session-a", brain.Message{Role: brain.RoleUser, Content: "msg-a"})
	store.SaveMessage("session-b", brain.Message{Role: brain.RoleUser, Content: "msg-b"})

	histA := store.LoadHistory("session-a", 50)
	histB := store.LoadHistory("session-b", 50)
	if len(histA) != 1 || histA[0].Content != "msg-a" {
		t.Errorf("session-a: unexpected history %v", histA)
	}
	if len(histB) != 1 || histB[0].Content != "msg-b" {
		t.Errorf("session-b: unexpected history %v", histB)
	}
}

func TestLoadHistory_EmptySession(t *testing.T) {
	store := openTestDB(t)
	history := store.LoadHistory("nonexistent", 50)
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d messages", len(history))
	}
}

func TestSaveMessage_WithToolCalls(t *testing.T) {
	store := openTestDB(t)
	session := "tool-test"

	msg := brain.Message{
		Role:    brain.RoleAssistant,
		Content: "Let me check that.",
		ToolCalls: []brain.ToolCall{
			{
				ID:   "tc-1",
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: "shell", Arguments: `{"command":"echo hi"}`},
			},
		},
	}
	if err := store.SaveMessage(session, msg); err != nil {
		t.Fatalf("SaveMessage with tool calls failed: %v", err)
	}

	history := store.LoadHistory(session, 10)
	if len(history) != 1 {
		t.Fatalf("expected 1 message, got %d", len(history))
	}
	if len(history[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(history[0].ToolCalls))
	}
	if history[0].ToolCalls[0].Function.Name != "shell" {
		t.Errorf("expected tool name 'shell', got %q", history[0].ToolCalls[0].Function.Name)
	}
}

func TestSaveMessage_WithToolCallID(t *testing.T) {
	store := openTestDB(t)
	session := "tool-result-test"

	msg := brain.Message{
		Role:       brain.RoleTool,
		Content:    "hello",
		ToolCallID: "tc-42",
	}
	if err := store.SaveMessage(session, msg); err != nil {
		t.Fatalf("SaveMessage with tool_call_id failed: %v", err)
	}

	history := store.LoadHistory(session, 10)
	if len(history) != 1 {
		t.Fatalf("expected 1 message, got %d", len(history))
	}
	if history[0].ToolCallID != "tc-42" {
		t.Errorf("expected tool_call_id 'tc-42', got %q", history[0].ToolCallID)
	}
}

func TestSave_And_Recall(t *testing.T) {
	store := openTestDB(t)

	result, err := store.Save("user_name", "Alice")
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if !strings.Contains(result, "Saved memory") {
		t.Errorf("unexpected save result: %q", result)
	}

	recalled, err := store.Recall("user_name")
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}
	if !strings.Contains(recalled, "user_name") {
		t.Errorf("expected recall to contain key, got %q", recalled)
	}
	if !strings.Contains(recalled, "Alice") {
		t.Errorf("expected recall to contain value 'Alice', got %q", recalled)
	}
}

func TestSave_Upsert(t *testing.T) {
	store := openTestDB(t)

	store.Save("color", "blue")
	store.Save("color", "red")

	recalled, err := store.Recall("color")
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}
	if !strings.Contains(recalled, "red") {
		t.Errorf("expected upserted value 'red', got %q", recalled)
	}
	// Should only have one entry for "color"
	count := strings.Count(recalled, "[color]")
	if count != 1 {
		t.Errorf("expected exactly 1 entry for 'color', found %d", count)
	}
}

func TestRecall_NoMatch(t *testing.T) {
	store := openTestDB(t)

	result, err := store.Recall("nonexistent_query")
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}
	if !strings.Contains(result, "No memories found") {
		t.Errorf("expected no memories message, got %q", result)
	}
}

func TestRecall_SearchByValue(t *testing.T) {
	store := openTestDB(t)

	store.Save("pet", "golden retriever")
	store.Save("food", "spaghetti")

	recalled, err := store.Recall("retriever")
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}
	if !strings.Contains(recalled, "golden retriever") {
		t.Errorf("expected to find 'golden retriever' by value search, got %q", recalled)
	}
}

func TestRecall_MultipleResults(t *testing.T) {
	store := openTestDB(t)

	store.Save("project_name", "LocalKin")
	store.Save("project_version", "1.0")

	recalled, err := store.Recall("project")
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}
	if !strings.Contains(recalled, "project_name") || !strings.Contains(recalled, "project_version") {
		t.Errorf("expected both project entries, got %q", recalled)
	}
}

func TestDefaultDBPath(t *testing.T) {
	path := DefaultDBPath()
	if !strings.Contains(path, ".localkin") {
		t.Errorf("expected path to contain '.localkin', got %q", path)
	}
	if !strings.HasSuffix(path, "memory.db") {
		t.Errorf("expected path to end with 'memory.db', got %q", path)
	}
}
