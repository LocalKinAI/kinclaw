package localkin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type MemoryStore interface {
	SaveMessage(sessionID string, msg Message) error
	LoadHistory(sessionID string, limit int) []Message
	Save(key, value string) (string, error)
	Recall(query string) (string, error)
	Close() error
}

type SQLiteStore struct {
	db *sql.DB
}

func DefaultDBPath() string {
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
	);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) SaveMessage(sessionID string, msg Message) error {
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

func (s *SQLiteStore) LoadHistory(sessionID string, limit int) []Message {
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

	var messages []Message
	for rows.Next() {
		var msg Message
		var toolCallsJSON, toolCallID sql.NullString
		if err := rows.Scan(&msg.Role, &msg.Content, &toolCallsJSON, &toolCallID); err != nil {
			continue
		}
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

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
