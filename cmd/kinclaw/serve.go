// serve.go — `kinclaw serve` subcommand.
//
// Spins up the chat-UI HTTP server (pkg/server) and bridges browser
// chat → kernel turn → SSE events. The runTurn function below mirrors
// chatLoop in main.go but reports through srv.Push instead of stdout
// so every text delta + tool call + tool result becomes a UI event.
//
// Why a parallel loop instead of refactoring chatLoop to take a sink:
// chatLoop's stdout shape (printing chunks directly + debug stderr)
// is what the REPL and -exec depend on; serve mode wants structured
// events with tool ids, image URL resolution, and forge-detection. A
// duplicate ~80-line function is cleaner than a generic sink interface
// every caller would have to construct, and keeps the REPL hot path
// allocation-free.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/LocalKinAI/kinclaw/pkg/brain"
	"github.com/LocalKinAI/kinclaw/pkg/server"
	"github.com/LocalKinAI/kinclaw/pkg/skill"
	"github.com/LocalKinAI/kinclaw/pkg/soul"
)

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	soulPath := fs.String("soul", "", "Path to .soul.md file (defaults to ./souls/pilot.soul.md)")
	// 8020 not 8019 — localkin (sibling project, "always running") sits
	// on 8019 IPv6 wildcard and the collision is a footgun even though
	// our IPv4 bind technically wins. Pick a neighbour port instead.
	addr := fs.String("addr", "127.0.0.1:8020", "HTTP listen address (host:port)")
	// -port is the common case shortcut. If non-zero, it overrides the
	// port portion of -addr (host stays 127.0.0.1). For LAN binding
	// (-addr 0.0.0.0:9000) use -addr directly.
	port := fs.Int("port", 0, "Port shortcut (overrides -addr port; host stays 127.0.0.1)")
	// -replay PATH plays a recorded session log instead of running a
	// real soul. Useful for showing demos without spending tokens or
	// for reviewing a past run frame-by-frame.
	replay := fs.String("replay", "", "Replay a recorded session JSONL file (read-only mode)")
	// -no-record disables the per-server-run JSONL log. Default is on
	// because recordings are tiny and let you replay later.
	noRecord := fs.Bool("no-record", false, "Disable session JSONL recording")
	debug := fs.Bool("debug", false, "Show kernel debug output on stderr (browser stays clean)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `kinclaw serve — chat UI · 看着 5 爪干活

Usage:
  kinclaw serve [-soul PATH] [-port N | -addr HOST:PORT] [-debug]

Examples:
  kinclaw serve                              # 127.0.0.1:8020 (default)
  kinclaw serve -port 9000                   # 127.0.0.1:9000
  kinclaw serve -addr 0.0.0.0:9000           # bind LAN, accept remote tabs
  kinclaw serve -soul ./souls/marketer.soul.md -port 8888

Opens an HTTP server with a single-page UI:
  · left:  chat box (你说话,kinclaw 流式回)
  · right: live screen flipbook + tool result cards (5 爪每帧都在)

Open the printed URL in a browser. Ctrl-C to quit.

Flags:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	// -port short form wins if set (typed it explicitly), else fall
	// through to -addr's value.
	if *port > 0 {
		if *port < 1 || *port > 65535 {
			fmt.Fprintf(os.Stderr, "Error: -port must be 1..65535 (got %d)\n", *port)
			os.Exit(2)
		}
		*addr = fmt.Sprintf("127.0.0.1:%d", *port)
	}

	// Replay mode short-circuits the entire soul/session pipeline —
	// it just plays back recorded events at original timing.
	if *replay != "" {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		runReplayServer(ctx, *addr, *replay)
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

	// /file allow-list. Anywhere a skill might write a screenshot or
	// recording. ~/Library/Caches/kinclaw is the default OutputDir for
	// screen + record; ~/.kinclaw holds product-specific state (serve
	// recordings, harvest artifacts, learned.md); ~/.localkin holds
	// holds shared family runtime (memory.db, souls, audio caches —
	// some of those skills emit /file URLs from there); ./output is
	// where marketing demos and similar land.
	home := homeDir()
	allowed := []string{
		filepath.Join(home, "Library", "Caches", "kinclaw"),
		filepath.Join(home, ".kinclaw"),
		filepath.Join(home, ".localkin"),
		"./output",
	}
	// If the soul declared a custom output_dir, allow that too.
	if od := sess.soul.Meta.Skills.OutputDir; od != "" {
		allowed = append(allowed, expandHome(od))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Serialize turns — only one in flight at a time. The UI prevents
	// a second submit from getting through but defense-in-depth: if
	// somebody POSTs /api/chat directly while a turn is running we
	// reply with a "busy" event rather than racing on sess.history.
	var turnMu sync.Mutex
	var srv *server.Server

	// Track the in-flight turn's cancel func so DELETE /api/chat (the
	// "Esc / interrupt" path) can stop it. Guarded by cancelMu — set
	// when a turn starts, cleared on exit, called from the interrupt
	// handler. nil = no turn in flight, interrupt is a no-op.
	var cancelMu sync.Mutex
	var currentCancel context.CancelFunc

	// currentSess is swappable behind sessMu so the soul switcher can
	// hot-replace it. chatHandler / runTurn deref it on each call so
	// in-flight nothing-happens-here turns will use the OLD session
	// (we hold turnMu through the swap, so this is consistent).
	var sessMu sync.Mutex
	currentSess := sess

	chatHandler := func(_ context.Context, message string) {
		if !turnMu.TryLock() {
			srv.Push(server.Event{Type: "error", Message: "已有任务在跑,等当前回合结束"})
			return
		}
		defer turnMu.Unlock()

		sessMu.Lock()
		s := currentSess
		sessMu.Unlock()

		turnCtx, cancel := context.WithCancel(ctx)
		cancelMu.Lock()
		currentCancel = cancel
		cancelMu.Unlock()
		defer func() {
			cancelMu.Lock()
			currentCancel = nil
			cancelMu.Unlock()
			cancel()
		}()

		runTurn(turnCtx, s, srv, message)
	}

	interruptHandler := func() {
		cancelMu.Lock()
		c := currentCancel
		cancelMu.Unlock()
		if c != nil {
			c()
		}
	}

	soulListHandler := func() []server.SoulInfo {
		sessMu.Lock()
		activePath, _ := filepath.Abs(currentSess.soulPath)
		sessMu.Unlock()
		out := listAvailableSouls()
		for i := range out {
			if out[i].Path == activePath {
				out[i].Active = true
			}
		}
		return out
	}

	soulSwitchHandler := func(newPath string) error {
		// Refuse mid-turn — turn loop holds sess.history; swapping
		// underneath would either lose pending tool results or apply
		// them to the wrong soul's context.
		if !turnMu.TryLock() {
			return fmt.Errorf("turn in progress, cancel first (Esc) then switch")
		}
		defer turnMu.Unlock()

		newSess, err := newSession(newPath, *debug)
		if err != nil {
			return err
		}

		sessMu.Lock()
		oldSess := currentSess
		currentSess = newSess
		sessMu.Unlock()

		// Old session's sqlite handle stays valid for its history but
		// we don't need to read from it anymore. Closing it now releases
		// the file lock so a future "switch back" can reopen cleanly.
		if oldSess.store != nil {
			oldSess.store.Close()
		}

		// Repoint hello so any new SSE subscribers (or page reloads)
		// see the right soul up front, and notify currently-connected
		// clients via a soul_switched event.
		srv.SetHello(server.Event{
			Type: "hello",
			Name: newSess.soul.Meta.Name,
			Params: map[string]string{
				"brain":  fmt.Sprintf("%s/%s", newSess.soul.Meta.Brain.Provider, newSess.soul.Meta.Brain.Model),
				"skills": fmt.Sprintf("%d", len(newSess.toolDefs)),
			},
		})
		srv.Push(server.Event{
			Type: "soul_switched",
			Name: newSess.soul.Meta.Name,
			Params: map[string]string{
				"brain":  fmt.Sprintf("%s/%s", newSess.soul.Meta.Brain.Provider, newSess.soul.Meta.Brain.Model),
				"skills": fmt.Sprintf("%d", len(newSess.toolDefs)),
			},
		})
		return nil
	}

	srv = server.New(*addr, allowed, chatHandler)
	srv.SetInterruptHandler(interruptHandler)
	srv.SetSoulHandlers(soulListHandler, soulSwitchHandler)
	// Live-screen feed is intentionally not wired — the UI is now a
	// floating chat-only remote, no in-pane preview. The endpoint
	// stays registered (returns 501) in case a future tool wants to
	// proxy a screenshot through the same path.

	// Per-server-run JSONL recording. ~/.kinclaw/serve-sessions/
	// <YYYYMMDD-HHMMSS>.jsonl, one line per Event. `kinclaw serve
	// --replay <file>` plays it back. Disabled with -no-record.
	var recordPath string
	if !*noRecord {
		rec, p, err := openSessionRecorder()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warn: recording disabled (%v)\n", err)
		} else {
			recordPath = p
			srv.SetEventLogger(rec.log)
			defer rec.close()
		}
	}

	helloEv := server.Event{
		Type: "hello",
		Name: sess.soul.Meta.Name,
		Params: map[string]string{
			"brain":  fmt.Sprintf("%s/%s", sess.soul.Meta.Brain.Provider, sess.soul.Meta.Brain.Model),
			"skills": fmt.Sprintf("%d", len(sess.toolDefs)),
		},
	}
	srv.SetHello(helloEv)
	// Also push hello through Push so the recorder captures it as
	// the first event of the file — replay then starts with the right
	// soul/brain in the header instead of "— soul loading —".
	if recordPath != "" {
		srv.Push(helloEv)
	}

	fmt.Fprintf(os.Stderr,
		"\033[2m  LocalKin %s\n  Soul:     %s (%s)\n  Brain:    %s / %s\n  Skills:   %d loaded\033[0m\n",
		version, sess.soul.Meta.Name, sess.soul.FilePath,
		sess.soul.Meta.Brain.Provider, sess.soul.Meta.Brain.Model, len(sess.toolDefs))
	if recordPath != "" {
		fmt.Fprintf(os.Stderr, "  Record:   %s\n", recordPath)
	}
	fmt.Fprintf(os.Stderr, "  Open:     \033[1mhttp://%s/\033[0m\n", browserAddr(*addr))
	fmt.Fprintf(os.Stderr, "  Float:    \033[1mhttp://%s/?compact\033[0m  (chat-only,小窗贴角)\n", browserAddr(*addr))
	fmt.Fprintf(os.Stderr, "\033[2m  Tip: float 模式做 always-on-top:\n"+
		"    chrome --app=http://%s/?compact     # standalone window 模式\n"+
		"    或 Rectangle / Hammerspoon 给窗口绑 \"always on top\" 快捷键\033[0m\n\n",
		browserAddr(*addr))

	if err := srv.ListenAndServe(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(1)
	}
}

// runTurn drives one user → assistant → tool* → assistant cycle and
// pushes structured events to the SSE stream. Direct port of chatLoop
// (main.go) shaped for the UI: every text chunk emits text_delta,
// every dispatched call emits tool_call, every result emits tool_result
// with image paths resolved to /file URLs the browser can fetch.
func runTurn(ctx context.Context, sess *session, srv *server.Server, input string) {
	userMsg := brain.Message{Role: brain.RoleUser, Content: input}
	sess.history = append(sess.history, userMsg)
	if sess.store != nil {
		sess.store.SaveMessage(sess.id, userMsg)
	}

	messages := buildMessages(sess.soul, sess.history)

	onChunk := func(chunk string, thinking bool) error {
		srv.Push(server.Event{Type: "text_delta", Text: chunk, Thinking: thinking})
		return nil
	}

	var intermediateHistory []brain.Message
	cb := skill.NewCircuitBreaker()
	forgeFired := false

	for round := 0; round < maxToolRounds; round++ {
		result, err := sess.brain.Chat(ctx, messages, sess.toolDefs, onChunk)
		if err != nil {
			srv.Push(server.Event{Type: "error", Message: err.Error()})
			persistHistory(sess, intermediateHistory)
			srv.Push(server.Event{Type: "turn_done"})
			return
		}
		if len(result.ToolCalls) == 0 {
			// Final assistant message.
			assistantMsg := brain.Message{Role: brain.RoleAssistant, Content: result.Content}
			persistHistory(sess, intermediateHistory)
			sess.history = append(sess.history, assistantMsg)
			if sess.store != nil {
				sess.store.SaveMessage(sess.id, assistantMsg)
			}
			if forgeFired {
				sess.toolDefs = sess.registry.FilteredToolDefs(sess.soul.Meta.Skills.Enable)
			}
			srv.Push(server.Event{Type: "turn_done"})
			return
		}

		assistantMsg := brain.Message{
			Role: brain.RoleAssistant, Content: result.Content, ToolCalls: result.ToolCalls,
		}
		messages = append(messages, assistantMsg)
		intermediateHistory = append(intermediateHistory, assistantMsg)

		var callInfos []skill.ToolCallInfo
		for _, tc := range result.ToolCalls {
			if tc.Function.Name == "forge" {
				forgeFired = true
			}
			params, perr := tc.ParseArguments()
			if perr != nil {
				toolMsg := brain.Message{
					Role: brain.RoleTool, Content: "Error: " + perr.Error(), ToolCallID: tc.ID,
				}
				messages = append(messages, toolMsg)
				intermediateHistory = append(intermediateHistory, toolMsg)
				srv.Push(server.Event{
					Type: "tool_error", ID: tc.ID, Name: tc.Function.Name, Message: perr.Error(),
				})
				continue
			}
			srv.Push(server.Event{
				Type: "tool_call", ID: tc.ID, Name: tc.Function.Name, Params: params,
			})
			callInfos = append(callInfos, skill.ToolCallInfo{
				ID: tc.ID, Name: tc.Function.Name, Params: params,
			})
		}

		results := skill.ExecuteToolCalls(sess.registry, callInfos)

		if tripped, tripMsg := cb.Record(results); tripped {
			cbMsg := brain.Message{Role: brain.RoleUser, Content: tripMsg}
			messages = append(messages, cbMsg)
			intermediateHistory = append(intermediateHistory, cbMsg)
			srv.Push(server.Event{Type: "error", Message: tripMsg})
		}

		for _, r := range results {
			urls := make([]string, 0, len(r.Images))
			for _, p := range r.Images {
				if u := srv.FileURL(p); u != "" {
					urls = append(urls, u)
				}
			}
			// Pull video / image paths out of structured `path: ...` lines
			// (record stop uses this shape; screen capture uses image://
			// markers which already populated r.Images).
			for _, p := range extractStructuredPaths(r.Output) {
				if u := srv.FileURL(p); u != "" {
					urls = append(urls, u)
				}
			}
			srv.Push(server.Event{
				Type: "tool_result", ID: r.ToolCallID, Name: r.Name,
				Output: r.Output, Images: r.Images, URLs: urls,
			})

			toolMsg := brain.Message{
				Role: brain.RoleTool, Content: r.Output, ToolCallID: r.ToolCallID,
				Images: r.Images,
			}
			messages = append(messages, toolMsg)
			intermediateHistory = append(intermediateHistory, toolMsg)
		}
	}

	srv.Push(server.Event{
		Type: "error", Message: fmt.Sprintf("too many tool rounds (max %d)", maxToolRounds),
	})
	persistHistory(sess, intermediateHistory)
	srv.Push(server.Event{Type: "turn_done"})
}

func persistHistory(sess *session, history []brain.Message) {
	for _, msg := range history {
		if sess.store != nil {
			sess.store.SaveMessage(sess.id, msg)
		}
		sess.history = append(sess.history, msg)
	}
}

// extractStructuredPaths picks up `path: /abs/foo.mp4` lines from
// structured tool output (record stop / screen capture's text body).
// Returns absolute paths whose extension we recognize as renderable.
var pathRe = regexp.MustCompile(`(?m)^\s*path:\s*(/[^\s]+\.(?:mp4|mov|m4v|png|jpe?g|webp|gif))\s*$`)

func extractStructuredPaths(out string) []string {
	if out == "" {
		return nil
	}
	matches := pathRe.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		return nil
	}
	out2 := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) >= 2 && !seen[m[1]] {
			seen[m[1]] = true
			out2 = append(out2, m[1])
		}
	}
	return out2
}

// listAvailableSouls scans the standard soul dirs (./souls/ +
// ~/.localkin/souls/, per soulDirs() in main.go) for *.soul.md files
// and returns their meta. Skips files that fail to parse — broken
// souls just don't show up in the dropdown.
func listAvailableSouls() []server.SoulInfo {
	var out []server.SoulInfo
	seen := map[string]bool{}
	for _, dir := range soulDirs() {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.soul.md"))
		for _, f := range matches {
			abs, err := filepath.Abs(f)
			if err != nil || seen[abs] {
				continue
			}
			seen[abs] = true
			s, err := soul.LoadSoul(f)
			if err != nil {
				continue
			}
			out = append(out, server.SoulInfo{
				Path:  abs,
				Name:  s.Meta.Name,
				Brain: fmt.Sprintf("%s/%s", s.Meta.Brain.Provider, s.Meta.Brain.Model),
			})
		}
	}
	return out
}

// browserAddr converts a listen address into something a human can
// click. 0.0.0.0:8019 → 127.0.0.1:8019 (browsers won't navigate to
// 0.0.0.0). Bare ports like ":8019" get the same treatment.
func browserAddr(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return listen
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// (Live-screen tracker + screencapture wrapper removed when the
// right-pane preview was retired. The /api/screen/current.jpg
// endpoint stays registered server-side and returns 501 unless a
// future caller installs a LiveScreenCapture handler.)

// expandHome resolves a leading ~ to the user's home dir. We accept
// "~/foo" and "~user/foo" forms; bare "~" expands to home.
func expandHome(p string) string {
	if p == "" || p[0] != '~' {
		return p
	}
	if p == "~" {
		return homeDir()
	}
	if len(p) > 1 && p[1] == '/' {
		return filepath.Join(homeDir(), p[2:])
	}
	return p
}

// ─── session recording ────────────────────────────────────────
// recordEntry is one line of the JSONL log. TS is wall-clock ms so
// replay can reproduce the original timing (capped to keep idle gaps
// from making playback boring).
type recordEntry struct {
	TS    int64        `json:"ts_ms"`
	Event server.Event `json:"event"`
}

type sessionRecorder struct {
	f  *os.File
	mu sync.Mutex
}

func (r *sessionRecorder) log(ev server.Event) {
	if r == nil || r.f == nil {
		return
	}
	entry := recordEntry{TS: time.Now().UnixMilli(), Event: ev}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	r.mu.Lock()
	_, _ = r.f.Write(data)
	_, _ = r.f.Write([]byte("\n"))
	r.mu.Unlock()
}

func (r *sessionRecorder) close() {
	if r == nil || r.f == nil {
		return
	}
	r.mu.Lock()
	_ = r.f.Close()
	r.mu.Unlock()
}

// openSessionRecorder creates ~/.kinclaw/serve-sessions/<ts>.jsonl
// and returns the recorder + its path. Caller installs r.log as the
// EventLogger and defers r.close() before exit.
//
// Pre-2026-05-03 this was ~/.localkin/serve-sessions/ — moved to
// ~/.kinclaw/ since serve recordings are kinclaw-specific output,
// not LocalKin family runtime data.
func openSessionRecorder() (*sessionRecorder, string, error) {
	dir := filepath.Join(homeDir(), ".kinclaw", "serve-sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}
	name := time.Now().Format("20060102-150405") + ".jsonl"
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, "", err
	}
	return &sessionRecorder{f: f}, path, nil
}

// ─── replay mode ──────────────────────────────────────────────
// runReplayServer plays a recorded JSONL session log into a fresh
// server. chat is rejected (read-only mode), Esc cancels playback,
// soul switcher stays available but with the live-mode handler
// disabled. Caller passes a ctx that gets canceled on SIGINT.
func runReplayServer(ctx context.Context, addr, replayPath string) {
	abs, err := filepath.Abs(replayPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: bad replay path: %v\n", err)
		os.Exit(1)
	}
	f, err := os.Open(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: open replay file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	// Read all entries up front so we can show event count + check
	// for malformed lines without being mid-playback when something
	// breaks. Recordings are tiny (~few hundred KB even for long
	// turns) so this is fine.
	var entries []recordEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1 MB max line
	for scanner.Scan() {
		var e recordEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, e)
	}
	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "Error: replay file empty or no valid events: %s\n", abs)
		os.Exit(1)
	}

	// Allowed dirs for /file in replay — same set as live mode plus
	// wherever the original recording references. We can't introspect
	// every URL so just allow the standard locations; URLs outside
	// will 404 in the browser (graceful).
	home := homeDir()
	allowed := []string{
		filepath.Join(home, "Library", "Caches", "kinclaw"),
		filepath.Join(home, ".kinclaw"),
		filepath.Join(home, ".localkin"),
		"./output",
	}

	// Stub chat handler — reject with a friendly message.
	chatStub := func(_ context.Context, _ string) {
		// Server.handleChatPost echoes user_message before we get here,
		// so error here completes the visual.
	}

	// Replay control: a single cancelable context for the playback
	// goroutine. Esc / DELETE /api/chat stops playback.
	playCtx, playCancel := context.WithCancel(ctx)
	defer playCancel()

	srv := server.New(addr, allowed, chatStub)
	srv.SetInterruptHandler(func() { playCancel() })
	srv.SetHello(server.Event{
		Type: "hello",
		Name: "REPLAY",
		Params: map[string]string{
			"brain":  "playback",
			"replay": filepath.Base(abs),
		},
	})

	// Override chatStub: in replay, any POST should bounce back as
	// an error so the UI shows "replay 模式,无法对话".
	chatRejectHandler := func(_ context.Context, _ string) {
		srv.Push(server.Event{Type: "error", Message: "replay 模式 · 无法发新消息"})
	}
	// Re-wire by creating a new server with the proper handler.
	// (Cleaner than mutating srv; chatStub above was just a placeholder
	// because Server requires non-nil handler at construction.)
	srv = server.New(addr, allowed, chatRejectHandler)
	srv.SetInterruptHandler(func() { playCancel() })
	srv.SetHello(server.Event{
		Type: "hello",
		Name: "REPLAY · " + filepath.Base(abs),
		Params: map[string]string{
			"brain":  fmt.Sprintf("%d events", len(entries)),
			"replay": "1",
		},
	})

	// Playback goroutine. Sleep deltas between events, capped at 2s
	// so a long brain pause doesn't make the user wait for nothing.
	// We block on first-subscriber so events recorded before the
	// browser opens (e.g. the initial hello) don't fire into the void.
	go func() {
		if err := srv.WaitForFirstSubscriber(playCtx); err != nil {
			return
		}
		// Tiny grace so the browser finishes initial render before
		// the first event lands.
		select {
		case <-time.After(200 * time.Millisecond):
		case <-playCtx.Done():
			return
		}
		var prevTS int64
		for i, e := range entries {
			if playCtx.Err() != nil {
				srv.Push(server.Event{Type: "error", Message: "replay 已取消"})
				return
			}
			// Skip recorded hello — replay mode has its own ("REPLAY ·
			// <file>") and we don't want to overwrite it with the
			// original soul name. Same for soul_switched-during-replay
			// would be confusing; we keep that one because it might be
			// part of the meaningful narrative being replayed.
			if e.Event.Type == "hello" {
				prevTS = e.TS
				continue
			}
			if i > 0 {
				delta := e.TS - prevTS
				if delta < 0 {
					delta = 0
				}
				if delta > 2000 {
					delta = 2000
				}
				if delta > 0 {
					select {
					case <-time.After(time.Duration(delta) * time.Millisecond):
					case <-playCtx.Done():
						return
					}
				}
			}
			prevTS = e.TS
			srv.Push(e.Event)
		}
		// Tail event so the UI knows playback is done.
		srv.Push(server.Event{Type: "turn_done"})
		srv.Push(server.Event{Type: "error", Message: fmt.Sprintf("replay 完成 · %d events", len(entries))})
	}()

	fmt.Fprintf(os.Stderr,
		"\033[2m  LocalKin %s · REPLAY MODE\n  File:     %s\n  Events:   %d\033[0m\n",
		version, abs, len(entries))
	fmt.Fprintf(os.Stderr, "  Open:     \033[1mhttp://%s/\033[0m\n\n", browserAddr(addr))

	if err := srv.ListenAndServe(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(1)
	}
}
