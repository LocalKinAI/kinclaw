//go:build measure

package main

// What one pilot call carries before the conversation starts, piece by piece:
//
//	go test -tags measure -run TestPilotPromptSize -v ./cmd/kinclaw/
//
// MCP servers are not started (a server run by npx can mean a download):
// HOME is a copy of the real one without ~/.localkin/mcp.json. Their tools
// are measured from MEASURE_MCP_TOOLS instead — a tools/list result saved
// as JSON — both as they are sent today (every schema) and deferred (one
// index line each, as skill.DeferredIndex writes them).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LocalKinAI/kinclaw/pkg/brain"
)

func TestPilotPromptSize(t *testing.T) {
	real, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	for _, dir := range []string{".kinclaw", ".kinbrain"} {
		_ = os.Symlink(filepath.Join(real, dir), filepath.Join(home, dir))
	}
	localkin := filepath.Join(home, ".localkin")
	if err := os.MkdirAll(localkin, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(real, ".localkin"))
	for _, e := range entries {
		if e.Name() == "mcp.json" {
			continue
		}
		_ = os.Symlink(filepath.Join(real, ".localkin", e.Name()), filepath.Join(localkin, e.Name()))
	}
	// MEASURE_MCP_JSON: an mcp.json of servers safe to start here (say the panel alone), to see
	// what the kernel really sends with them.
	if p := os.Getenv("MEASURE_MCP_JSON"); p != "" {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(localkin, "mcp.json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)

	soulPath, _ := filepath.Abs("../../souls/pilot.soul.md")
	if p := os.Getenv("MEASURE_SOUL"); p != "" {
		soulPath = p
	}
	sess, err := newSession(soulPath, false, true)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.closeMCP()

	system := buildMessages(sess)[0].Content
	var tools strings.Builder
	for _, def := range sess.toolDefs {
		tools.Write(def)
	}
	soulTokens := brain.EstimateText(sess.soul.SystemPrompt)
	systemTokens := brain.EstimateText(system)
	toolTokens := brain.EstimateText(tools.String())
	fmt.Printf("soul + injected memories: %6d chars ~%6d tokens\n", len([]rune(sess.soul.SystemPrompt)), soulTokens)
	fmt.Printf("system message in all:    %6d chars ~%6d tokens (workspace, deferred index)\n", len([]rune(system)), systemTokens)
	fmt.Printf("native tools sent:        %6d tools ~%6d tokens\n", len(sess.toolDefs), toolTokens)
	pending := sess.pendingDeferred()
	mcpDeferred := 0
	for _, n := range pending {
		if strings.HasPrefix(n, "mcp_") {
			mcpDeferred++
		}
	}
	fmt.Printf("deferred skills:          %d (%d of them MCP tools)\n", len(pending), mcpDeferred)
	fmt.Printf("per call before history:  ~%d tokens\n", systemTokens+toolTokens)

	if path := os.Getenv("MEASURE_MCP_TOOLS"); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var listed struct {
			Result struct {
				Tools []struct {
					Name        string          `json:"name"`
					Description string          `json:"description"`
					Schema      json.RawMessage `json:"inputSchema"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &listed); err != nil {
			t.Fatal(err)
		}
		server := os.Getenv("MEASURE_MCP_SERVER")
		if server == "" {
			server = "panel"
		}
		var all, index strings.Builder
		for _, tool := range listed.Result.Tools {
			name := "mcp_" + server + "_" + tool.Name
			def, _ := json.Marshal(map[string]any{"type": "function", "function": map[string]any{
				"name": name, "description": tool.Description, "parameters": tool.Schema}})
			all.Write(def)
			fmt.Fprintf(&index, "- %s — %s\n", name, firstSentenceForMeasure(tool.Description, 110))
		}
		fmt.Printf("MCP %s tools, all sent:  %6d tools ~%6d tokens\n", server, len(listed.Result.Tools), brain.EstimateText(all.String()))
		fmt.Printf("MCP %s tools, deferred:  index ~%6d tokens\n", server, brain.EstimateText(index.String()))
	}
}

// firstSentenceForMeasure is skill.firstSentence, which is not exported.
func firstSentenceForMeasure(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	for _, sep := range []string{". ", "。", "! ", "? "} {
		if i := strings.Index(s, sep); i > 20 {
			s = s[:i+1]
			break
		}
	}
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
