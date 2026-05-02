// Package server hosts the kinclaw "watch-it-work" UI: a single-file
// HTML page on / and a Server-Sent-Events stream on /api/events that
// pushes every text delta + tool call + tool result the kernel produces.
//
// The transport choice is deliberate. SSE is one direction (server →
// client), one TCP stream, no extra deps, no upgrade dance. The other
// direction is a single POST /api/chat that just kicks a turn — chat
// I/O is asymmetric (user types in bursts, agent streams continuously)
// so SSE matches the shape and lets us keep the deps list at zero.
//
// File serving: the kernel emits absolute filesystem paths for
// screenshots and recordings. /file/<abspath> serves them with a strict
// allow-list prefix check (no .. traversal, no arbitrary disk reads)
// so the browser can render them as <img src> / <video src>.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed index.html
var staticFS embed.FS

// Event is a single UI update pushed to all SSE subscribers. The shape
// is intentionally flat — one struct, JSON-tagged, fields filled per
// Type. Frontend dispatches on Type; missing fields are zero values.
type Event struct {
	Type string `json:"type"`

	// user_message / assistant text — text_delta carries deltas, the
	// frontend appends to the current assistant bubble until a tool_call
	// or turn_done arrives.
	Text     string `json:"text,omitempty"`
	Thinking bool   `json:"thinking,omitempty"`

	// tool_call: which claw, with what params.
	ID     string            `json:"id,omitempty"`
	Name   string            `json:"name,omitempty"`
	Params map[string]string `json:"params,omitempty"`

	// tool_result / screen_frame / record_done.
	Output string   `json:"output,omitempty"`
	Images []string `json:"images,omitempty"` // absolute paths
	URLs   []string `json:"urls,omitempty"`   // browser-fetchable /file/... URLs
	Path   string   `json:"path,omitempty"`
	URL    string   `json:"url,omitempty"`

	// error
	Message string `json:"message,omitempty"`
}

// ChatHandler is invoked for every POST /api/chat. Runs in its own
// goroutine — should call back into Server.Push to stream events.
// Context is request-scoped; the handler should respect cancellation
// (browser closed, etc.).
type ChatHandler func(ctx context.Context, message string)

// InterruptHandler is invoked when the browser asks to abort the
// current turn (DELETE /api/chat or "Esc" in the UI). Implementation
// should cancel whatever ctx the running turn is using. No-op if no
// turn is in flight.
type InterruptHandler func()

// SoulInfo is one entry in the soul-list response. Active marks the
// currently-loaded soul so the UI can highlight it.
type SoulInfo struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Brain  string `json:"brain"`
	Active bool   `json:"active,omitempty"`
}

// SoulListHandler returns the souls available to switch to. Path is
// absolute. The implementation decides the search strategy (typically
// ./souls/ + ~/.localkin/souls/).
type SoulListHandler func() []SoulInfo

// SoulSwitchHandler swaps the running session over to the soul at
// path. Should refuse if a turn is in flight (caller-policy). Returns
// an error if the soul fails to load.
type SoulSwitchHandler func(path string) error

type Server struct {
	addr             string
	chatHandler      ChatHandler
	interruptHandler InterruptHandler
	soulList         SoulListHandler
	soulSwitch       SoulSwitchHandler
	allowedDirs      []string // /file allow-list (absolute, cleaned)

	mu          sync.Mutex
	subs        map[chan Event]struct{}
	hello       *Event // pushed to each new subscriber on connect (soul info)
	eventLogger EventLogger
	// firstSubCh is closed when the first subscriber connects. Replay
	// mode waits on this so it doesn't push the recorded events into
	// the void before the browser opens. Reset to nil after firing.
	firstSubCh chan struct{}

	// Live-screen feed cache. 800ms TTL absorbs client polling faster
	// than the macOS screencapture call returns (~100ms). All access
	// guarded by mu.
	liveScreen      LiveScreenCapture
	liveScreenInfo  LiveScreenInfo
	liveCache       []byte
	liveCacheStamp  time.Time
}

// New constructs a server. allowedDirs are filesystem prefixes that
// /file is willing to serve from (e.g. ~/Library/Caches/kinclaw,
// ./output). Anything outside returns 403. Empty list = no /file
// service at all.
func New(addr string, allowedDirs []string, h ChatHandler) *Server {
	clean := make([]string, 0, len(allowedDirs))
	for _, d := range allowedDirs {
		if abs, err := filepath.Abs(d); err == nil {
			clean = append(clean, abs)
		}
	}
	return &Server{
		addr: addr, chatHandler: h, allowedDirs: clean,
		subs: make(map[chan Event]struct{}),
	}
}

// SetInterruptHandler wires the abort path. Optional — without it
// DELETE /api/chat returns 501 and the UI's interrupt button fails
// gracefully (input stays disabled until normal turn_done).
func (s *Server) SetInterruptHandler(h InterruptHandler) {
	s.interruptHandler = h
}

// SetSoulHandlers wires the soul list + switch endpoints. Both
// optional — without them the UI's dropdown will hit 501 and stay
// in single-soul mode.
func (s *Server) SetSoulHandlers(list SoulListHandler, switcher SoulSwitchHandler) {
	s.soulList = list
	s.soulSwitch = switcher
}

// EventLogger is called for every event Push'd to subscribers. Used
// by the recorder in serve.go to append events to a JSONL session
// log, which `kinclaw serve --replay <file>` can replay verbatim.
// Hook is called BEFORE the broadcast so the log captures all events
// even if a subscriber's channel is full and would have dropped.
type EventLogger func(Event)

// SetEventLogger installs the per-event hook. nil unhooks.
func (s *Server) SetEventLogger(l EventLogger) {
	s.mu.Lock()
	s.eventLogger = l
	s.mu.Unlock()
}

// LiveScreenCapture grabs a fresh screenshot of the user's desktop
// and returns the JPEG bytes. Called by the /api/screen/current.jpg
// route when the browser polls for the live feed. Should be quick
// (~80-150ms typical) — server caches result for 800ms to absorb
// over-eager polling.
type LiveScreenCapture func() ([]byte, error)

// LiveScreenInfo describes what the capture is currently targeting
// — used by the /api/screen/info endpoint so the UI can label the
// feed (e.g. "🔴 LIVE · Reminders" instead of just "🔴 LIVE").
// Implementation may report the tracked app's name; "" means we're
// falling back to whole-display capture.
type LiveScreenInfo func() string

// SetLiveScreenCapture wires the live-screen feed. Without it the
// /api/screen/current.jpg route returns 501 and the UI's "agent's
// eyes" mode falls back to the empty placeholder.
func (s *Server) SetLiveScreenCapture(c LiveScreenCapture) {
	s.mu.Lock()
	s.liveScreen = c
	s.mu.Unlock()
}

// SetLiveScreenInfo wires the metadata callback for the feed. nil
// (or unwired) reports as untracked / whole-screen.
func (s *Server) SetLiveScreenInfo(i LiveScreenInfo) {
	s.mu.Lock()
	s.liveScreenInfo = i
	s.mu.Unlock()
}

// SetHello stores an event that will be pushed to every new SSE
// subscriber as soon as their stream opens. Used to ship soul/brain
// metadata so the page header can render before the first turn.
func (s *Server) SetHello(ev Event) {
	s.mu.Lock()
	s.hello = &ev
	s.mu.Unlock()
}

// Push fans an event out to every subscriber non-blockingly. A slow
// browser tab won't stall the kernel — its channel just drops events
// once full (64-deep buffer, ~plenty of headroom for normal turns).
// Also calls the eventLogger if installed (records BEFORE broadcast
// so every event is captured even if a sub drops).
func (s *Server) Push(ev Event) {
	s.mu.Lock()
	logger := s.eventLogger
	s.mu.Unlock()
	if logger != nil {
		logger(ev)
	}
	s.mu.Lock()
	for ch := range s.subs {
		select {
		case ch <- ev:
		default:
		}
	}
	s.mu.Unlock()
}

// FileURL turns an absolute filesystem path into a /file/... URL the
// browser can fetch. Returns "" for paths that aren't allowed — the
// frontend will fall back to showing the path as text.
func (s *Server) FileURL(abs string) string {
	if abs == "" {
		return ""
	}
	clean, err := filepath.Abs(abs)
	if err != nil {
		return ""
	}
	for _, dir := range s.allowedDirs {
		if strings.HasPrefix(clean, dir+string(os.PathSeparator)) || clean == dir {
			return "/file" + clean
		}
	}
	return ""
}

func (s *Server) subscribe() chan Event {
	ch := make(chan Event, 64)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	signalCh := s.firstSubCh
	s.firstSubCh = nil
	s.mu.Unlock()
	// First-subscriber signal: replay mode blocks on this so the
	// browser is connected before playback fires the first event.
	if signalCh != nil {
		close(signalCh)
	}
	return ch
}

// WaitForFirstSubscriber blocks until at least one SSE client is
// connected (or ctx fires). Returns nil immediately if there's
// already a subscriber. Used by replay mode to gate playback.
func (s *Server) WaitForFirstSubscriber(ctx context.Context) error {
	s.mu.Lock()
	if len(s.subs) > 0 {
		s.mu.Unlock()
		return nil
	}
	if s.firstSubCh == nil {
		s.firstSubCh = make(chan struct{})
	}
	ch := s.firstSubCh
	s.mu.Unlock()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) unsubscribe(ch chan Event) {
	s.mu.Lock()
	delete(s.subs, ch)
	s.mu.Unlock()
	// Drain then close so any in-flight Push goroutines don't panic on
	// send-to-closed (Push holds the mu while sending, but the brief
	// window between unlock and close-by-caller is enough).
	close(ch)
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/souls", s.handleSouls)
	mux.HandleFunc("/api/soul", s.handleSoul)
	mux.HandleFunc("/api/screen/current.jpg", s.handleLiveScreen)
	mux.HandleFunc("/api/screen/info", s.handleLiveScreenInfo)
	mux.HandleFunc("/api/voice/transcribe", s.handleVoiceTranscribe)
	mux.HandleFunc("/api/voice/tts", s.handleVoiceTTS)
	mux.HandleFunc("/file/", s.handleFile)

	// Bind manually rather than using srv.ListenAndServe so we can
	// catch the "address in use" error early and print an
	// actionable message — port 5000/7000 collide with macOS's
	// AirPlay Receiver, which is the #1 cause of "Access denied
	// 403" via browser when the user assumed they were hitting
	// kinclaw.
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		_, port, _ := net.SplitHostPort(s.addr)
		hint := ""
		if port == "5000" || port == "7000" {
			hint = "\n  hint: macOS AirPlay Receiver binds 5000/7000 by default.\n" +
				"        关闭: 系统设置 → 通用 → 隔空播放接收器 → 关\n" +
				"        或换端口: -port 8020 (default) / 8088 / 7777"
		}
		return fmt.Errorf("listen %s: %w%s", s.addr, err, hint)
	}

	srv := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	fmt.Fprintf(os.Stderr, "  serve: http://%s\n", s.addr)
	if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := staticFS.ReadFile("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // bypass nginx buffering if anyone proxies

	ch := s.subscribe()
	defer s.unsubscribe(ch)

	// Hello so the browser flips into "connected" state immediately
	// rather than waiting for the first real event.
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	// Send the hello event (soul info) to this subscriber if one was
	// configured. Per-subscriber, not via the broadcast — late joiners
	// shouldn't replay history, but they do need to know what soul
	// they're talking to.
	s.mu.Lock()
	hello := s.hello
	s.mu.Unlock()
	if hello != nil {
		if data, err := json.Marshal(hello); err == nil {
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleChatPost(w, r)
	case http.MethodDelete:
		s.handleChatDelete(w, r)
	default:
		http.Error(w, "POST or DELETE only", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleChatPost(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	msg := strings.TrimSpace(body.Message)
	if msg == "" {
		http.Error(w, "empty message", http.StatusBadRequest)
		return
	}

	// Echo the user message into the SSE stream so the frontend can
	// just listen and not duplicate render logic.
	s.Push(Event{Type: "user_message", Text: msg})

	// Run the turn in its own goroutine; we return 202 immediately so
	// the browser's POST resolves and the SSE stream is the only
	// long-lived connection.
	go s.chatHandler(context.Background(), msg)
	w.WriteHeader(http.StatusAccepted)
}

// handleChatDelete is the abort endpoint. UI hits this when the user
// presses Esc or clicks the interrupt button. We delegate to the
// installed InterruptHandler (if any), which cancels the in-flight
// turn's ctx; the running runTurn observes the cancellation, pushes
// an error event + turn_done, releases the turn lock.
func (s *Server) handleChatDelete(w http.ResponseWriter, _ *http.Request) {
	if s.interruptHandler == nil {
		http.Error(w, "interrupt not wired", http.StatusNotImplemented)
		return
	}
	s.interruptHandler()
	w.WriteHeader(http.StatusAccepted)
}

// handleSouls returns the list of souls available to swap to.
// JSON: [{path, name, brain, active}]. 501 if the handler isn't wired.
func (s *Server) handleSouls(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	if s.soulList == nil {
		http.Error(w, "soul listing not wired", http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.soulList())
}

// handleSoul switches the running session to a different soul.
// Body: {"path": "/abs/path/to/x.soul.md"}. Returns 202 on success,
// 4xx with the error message if the swap is refused (e.g. turn in
// flight, soul doesn't load).
func (s *Server) handleSoul(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if s.soulSwitch == nil {
		http.Error(w, "soul switching not wired", http.StatusNotImplemented)
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.Path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	if err := s.soulSwitch(body.Path); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleLiveScreen serves a fresh JPEG screenshot of the user's
// desktop on every request — the "agent's eyes" feed. Cached for
// 800ms so the UI's polling at 1.5s + occasional double-fetch don't
// trigger redundant captures.
//
// Cache-Control: no-store; we WANT the browser to refetch each time
// (rather than serve from disk cache) — the URL has a ?t=timestamp
// cache-buster too as belt-and-suspenders.
func (s *Server) handleLiveScreen(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	cap := s.liveScreen
	if cap == nil {
		s.mu.Unlock()
		http.Error(w, "live screen not wired (macOS only?)", http.StatusNotImplemented)
		return
	}
	// Hit cache if recent enough.
	if s.liveCache != nil && time.Since(s.liveCacheStamp) < 800*time.Millisecond {
		data := s.liveCache
		s.mu.Unlock()
		writeImageJPEG(w, data)
		return
	}
	s.mu.Unlock()

	data, err := cap()
	if err != nil {
		http.Error(w, "capture failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	s.liveCache = data
	s.liveCacheStamp = time.Now()
	s.mu.Unlock()
	writeImageJPEG(w, data)
}

func writeImageJPEG(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	_, _ = w.Write(data)
}

// handleLiveScreenInfo returns JSON metadata about what the live
// feed is tracking. UI calls this on a slow cadence (every few sec)
// to update the "🔴 LIVE · <app>" label.
func (s *Server) handleLiveScreenInfo(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	info := s.liveScreenInfo
	s.mu.Unlock()
	app := ""
	if info != nil {
		app = info()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"tracked_app": app})
}

// handleVoiceTranscribe proxies the browser's mic recording to the
// LocalKin Service Audio Server (default :8000 / SenseVoice). The
// request is multipart/form-data with a "file" field; we forward as-
// is. Same response shape: {"text":"...","language":...,"confidence":...}.
//
// Why proxy instead of letting the browser hit :8000 directly:
// CORS. The audio server doesn't set Access-Control-Allow-Origin,
// so the browser would 403 on a cross-origin POST. Proxying keeps
// it single-origin from the browser's view.
func (s *Server) handleVoiceTranscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	endpoint := os.Getenv("STT_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8000"
	}
	target := strings.TrimRight(endpoint, "/") + "/transcribe"

	upstream, err := http.NewRequest("POST", target, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Pass through Content-Type (incl. multipart boundary) — that's
	// the only header the audio server needs to parse the upload.
	if ct := r.Header.Get("Content-Type"); ct != "" {
		upstream.Header.Set("Content-Type", ct)
	}
	// 60s for a long-ish recording. Local SenseVoice typically
	// transcribes in under 2-3s for normal sentence-length input.
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(upstream)
	if err != nil {
		http.Error(w, "STT server unreachable: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// handleVoiceTTS proxies a {text, speaker?} JSON request to the
// LocalKin TTS server (default :8001 / Kokoro). Returns audio/wav
// bytes the browser can play via <audio> or AudioContext.
//
// CJK auto-detection happens client-side in our index.html (it picks
// the speaker based on text content) — keeping the server proxy
// dumb. If the body already has a "speaker" field, it's preserved.
func (s *Server) handleVoiceTTS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	endpoint := os.Getenv("TTS_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8001"
	}
	target := strings.TrimRight(endpoint, "/") + "/synthesize"

	upstream, err := http.NewRequest("POST", target, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	upstream.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(upstream)
	if err != nil {
		http.Error(w, "TTS server unreachable: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	// Forward response — audio/wav typically ~500KB-2MB for a normal
	// reply length. Streaming the body chunks rather than buffering
	// the whole thing keeps memory flat on long synthesis.
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// handleFile serves files from the allow-list. Path traversal: we
// filepath.Clean and require the result to live under one of the
// allowed dirs (prefix + path separator, so /tmp/foo doesn't match
// /tmp/foobar). Caller is the same machine running the agent —
// this is defense-in-depth not a hardened service.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimPrefix(r.URL.Path, "/file")
	if rawPath == "" || rawPath[0] != '/' {
		http.NotFound(w, r)
		return
	}
	clean := filepath.Clean(rawPath)
	allowed := false
	for _, dir := range s.allowedDirs {
		if strings.HasPrefix(clean, dir+string(os.PathSeparator)) || clean == dir {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "path not allowed", http.StatusForbidden)
		return
	}
	// Set a permissive cache for screenshots so flipping back to a
	// frame doesn't refetch. Recordings get the same — they're
	// content-addressed by mtime+name.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, clean)
}
