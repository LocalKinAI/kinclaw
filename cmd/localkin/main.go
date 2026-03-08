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

	kin "github.com/LocalKinAI/localkin"
)

const (
	version          = "0.1.0"
	maxToolRounds    = 20
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
		if err := kin.Login(); err != nil {
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

	soul, err := kin.LoadSoul(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	apiKey := soul.Meta.Brain.APIKey
	if apiKey == "" {
		switch soul.Meta.Brain.Provider {
		case "claude":
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
			if apiKey == "" {
				apiKey = loadOAuthToken()
			}
		case "openai":
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
	}
	if apiKey == "" && soul.Meta.Brain.Provider != "ollama" {
		fmt.Fprintf(os.Stderr, "Error: API key not set. Set brain.api_key in soul file or $ANTHROPIC_API_KEY / $OPENAI_API_KEY\n")
		os.Exit(1)
	}

	brain := kin.NewBrain(soul.Meta.Brain.Provider, soul.Meta.Brain.Endpoint,
		soul.Meta.Brain.Model, apiKey, soul.Meta.Brain.Temperature)

	store, err := kin.OpenMemory(kin.DefaultDBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: memory unavailable: %v\n", err)
	}
	if store != nil {
		defer store.Close()
	}

	registry := kin.NewRegistry()
	skillsDir := "./skills"
	if soul.Meta.Skills.Dir != "" {
		skillsDir = soul.Meta.Skills.Dir
	}

	if soul.Meta.Permissions.Shell {
		registry.Register(kin.NewShellSkill(30))
		registry.Register(kin.NewForgeSkill(skillsDir, registry))
	}
	registry.Register(kin.NewFileReadSkill())
	registry.Register(kin.NewFileWriteSkill())
	if soul.Meta.Permissions.Network {
		registry.Register(kin.NewWebFetchSkill())
	}
	if store != nil {
		registry.Register(kin.NewMemorySkill(store))
	}

	for _, dir := range []string{skillsDir, homeSkillsDir()} {
		exts, _ := kin.LoadExternalSkills(dir)
		for _, ext := range exts {
			registry.Register(ext)
		}
	}

	toolDefs := registry.FilteredToolDefs(soul.Meta.Skills.Enable)
	fmt.Fprintf(os.Stderr, "\033[2m  LocalKin %s\n  Soul:     %s (%s)\n  Brain:    %s / %s\n  Skills:   %d loaded\033[0m\n\n",
		version, soul.Meta.Name, soul.FilePath, soul.Meta.Brain.Provider, soul.Meta.Brain.Model, len(toolDefs))
	sessionID := fmt.Sprintf("%s-%d", soul.Meta.Name, os.Getpid())
	var history []kin.Message
	if store != nil {
		history = store.LoadHistory(sessionID, 50)
	}
	if *execMsg != "" {
		os.Exit(runOnce(brain, soul, registry, toolDefs, store, sessionID, history, *execMsg, *debug))
	}
	runREPL(brain, soul, registry, toolDefs, store, sessionID, history, *debug)
}

func runREPL(brain kin.Brain, soul *kin.Soul, registry *kin.Registry, toolDefs []json.RawMessage, store *kin.SQLiteStore, sessionID string, history []kin.Message, debug bool) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	prompt := fmt.Sprintf("\033[1;36m%s>\033[0m ", soul.Meta.Name)
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
		userMsg := kin.Message{Role: kin.RoleUser, Content: input}
		history = append(history, userMsg)
		if store != nil {
			store.SaveMessage(sessionID, userMsg)
		}
		messages := buildMessages(soul, history)
		onChunk := func(chunk string, thinking bool) error {
			if thinking {
				fmt.Fprint(os.Stderr, "\033[2m"+chunk+"\033[0m")
			} else {
				fmt.Print(chunk)
			}
			return nil
		}
		reply, toolHistory, err := chatLoop(ctx, brain, messages, toolDefs, registry, onChunk, debug)
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
		assistantMsg := kin.Message{Role: kin.RoleAssistant, Content: reply}
		history = append(history, assistantMsg)
		if store != nil {
			store.SaveMessage(sessionID, assistantMsg)
		}
		for _, msg := range toolHistory {
			if msg.Role == kin.RoleAssistant {
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

func runOnce(brain kin.Brain, soul *kin.Soul, registry *kin.Registry, toolDefs []json.RawMessage, store *kin.SQLiteStore, sessionID string, history []kin.Message, input string, debug bool) int {
	ctx := context.Background()

	userMsg := kin.Message{Role: kin.RoleUser, Content: input}
	history = append(history, userMsg)
	if store != nil {
		store.SaveMessage(sessionID, userMsg)
	}

	messages := buildMessages(soul, history)

	onChunk := func(chunk string, thinking bool) error {
		if thinking {
			fmt.Fprint(os.Stderr, "\033[2m"+chunk+"\033[0m")
		} else {
			fmt.Print(chunk) // stdout for piping
		}
		return nil
	}

	reply, toolHistory, err := chatLoop(ctx, brain, messages, toolDefs, registry, onChunk, debug)
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
		store.SaveMessage(sessionID, kin.Message{Role: kin.RoleAssistant, Content: reply})
	}

	return 0
}

func chatLoop(ctx context.Context, brain kin.Brain, messages []kin.Message, toolDefs []json.RawMessage, registry *kin.Registry, onChunk kin.StreamFunc, debug bool) (string, []kin.Message, error) {
	var intermediateHistory []kin.Message

	for round := 0; round < maxToolRounds; round++ {
		result, err := brain.Chat(ctx, messages, toolDefs, onChunk)
		if err != nil {
			return "", nil, err
		}

		if len(result.ToolCalls) == 0 {
			return result.Content, intermediateHistory, nil
		}

		assistantMsg := kin.Message{
			Role:      kin.RoleAssistant,
			Content:   result.Content,
			ToolCalls: result.ToolCalls,
		}
		messages = append(messages, assistantMsg)
		intermediateHistory = append(intermediateHistory, assistantMsg)

		var callInfos []kin.ToolCallInfo
		for _, tc := range result.ToolCalls {
			params, err := tc.ParseArguments()
			if err != nil {
				toolMsg := kin.Message{Role: kin.RoleTool, Content: "Error: " + err.Error(), ToolCallID: tc.ID}
				messages = append(messages, toolMsg)
				intermediateHistory = append(intermediateHistory, toolMsg)
				continue
			}
			if debug {
				fmt.Fprintf(os.Stderr, "\033[2m[tool: %s %v]\033[0m\n", tc.Function.Name, params)
			}
			callInfos = append(callInfos, kin.ToolCallInfo{ID: tc.ID, Name: tc.Function.Name, Params: params})
		}

		results := kin.ExecuteToolCalls(registry, callInfos)
		for _, r := range results {
			if debug {
				display := r.Output
				if len(display) > 200 {
					display = display[:200] + "..."
				}
				fmt.Fprintf(os.Stderr, "\033[2m[%s → %s]\033[0m\n", r.Name, strings.ReplaceAll(display, "\n", " "))
			}
			toolMsg := kin.Message{Role: kin.RoleTool, Content: r.Output, ToolCallID: r.ToolCallID}
			messages = append(messages, toolMsg)
			intermediateHistory = append(intermediateHistory, toolMsg)
		}
	}

	return "", intermediateHistory, fmt.Errorf("too many tool call rounds (max %d)", maxToolRounds)
}

func buildMessages(soul *kin.Soul, history []kin.Message) []kin.Message {
	messages := []kin.Message{{Role: kin.RoleSystem, Content: soul.SystemPrompt}}
	messages = append(messages, history...)
	return messages
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
	var auth struct{ AccessToken string `json:"access_token"` }
	if json.Unmarshal(data, &auth) != nil {
		return ""
	}
	return auth.AccessToken
}
