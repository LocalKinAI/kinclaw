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
	version       = "1.0.0"
	maxToolRounds = 20
)

// session holds the mutable runtime state for the REPL.
type session struct {
	soul     *soul.Soul
	brain    brain.Brain
	registry *skill.Registry
	toolDefs []json.RawMessage
	store    *memory.SQLiteStore
	id       string
	history  []brain.Message
	debug    bool
	soulPath string
}

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

	sess, err := newSession(path, *debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if sess.store != nil {
		defer sess.store.Close()
	}

	fmt.Fprintf(os.Stderr, "\033[2m  LocalKin %s\n  Soul:     %s (%s)\n  Brain:    %s / %s\n  Skills:   %d loaded\033[0m\n\n",
		version, sess.soul.Meta.Name, sess.soul.FilePath,
		sess.soul.Meta.Brain.Provider, sess.soul.Meta.Brain.Model, len(sess.toolDefs))

	if *execMsg != "" {
		os.Exit(runOnce(sess, *execMsg))
	}

	InitHistory(filepath.Join(homeDir(), ".localkin", "readline_history"))
	runREPL(sess)
}

func newSession(soulPath string, debug bool) (*session, error) {
	s, err := soul.LoadSoul(soulPath)
	if err != nil {
		return nil, err
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
		msg := "Error: API key not set. Set brain.api_key in soul file or $ANTHROPIC_API_KEY / $OPENAI_API_KEY"
		if s.Meta.Brain.Provider == "claude" {
			msg += "\n  Tip: run 'localkin -login' to authenticate with Claude (free tier)"
		}
		return nil, fmt.Errorf("%s", msg)
	}

	b := brain.NewBrain(s.Meta.Brain.Provider, s.Meta.Brain.Endpoint,
		s.Meta.Brain.Model, apiKey, s.Meta.Brain.Temperature)

	store, err := memory.OpenMemory(memory.DefaultDBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: memory unavailable: %v\n", err)
	}

	reg := buildRegistry(s)

	sessionID := fmt.Sprintf("%s-%d", s.Meta.Name, os.Getpid())
	var history []brain.Message
	if store != nil {
		history = store.LoadHistory(sessionID, 50)
	}

	return &session{
		soul: s, brain: b, registry: reg,
		toolDefs: reg.FilteredToolDefs(s.Meta.Skills.Enable),
		store: store, id: sessionID, history: history,
		debug: debug, soulPath: soulPath,
	}, nil
}

func buildRegistry(s *soul.Soul) *skill.Registry {
	reg := skill.NewRegistry()
	skillsDir := "./skills"
	if s.Meta.Skills.Dir != "" {
		skillsDir = s.Meta.Skills.Dir
	}
	if s.Meta.Permissions.Shell {
		reg.Register(skill.NewShellSkill(s.Meta.Permissions.ShellTimeout))
		reg.Register(skill.NewForgeSkill(skillsDir, reg))
	}
	reg.Register(skill.NewFileReadSkill())
	reg.Register(skill.NewFileWriteSkill())
	reg.Register(skill.NewFileEditSkill())
	if s.Meta.Permissions.Network {
		reg.Register(skill.NewWebFetchSkill())
		reg.Register(skill.NewWebSearchSkill())
	}
	for _, dir := range []string{skillsDir, homeSkillsDir()} {
		exts, _ := skill.LoadExternalSkills(dir)
		for _, ext := range exts {
			reg.Register(ext)
		}
	}
	return reg
}

func runREPL(sess *session) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Boot message: auto-execute if configured
	if msg := sess.soul.Meta.Boot.Message; msg != "" {
		fmt.Fprintf(os.Stderr, "\033[2m[boot] %s\033[0m\n", msg)
		handleUserMessage(ctx, sess, msg)
	}

	prompt := fmt.Sprintf("\033[1;36m%s>\033[0m ", sess.soul.Meta.Name)
	for {
		input, err := readLine(prompt)
		if err != nil {
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if strings.HasPrefix(input, "/") {
			if handleCommand(ctx, sess, input) {
				return
			}
			continue
		}
		handleUserMessage(ctx, sess, input)
	}
}

func runOnce(sess *session, input string) int {
	ctx := context.Background()
	handleUserMessage(ctx, sess, input)
	if len(sess.history) > 0 {
		last := sess.history[len(sess.history)-1]
		if last.Role == brain.RoleAssistant && last.Content != "" {
			return 0
		}
	}
	return 0
}

func handleUserMessage(ctx context.Context, sess *session, input string) {
	userMsg := brain.Message{Role: brain.RoleUser, Content: input}
	sess.history = append(sess.history, userMsg)
	if sess.store != nil {
		sess.store.SaveMessage(sess.id, userMsg)
	}

	messages := buildMessages(sess.soul, sess.history)
	onChunk := func(chunk string, thinking bool) error {
		if thinking {
			fmt.Fprint(os.Stderr, "\033[2m"+chunk+"\033[0m")
		} else {
			fmt.Print(chunk)
		}
		return nil
	}

	reply, toolHistory, err := chatLoop(ctx, sess, messages, onChunk)
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
		return
	}

	for _, msg := range toolHistory {
		if sess.store != nil {
			sess.store.SaveMessage(sess.id, msg)
		}
		sess.history = append(sess.history, msg)
	}
	assistantMsg := brain.Message{Role: brain.RoleAssistant, Content: reply}
	sess.history = append(sess.history, assistantMsg)
	if sess.store != nil {
		sess.store.SaveMessage(sess.id, assistantMsg)
	}

	// Check if forge created new skills
	for _, msg := range toolHistory {
		if msg.Role == brain.RoleAssistant {
			for _, tc := range msg.ToolCalls {
				if tc.Function.Name == "forge" {
					sess.toolDefs = sess.registry.FilteredToolDefs(sess.soul.Meta.Skills.Enable)
					return
				}
			}
		}
	}
}

func chatLoop(ctx context.Context, sess *session, messages []brain.Message, onChunk brain.StreamFunc) (string, []brain.Message, error) {
	var intermediateHistory []brain.Message
	cb := skill.NewCircuitBreaker()

	for round := 0; round < maxToolRounds; round++ {
		result, err := sess.brain.Chat(ctx, messages, sess.toolDefs, onChunk)
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
			if sess.debug {
				fmt.Fprintf(os.Stderr, "\033[2m[tool: %s %v]\033[0m\n", tc.Function.Name, params)
			}
			callInfos = append(callInfos, skill.ToolCallInfo{ID: tc.ID, Name: tc.Function.Name, Params: params})
		}

		results := skill.ExecuteToolCalls(sess.registry, callInfos)

		// Circuit breaker check
		if tripped, tripMsg := cb.Record(results); tripped {
			fmt.Fprintf(os.Stderr, "\033[33m%s\033[0m\n", tripMsg)
			cbMsg := brain.Message{Role: brain.RoleUser, Content: tripMsg}
			messages = append(messages, cbMsg)
			intermediateHistory = append(intermediateHistory, cbMsg)
		}

		for _, r := range results {
			if sess.debug {
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

// ─── Commands ─────────────────────────────────────────────

// handleCommand processes slash commands. Returns true if REPL should exit.
func handleCommand(ctx context.Context, sess *session, input string) bool {
	parts := strings.Fields(input)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = strings.Join(parts[1:], " ")
	}

	switch cmd {
	case "/quit", "/exit":
		fmt.Fprintln(os.Stderr, "Goodbye.")
		return true

	case "/help":
		fmt.Print("\033[2m" +
			"/quit      Exit\n" +
			"/skills    List available skills\n" +
			"/clear     Clear conversation history\n" +
			"/info      Show soul, model, and token stats\n" +
			"/reload    Reload current soul file\n" +
			"/soul      List or switch soul files\n" +
			"\033[0m")

	case "/skills":
		for _, def := range sess.toolDefs {
			var tool struct {
				Function struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"function"`
			}
			json.Unmarshal(def, &tool)
			fmt.Printf("  \033[1m%-15s\033[0m %s\n", tool.Function.Name, truncate(tool.Function.Description, 60))
		}

	case "/clear":
		sess.history = nil
		fmt.Println("\033[2mConversation cleared.\033[0m")

	case "/info":
		tokens := estimateTokens(sess.history)
		fmt.Printf("\033[2m"+
			"  Version:  %s\n"+
			"  Soul:     %s (%s)\n"+
			"  Brain:    %s / %s\n"+
			"  Skills:   %d loaded\n"+
			"  History:  %d messages (~%d tokens)\n"+
			"\033[0m", version, sess.soul.Meta.Name, sess.soul.FilePath,
			sess.soul.Meta.Brain.Provider, sess.soul.Meta.Brain.Model,
			len(sess.toolDefs), len(sess.history), tokens)

	case "/reload":
		s, err := soul.LoadSoul(sess.soulPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mReload failed: %v\033[0m\n", err)
			break
		}
		sess.soul = s
		sess.registry = buildRegistry(s)
		sess.toolDefs = sess.registry.FilteredToolDefs(s.Meta.Skills.Enable)
		fmt.Printf("\033[2mReloaded %s (%d skills)\033[0m\n", s.Meta.Name, len(sess.toolDefs))

	case "/soul":
		if arg == "" {
			listSouls()
		} else {
			path := findSoulByName(arg)
			if path == "" {
				fmt.Fprintf(os.Stderr, "\033[31mSoul not found: %s\033[0m\n", arg)
				break
			}
			s, err := soul.LoadSoul(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "\033[31mFailed to load: %v\033[0m\n", err)
				break
			}
			sess.soul = s
			sess.soulPath = path
			sess.registry = buildRegistry(s)
			sess.toolDefs = sess.registry.FilteredToolDefs(s.Meta.Skills.Enable)
			sess.history = nil
			fmt.Printf("\033[2mSwitched to %s (%s)\033[0m\n", s.Meta.Name, path)
		}

	default:
		fmt.Fprintf(os.Stderr, "\033[31mUnknown command: %s (try /help)\033[0m\n", cmd)
	}
	return false
}

// ─── Helpers ──────────────────────────────────────────────

func buildMessages(s *soul.Soul, history []brain.Message) []brain.Message {
	messages := []brain.Message{{Role: brain.RoleSystem, Content: s.SystemPrompt}}
	return append(messages, history...)
}

func findSoulFile(explicit string) string {
	if explicit != "" {
		return explicit
	}
	for _, dir := range soulDirs() {
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".soul.md") {
					return filepath.Join(dir, e.Name())
				}
			}
		}
	}
	return ""
}

func soulDirs() []string {
	return []string{"./souls", filepath.Join(homeDir(), ".localkin", "souls")}
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

func homeSkillsDir() string {
	return filepath.Join(homeDir(), ".localkin", "skills")
}

func loadOAuthToken() string {
	data, err := os.ReadFile(filepath.Join(homeDir(), ".localkin", "auth.json"))
	if err != nil {
		return ""
	}
	var a struct{ AccessToken string `json:"access_token"` }
	if json.Unmarshal(data, &a) != nil {
		return ""
	}
	return a.AccessToken
}

func estimateTokens(messages []brain.Message) int {
	total := 0
	for _, m := range messages {
		total += len(strings.Fields(m.Content))
	}
	return int(float64(total) * 1.3)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func listSouls() {
	found := false
	for _, dir := range soulDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".soul.md") {
				name := strings.TrimSuffix(e.Name(), ".soul.md")
				fmt.Printf("  %s  \033[2m(%s)\033[0m\n", name, filepath.Join(dir, e.Name()))
				found = true
			}
		}
	}
	if !found {
		fmt.Println("  \033[2mNo soul files found.\033[0m")
	}
}

func findSoulByName(name string) string {
	if strings.HasSuffix(name, ".soul.md") {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	for _, dir := range soulDirs() {
		path := filepath.Join(dir, name+".soul.md")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
