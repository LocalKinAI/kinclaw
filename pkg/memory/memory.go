package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/LocalKinAI/kinclaw/pkg/brain"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct{ db *sql.DB }

// DefaultDBPath returns the path KinClaw uses to persist conversation
// history + durable memories.
//
// === Shared with the LocalKin family (intentional design) ===
//
// The default path is "~/.localkin/memory.db" — INTENTIONALLY shared
// with the LocalKin runtime, kin-code, and any other sibling product
// from the same family. They all read/write this single SQLite file.
// Each soul's messages live in its own session_id bucket; soul naming
// convention prevents collision (KinClaw souls prefix with "KinClaw ").
//
// Why shared:
//   - Cross-product user-facts: telling LocalKin's coder "I'm in SF"
//     means KinClaw's pilot also knows. The lobster family acts as one
//     brain regardless of which binary is the entry point.
//   - learned.md doctrine carries across (per-product, lives at
//     ~/.kinclaw/learned.md as of 2026-05-03 — used to be shared
//     under ~/.localkin/ but moved to product-specific home along
//     with serve-sessions and harvest artifacts).
//   - Single source of truth — one file to back up, debug, migrate.
//
// Why this is OK:
//   - Same machine, same user — there's no logical reason kinclaw's
//     pilot should forget what LocalKin's pilot was just told.
//   - Soul names are namespaced (KinClaw <X> vs <X>) so message
//     buckets don't accidentally merge across products.
//   - Schema changes are coordinated (one developer maintains both).
//
// === Override for isolation ===
//
// Set KINCLAW_DATA_DIR to point at a different directory if you want
// KinClaw's storage isolated from LocalKin (e.g., shipping KinClaw
// alone to users who don't have LocalKin installed, or separating
// "work" and "personal" agent contexts):
//
//   KINCLAW_DATA_DIR=~/.kinclaw kinclaw -soul ...
//   KINCLAW_DATA_DIR=/tmp/kinclaw-test kinclaw ...
//
// Env override only affects the memory.db path; learned.md and
// serve-sessions/ resolve under ~/.kinclaw/ regardless. (To
// override those too, separate envs are a future step.)
func DefaultDBPath() string {
	if dir := os.Getenv("KINCLAW_DATA_DIR"); dir != "" {
		// expand leading ~ since shell doesn't always do it for env
		if strings.HasPrefix(dir, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				dir = filepath.Join(home, dir[2:])
			}
		}
		return filepath.Join(dir, "memory.db")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".localkin", "memory.db")
}

func OpenMemory(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("opening memory db: %w", err)
	}
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		tool_calls TEXT,
		tool_call_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, id);
	CREATE TABLE IF NOT EXISTS memories (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}
	store := &SQLiteStore{db: db}

	// One-time migration: pre-v1.8 session_ids included the kinclaw
	// process PID ("KinClaw Pilot-23799"). v1.8.x changed the format
	// to plain soul name ("KinClaw Pilot") so cross-process history
	// works. This rolls every old PID-suffixed session into the
	// matching soul-name bucket so old conversations stay reachable.
	//
	// Idempotent — once migrated, the regex matches nothing and the
	// query is a no-op. Called once per process at OpenMemory time.
	if migrated, err := store.migratePIDSessions(); err != nil {
		// Log but don't fail — DB still usable, just with fragmented
		// session_ids. Worst case the user sees less history, never
		// data loss.
		fmt.Fprintf(os.Stderr, "Warning: session_id migration error: %v\n", err)
	} else if migrated > 0 {
		fmt.Fprintf(os.Stderr, "\033[2m  memory: migrated %d PID-suffixed session_ids → soul-name buckets\033[0m\n", migrated)
	}

	return store, nil
}

// migratePIDSessions strips trailing "-<digits>" from session_id
// values. Pre-v1.8 each kinclaw process got its own bucket
// "<soul-name>-<pid>"; this collapses them all to "<soul-name>" so
// LoadHistory(<soul-name>) finds them.
//
// Returns the number of distinct old session_ids that were merged.
// Soul names with internal hyphens (like "kin-code-12345") are
// handled correctly because the regex anchors on the LAST "-<digits>"
// at end of string.
func (s *SQLiteStore) migratePIDSessions() (int, error) {
	rows, err := s.db.Query(`SELECT DISTINCT session_id FROM messages`)
	if err != nil {
		return 0, fmt.Errorf("scan session_ids: %w", err)
	}
	type pair struct{ from, to string }
	var renames []pair
	pidSuffix := regexp.MustCompile(`^(.+)-\d+$`)
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err != nil {
			continue
		}
		m := pidSuffix.FindStringSubmatch(sid)
		if len(m) != 2 {
			continue // already a clean soul name, or unexpected shape
		}
		renames = append(renames, pair{from: sid, to: m[1]})
	}
	rows.Close() // explicit — we hold an UPDATE next
	if len(renames) == 0 {
		return 0, nil
	}

	// Use a transaction so partial migration doesn't leave a
	// half-merged DB if the kernel crashes mid-loop.
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	stmt, err := tx.Prepare(`UPDATE messages SET session_id = ? WHERE session_id = ?`)
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("prepare update: %w", err)
	}
	defer stmt.Close()
	for _, r := range renames {
		if _, err := stmt.Exec(r.to, r.from); err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("rename %q→%q: %w", r.from, r.to, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return len(renames), nil
}

func (s *SQLiteStore) SaveMessage(sessionID string, msg brain.Message) error {
	var toolCallsJSON *string
	if len(msg.ToolCalls) > 0 {
		b, _ := json.Marshal(msg.ToolCalls)
		str := string(b)
		toolCallsJSON = &str
	}
	_, err := s.db.Exec(
		`INSERT INTO messages (session_id, role, content, tool_calls, tool_call_id) VALUES (?, ?, ?, ?, ?)`,
		sessionID, msg.Role, msg.Content, toolCallsJSON, msg.ToolCallID,
	)
	return err
}

// maxMsgContentBytes caps individual loaded messages so a single
// 50KB tool output (e.g. yahoo finance dump, AX tree of a giant app)
// from a past session can't blow the entire context window. Picks
// 4KB per message — enough to preserve narrative meaning, lossy on
// fine-grained tool output but those are stale by next session
// anyway. Live current-session messages aren't affected; they go
// straight into history without round-tripping through this cap.
const maxMsgContentBytes = 4096

func truncateForRecall(s string) string {
	if len(s) <= maxMsgContentBytes {
		return s
	}
	return s[:maxMsgContentBytes] + "\n…[truncated; this is a historical message — full content was in the original session]"
}

func (s *SQLiteStore) LoadHistory(sessionID string, limit int) []brain.Message {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT role, content, tool_calls, tool_call_id FROM messages
		 WHERE session_id = ? ORDER BY id DESC LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var messages []brain.Message
	for rows.Next() {
		var msg brain.Message
		var toolCallsJSON, toolCallID sql.NullString
		if err := rows.Scan(&msg.Role, &msg.Content, &toolCallsJSON, &toolCallID); err != nil {
			continue
		}
		// Cap content size — a single oversized historical message
		// shouldn't be allowed to consume the whole context budget.
		msg.Content = truncateForRecall(msg.Content)
		if toolCallsJSON.Valid {
			json.Unmarshal([]byte(toolCallsJSON.String), &msg.ToolCalls)
		}
		if toolCallID.Valid {
			msg.ToolCallID = toolCallID.String
		}
		messages = append(messages, msg)
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages
}

// AllMemories returns every key/value in the memories table, most-
// recently-updated first. Used at session start to dump the user's
// durable facts into the soul's system prompt — so pilot doesn't
// have to remember to call recall just to find out who you are.
//
// Caps at 50 entries to keep the prompt budget reasonable; if the
// table grows beyond that, the oldest entries get filtered out at
// inject time (kernel-side).
func (s *SQLiteStore) AllMemories() ([]struct{ Key, Value string }, error) {
	rows, err := s.db.Query(
		`SELECT key, value FROM memories ORDER BY updated_at DESC LIMIT 50`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct{ Key, Value string }
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		out = append(out, struct{ Key, Value string }{k, v})
	}
	return out, nil
}

func (s *SQLiteStore) Save(key, value string) (string, error) {
	_, err := s.db.Exec(
		`INSERT INTO memories (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now(),
	)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Saved memory: %s", key), nil
}

func (s *SQLiteStore) Recall(query string) (string, error) {
	rows, err := s.db.Query(
		`SELECT key, value FROM memories WHERE key LIKE ? OR value LIKE ? ORDER BY updated_at DESC LIMIT 10`,
		"%"+query+"%", "%"+query+"%",
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var results []string
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		results = append(results, fmt.Sprintf("[%s]: %s", key, value))
	}
	if len(results) == 0 {
		return "No memories found matching: " + query, nil
	}
	return strings.Join(results, "\n"), nil
}

// RecallMessages searches the raw conversation history (messages
// table) for substring matches against `query`. Returns formatted
// excerpts with session_id + role + truncated content, most-recent
// first.
//
// This complements Recall(): Recall is for curated user-facts in
// the memories k-v table (small, hand-saved); RecallMessages is for
// "I remember we discussed X — what was that exactly?" (big,
// uncurated stream of every message ever).
//
// LIKE-based — fast on tens of thousands of rows, doesn't need an
// embedding model. False-positive risk is real (search "lobster"
// matches every Mr. Pinch joke too), so cap results at the limit
// param (caller controls; default 10) to keep the prompt budget
// reasonable. For semantic recall over many false-positive cases,
// future work would add embeddings — see grep-is-all-you-need paper
// for an alternative approach that also scales without a vector DB.
//
// Each excerpt is truncated to 240 chars so a 50KB tool dump
// matched on a single keyword doesn't drown the response.
func (s *SQLiteStore) RecallMessages(query string, limit int) (string, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(
		`SELECT session_id, role, content, created_at FROM messages
		 WHERE content LIKE ?
		 ORDER BY id DESC LIMIT ?`,
		"%"+query+"%", limit,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var results []string
	for rows.Next() {
		var sid, role, content, created string
		if err := rows.Scan(&sid, &role, &content, &created); err != nil {
			continue
		}
		// Truncate per-row so one giant message doesn't blow context.
		const excerptCap = 240
		if len(content) > excerptCap {
			content = content[:excerptCap] + "…"
		}
		// Trim time to date+hour for legibility.
		shortTime := created
		if len(shortTime) >= 16 {
			shortTime = shortTime[:16]
		}
		results = append(results, fmt.Sprintf("[%s · %s · %s] %s", sid, shortTime, role, content))
	}
	if len(results) == 0 {
		return "No messages found matching: " + query, nil
	}
	return strings.Join(results, "\n\n"), nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }
