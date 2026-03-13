package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuneDisplayWidth(t *testing.T) {
	tests := []struct {
		r    rune
		want int
	}{
		{'a', 1}, {'Z', 1}, {'1', 1}, {' ', 1},
		{'中', 2}, {'文', 2}, {'猫', 2},
		{'あ', 2}, {'ア', 2}, {'한', 2},
		{'é', 1}, {'ñ', 1},
	}
	for _, tt := range tests {
		if got := runeDisplayWidth(tt.r); got != tt.want {
			t.Errorf("runeDisplayWidth(%q U+%04X) = %d, want %d", tt.r, tt.r, got, tt.want)
		}
	}
}

func TestHistoryNavigation(t *testing.T) {
	cmdHistory = nil
	cmdHistFile = ""
	appendHistory("hello")
	appendHistory("world")
	appendHistory("你好")
	if len(cmdHistory) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(cmdHistory))
	}
	if cmdHistory[0] != "hello" || cmdHistory[1] != "world" || cmdHistory[2] != "你好" {
		t.Errorf("unexpected history: %v", cmdHistory)
	}
}

func TestHistoryDedup(t *testing.T) {
	cmdHistory = nil
	cmdHistFile = ""
	appendHistory("ls")
	appendHistory("ls")
	appendHistory("ls")
	appendHistory("pwd")
	appendHistory("pwd")
	if len(cmdHistory) != 2 {
		t.Fatalf("expected 2 entries after dedup, got %d", len(cmdHistory))
	}
	if cmdHistory[0] != "ls" || cmdHistory[1] != "pwd" {
		t.Errorf("unexpected history: %v", cmdHistory)
	}
}

func TestHistoryEmpty(t *testing.T) {
	cmdHistory = nil
	cmdHistFile = ""
	appendHistory("")
	appendHistory("")
	if len(cmdHistory) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(cmdHistory))
	}
}

func TestHistoryMax(t *testing.T) {
	cmdHistory = nil
	cmdHistFile = ""
	cmdHistoryMax = 10
	defer func() { cmdHistoryMax = 500 }()
	for i := 0; i < 20; i++ {
		appendHistory(strings.Repeat("x", i+1))
	}
	if len(cmdHistory) != 10 {
		t.Fatalf("expected 10 entries after trim, got %d", len(cmdHistory))
	}
	if cmdHistory[0] != strings.Repeat("x", 11) {
		t.Errorf("oldest entry should be 11 x's, got %q", cmdHistory[0])
	}
}

func TestHistoryPersistence(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "test_history")
	cmdHistory = nil
	cmdHistFile = histPath
	appendHistory("first")
	appendHistory("second")
	appendHistory("你好世界")
	data, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatalf("failed to read history file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines in file, got %d: %v", len(lines), lines)
	}
	cmdHistory = nil
	InitHistory(histPath)
	if len(cmdHistory) != 3 {
		t.Fatalf("expected 3 entries after reload, got %d", len(cmdHistory))
	}
	if cmdHistory[2] != "你好世界" {
		t.Errorf("CJK entry lost after reload: %q", cmdHistory[2])
	}
}

func TestHistoryLoadMissing(t *testing.T) {
	result := loadHistoryFile("/tmp/nonexistent_localkin_history_test_file")
	if result != nil {
		t.Errorf("expected nil for missing file, got %v", result)
	}
}

func TestUtf8ByteLen(t *testing.T) {
	tests := []struct {
		b    byte
		want int
	}{
		{0x41, 1}, {0xC3, 2}, {0xE4, 3}, {0xF0, 4},
	}
	for _, tt := range tests {
		if got := utf8ByteLen(tt.b); got != tt.want {
			t.Errorf("utf8ByteLen(0x%02X) = %d, want %d", tt.b, got, tt.want)
		}
	}
}
