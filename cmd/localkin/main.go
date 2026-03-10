package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/LocalKinAI/localkin/pkg/auth"
	"github.com/LocalKinAI/localkin/pkg/brain"
	"github.com/LocalKinAI/localkin/pkg/memory"
	"github.com/LocalKinAI/localkin/pkg/skill"
	"github.com/LocalKinAI/localkin/pkg/soul"
)

const (
	version       = "0.2.0"
	maxToolRounds = 20
)

func main() {
	soulPath := flag.String("soul", "", "Path to .soul.md file")
	execMsg := flag.String("exec", "", "Execute a single message and exit")
	debug := flag.Bool("debug", false, "Show debug output")
	showVersion := flag.Bool("version", false, "Show version")
	login := flag.Bool("login", false, "Login with Claude OAuth (free tier)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("localkin %s\n", version)
		return
	}
	if *login {
		if err := auth.Login(); err != nil {
			fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	path := findSoulFile(*soulPath)
	if path == "" {
		fmt.Fprintln(os.Stderr, "Error: no soul file found. Use -soul flag or place a .soul.md in ./souls/")
		os.Exit(1)
	}

	s, err := soul.LoadSoul(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	apiKey := s.Meta.Brain.APIKey
	if apiKey == "" {
		switch s.Meta.Brain.Provider {
		case "claude":
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
			if apiKey == "" {
				apiKey = loadOAuthToken()
			}
		case "openai":
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
	}
	if apiKey == "" && s.Meta.Brain.Provider != "ollama" {
		fmt.Fprintf(os.Stderr, "Error: API key not set. Set brain.api_key in soul file or $ANTHROPIC_API_KEY / $OPENAI_API_KEY\n")
		os.Exit(1)
	}

	b := brain.NewBrain(s.Meta.Brain.Provider, s.Meta.Brain.Endpoint,
		s.Meta.Brain.Model, apiKey, s.Meta.Brain.Temperature)

	store, err := memory.OpenMemory(memory.DefaultDBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: memory unavailable: %v\n", err)
	}
	if store != nil {
		defer store.Close()
	}

	registry := skill.NewRegistry()
	skillsDir := "./skills"
	if s.Meta.Skills.Dir != "" {
		skillsDir = s.Meta.Skills.Dir
	}

	if s.Meta.Permissions.Shell {
		registry.Register(skill.NewShellSkill(s.Meta.Permissions.ShellTimeout))
		registry.Register(skill.NewForgeSkill(skillsDir, registry))
	}
	registry.Register(skill.NewFileReadSkill())
	registry.Register(skill.NewFileWriteSkill())
	registry.Register(skill.NewFileEditSkill())
	if s.Meta.Permissions.Network {
		registry.Register(skill.NewWebFetchSkill())
	}
	if store != nil {
		registry.Register(skill.NewMemorySkill(store))
	}

	for _, dir := range []string{skillsDir, homeSkillsDir()} {
		exts, _ := skill.LoadExternalSkills(dir)
		for _, ext := range exts {
			registry.Register(ext)
		}
	}

	toolDefs := registry.FilteredToolDefs(s.Meta.Skills.Enable)
	fmt.Fprintf(os.Stderr, "\033[2m  LocalKin %s\n  Soul:     %s (%s)\n  Brain:    %s / %s\n  Skills:   %d loaded\033[0m\n\n",
		version, s.Meta.Name, s.FilePath, s.Meta.Brain.Provider, s.Meta.Brain.Model, len(toolDefs))
	sessionID := fmt.Sprintf("%s-%d", s.Meta.Name, os.Getpid())
	var history []brain.Message
	if store != nil {
		history = store.LoadHistory(sessionID, 50)
	}
	if *execMsg != "" {
		os.Exit(runOnce(b, s, registry, toolDefs, store, sessionID, history, *execMsg, *debug))
	}
	runREPL(b, s, registry, toolDefs, store, sessionID, history, *debug)
}

func runREPL(b brain.Brain, s *soul.Soul, registry *skill.Registry, toolDefs []json.RawMessage, store *memory.SQLiteStore, sessionID string, history []brain.Message, debug bool) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	prompt := fmt.Sprintf("\033[1;36m%s>\033[0m ", s.Meta.Name)
	for {
		input, err := readLine(prompt)
		if err != nil {
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		switch {
		case input == "/quit" || input == "/exit":
			fmt.Fprintln(os.Stderr, "Goodbye.")
			return
		case input == "/help":
			fmt.Println("\033[2m/quit    Exit\n/skills  List available skills\n/clear   Clear conversation history\033[0m")
			continue
		case input == "/skills":
			for _, def := range toolDefs {
				var tool struct {
					Function struct {
						Name string `json:"name"`
					} `json:"function"`
				}
				json.Unmarshal(def, &tool)
				fmt.Printf("  %s\n", tool.Function.Name)
			}
			continue
		case input == "/clear":
			history = nil
			fmt.Println("\033[2mConversation cleared.\033[0m")
			continue
		}
		userMsg := brain.Message{Role: brain.RoleUser, Content: input}
		history = append(history, userMsg)
		if store != nil {
			store.SaveMessage(sessionID, userMsg)
		}
		messages := buildMessages(s, history)
		onChunk := func(chunk string, thinking bool) error {
			if thinking {
				fmt.Fprint(os.Stderr, "\033[2m"+chunk+"\033[0m")
			} else {
				fmt.Print(chunk)
			}
			return nil
		}
		reply, toolHistory, err := chatLoop(ctx, b, messages, toolDefs, registry, onChunk, debug)
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
			continue
		}
		for _, msg := range toolHistory {
			if store != nil {
				store.SaveMessage(sessionID, msg)
			}
			history = append(history, msg)
		}
		assistantMsg := brain.Message{Role: brain.RoleAssistant, Content: reply}
		history = append(history, assistantMsg)
		if store != nil {
			store.SaveMessage(sessionID, assistantMsg)
		}
		for _, msg := range toolHistory {
			if msg.Role == brain.RoleAssistant {
				for _, tc := range msg.ToolCalls {
					if tc.Function.Name == "forge" {
						toolDefs = registry.ToolDefs()
						break
					}
				}
			}
		}
	}
}

func runOnce(b brain.Brain, s *soul.Soul, registry *skill.Registry, toolDefs []json.RawMessage, store *memory.SQLiteStore, sessionID string, history []brain.Message, input string, debug bool) int {
	ctx := context.Background()
	userMsg := brain.Message{Role: brain.RoleUser, Content: input}
	history = append(history, userMsg)
	if store != nil {
		store.SaveMessage(sessionID, userMsg)
	}
	messages := buildMessages(s, history)
	onChunk := func(chunk string, thinking bool) error {
		if thinking {
			fmt.Fprint(os.Stderr, "\033[2m"+chunk+"\033[0m")
		} else {
			fmt.Print(chunk)
		}
		return nil
	}
	reply, toolHistory, err := chatLoop(ctx, b, messages, toolDefs, registry, onChunk, debug)
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	for _, msg := range toolHistory {
		if store != nil {
			store.SaveMessage(sessionID, msg)
		}
	}
	if store != nil {
		store.SaveMessage(sessionID, brain.Message{Role: brain.RoleAssistant, Content: reply})
	}
	return 0
}

func chatLoop(ctx context.Context, b brain.Brain, messages []brain.Message, toolDefs []json.RawMessage, registry *skill.Registry, onChunk brain.StreamFunc, debug bool) (string, []brain.Message, error) {
	var intermediateHistory []brain.Message
	for round := 0; round < maxToolRounds; round++ {
		result, err := b.Chat(ctx, messages, toolDefs, onChunk)
		if err != nil {
			return "", nil, err
		}
		if len(result.ToolCalls) == 0 {
			return result.Content, intermediateHistory, nil
		}
		assistantMsg := brain.Message{
			Role: brain.RoleAssistant, Content: result.Content, ToolCalls: result.ToolCalls,
		}
		messages = append(messages, assistantMsg)
		intermediateHistory = append(intermediateHistory, assistantMsg)
		var callInfos []skill.ToolCallInfo
		for _, tc := range result.ToolCalls {
			params, err := tc.ParseArguments()
			if err != nil {
				toolMsg := brain.Message{Role: brain.RoleTool, Content: "Error: " + err.Error(), ToolCallID: tc.ID}
				messages = append(messages, toolMsg)
				intermediateHistory = append(intermediateHistory, toolMsg)
				continue
			}
			if debug {
				fmt.Fprintf(os.Stderr, "\033[2m[tool: %s %v]\033[0m\n", tc.Function.Name, params)
			}
			callInfos = append(callInfos, skill.ToolCallInfo{ID: tc.ID, Name: tc.Function.Name, Params: params})
		}
		results := skill.ExecuteToolCalls(registry, callInfos)
		for _, r := range results {
			if debug {
				display := r.Output
				if len(display) > 200 {
					display = display[:200] + "..."
				}
				fmt.Fprintf(os.Stderr, "\033[2m[%s -> %s]\033[0m\n", r.Name, strings.ReplaceAll(display, "\n", " "))
			}
			toolMsg := brain.Message{Role: brain.RoleTool, Content: r.Output, ToolCallID: r.ToolCallID}
			messages = append(messages, toolMsg)
			intermediateHistory = append(intermediateHistory, toolMsg)
		}
	}
	return "", intermediateHistory, fmt.Errorf("too many tool call rounds (max %d)", maxToolRounds)
}

func buildMessages(s *soul.Soul, history []brain.Message) []brain.Message {
	messages := []brain.Message{{Role: brain.RoleSystem, Content: s.SystemPrompt}}
	return append(messages, history...)
}

func findSoulFile(explicit string) string {
	if explicit != "" {
		return explicit
	}
	entries, err := os.ReadDir("./souls")
	if err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".soul.md") {
				return filepath.Join("./souls", e.Name())
			}
		}
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".localkin", "souls")
	entries, err = os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".soul.md") {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	return ""
}

func homeSkillsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".localkin", "skills")
}

func loadOAuthToken() string {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".localkin", "auth.json"))
	if err != nil {
		return ""
	}
	var a struct{ AccessToken string `json:"access_token"` }
	if json.Unmarshal(data, &a) != nil {
		return ""
	}
	return a.AccessToken
}
