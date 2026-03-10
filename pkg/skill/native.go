package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SafeEnv returns environment variables with secrets filtered out.
func SafeEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		k := strings.ToUpper(strings.SplitN(e, "=", 2)[0])
		if strings.Contains(k, "KEY") || strings.Contains(k, "SECRET") ||
			strings.Contains(k, "TOKEN") || strings.Contains(k, "PASSWORD") {
			continue
		}
		env = append(env, e)
	}
	return env
}

type shellSkill struct{ timeout time.Duration }

func NewShellSkill(timeoutSec int) Skill {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &shellSkill{timeout: time.Duration(timeoutSec) * time.Second}
}

func (s *shellSkill) Name() string { return "shell" }
func (s *shellSkill) Description() string {
	return "Execute a shell command and return stdout+stderr (max 128KB). Use for: running tests, git, grep/rg, build commands. Prefer file_read over cat. Prefer file_edit over sed. Dangerous commands are blocked."
}
func (s *shellSkill) ToolDef() json.RawMessage {
	return MakeToolDef("shell", s.Description(),
		map[string]map[string]string{
			"command": {"type": "string", "description": "Shell command to execute. Use && to chain commands."},
		}, []string{"command"})
}

var shellBlocklist = []string{
	"rm -rf /", "mkfs.", "dd if=", ":(){ :|:&",
	"shutdown", "reboot", "halt",
}
var pipeBlocklist = []string{"bash", "sh", "python", "perl", "ruby"}

func (s *shellSkill) Execute(params map[string]string) (string, error) {
	command := params["command"]
	if command == "" {
		return "", fmt.Errorf("command is required")
	}
	lower := strings.ToLower(command)
	for _, pat := range shellBlocklist {
		if strings.Contains(lower, pat) {
			return "", fmt.Errorf("blocked: dangerous command pattern '%s'", pat)
		}
	}
	if idx := strings.LastIndex(lower, "|"); idx >= 0 {
		after := strings.TrimSpace(lower[idx+1:])
		for _, interp := range pipeBlocklist {
			if after == interp || strings.HasPrefix(after, interp+" ") {
				return "", fmt.Errorf("blocked: piping to %s is not allowed", interp)
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Env = SafeEnv()
	out, err := cmd.CombinedOutput()
	const maxOutput = 128 * 1024
	result := string(out)
	if len(result) > maxOutput {
		result = result[:maxOutput] + "\n... (truncated)"
	}
	if err != nil {
		return result + "\nError: " + err.Error(), nil
	}
	return result, nil
}

type fileReadSkill struct{}

func NewFileReadSkill() Skill { return &fileReadSkill{} }

func (s *fileReadSkill) Name() string { return "file_read" }
func (s *fileReadSkill) Description() string {
	return "Read file contents (max 64KB). Always read a file before editing it."
}
func (s *fileReadSkill) ToolDef() json.RawMessage {
	return MakeToolDef("file_read", s.Description(),
		map[string]map[string]string{
			"path": {"type": "string", "description": "File path to read"},
		}, []string{"path"})
}

func (s *fileReadSkill) Execute(params map[string]string) (string, error) {
	path := params["path"]
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	const maxSize = 64 * 1024
	content := string(data)
	if len(content) > maxSize {
		content = content[:maxSize] + "\n... (truncated, file too large)"
	}
	return content, nil
}

type fileWriteSkill struct{}

func NewFileWriteSkill() Skill { return &fileWriteSkill{} }

func (s *fileWriteSkill) Name() string { return "file_write" }
func (s *fileWriteSkill) Description() string {
	return "Write content to a file. OVERWRITES entire file. For partial edits use file_edit instead."
}
func (s *fileWriteSkill) ToolDef() json.RawMessage {
	return MakeToolDef("file_write", s.Description(),
		map[string]map[string]string{
			"path":    {"type": "string", "description": "File path. Parent dirs created automatically."},
			"content": {"type": "string", "description": "Complete file content (replaces everything)."},
		}, []string{"path", "content"})
}

func (s *fileWriteSkill) Execute(params map[string]string) (string, error) {
	path := params["path"]
	content := params["content"]
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Written %d bytes to %s", len(content), abs), nil
}

type fileEditSkill struct{}

func NewFileEditSkill() Skill { return &fileEditSkill{} }

func (s *fileEditSkill) Name() string { return "file_edit" }
func (s *fileEditSkill) Description() string {
	return "Edit a file by replacing exact text. The old_text must appear exactly once. Read the file first to get the exact text."
}
func (s *fileEditSkill) ToolDef() json.RawMessage {
	return MakeToolDef("file_edit", s.Description(),
		map[string]map[string]string{
			"path":     {"type": "string", "description": "File path to edit"},
			"old_text": {"type": "string", "description": "Exact text to find (must be unique in file)"},
			"new_text": {"type": "string", "description": "Replacement text"},
		}, []string{"path", "old_text", "new_text"})
}

func (s *fileEditSkill) Execute(params map[string]string) (string, error) {
	path, oldText, newText := params["path"], params["old_text"], params["new_text"]
	if path == "" || oldText == "" {
		return "", fmt.Errorf("path and old_text are required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	content := string(data)
	count := strings.Count(content, oldText)
	if count == 0 {
		return "", fmt.Errorf("old_text not found in file")
	}
	if count > 1 {
		return "", fmt.Errorf("old_text found %d times, must be unique (provide more context)", count)
	}
	content = strings.Replace(content, oldText, newText, 1)
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Edited %s: replaced 1 occurrence", abs), nil
}

type webFetchSkill struct{}

func NewWebFetchSkill() Skill { return &webFetchSkill{} }

func (s *webFetchSkill) Name() string        { return "web_fetch" }
func (s *webFetchSkill) Description() string  { return "Fetch a URL and return its content as readable text" }
func (s *webFetchSkill) ToolDef() json.RawMessage {
	return MakeToolDef("web_fetch", s.Description(),
		map[string]map[string]string{
			"url": {"type": "string", "description": "The URL to fetch"},
		}, []string{"url"})
}

func (s *webFetchSkill) Execute(params map[string]string) (string, error) {
	rawURL := params["url"]
	if rawURL == "" {
		return "", fmt.Errorf("url is required")
	}
	if isPrivateURL(rawURL) {
		return "", fmt.Errorf("blocked: cannot fetch private/internal URLs")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "LocalKin/1.0 (AI Agent Runtime)")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	if err != nil {
		return "", err
	}
	contentType := resp.Header.Get("Content-Type")
	content := string(body)
	if strings.Contains(contentType, "text/html") {
		content = htmlToText(content)
	}
	const maxOutput = 32 * 1024
	if len(content) > maxOutput {
		content = content[:maxOutput] + "\n... (truncated)"
	}
	return "---BEGIN UNTRUSTED WEB CONTENT---\n" + content + "\n---END UNTRUSTED WEB CONTENT---", nil
}

func isPrivateURL(rawURL string) bool {
	host := rawURL
	for _, sep := range []string{"://", "/", ":"} {
		if i := strings.Index(host, sep); i >= 0 {
			if sep == "://" {
				host = host[i+3:]
			} else {
				host = host[:i]
			}
		}
	}
	if host == "localhost" || host == "127.0.0.1" || host == "0.0.0.0" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
			ip = ips[0]
		}
	}
	if ip == nil {
		return false
	}
	for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16"} {
		if _, n, _ := net.ParseCIDR(cidr); n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}

func htmlToText(s string) string {
	for _, tag := range []string{"script", "style"} {
		for {
			lo := strings.ToLower(s)
			i := strings.Index(lo, "<"+tag)
			if i < 0 {
				break
			}
			j := strings.Index(lo[i:], "</"+tag+">")
			if j < 0 {
				break
			}
			s = s[:i] + s[i+j+len("</"+tag+">"):]
		}
	}
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			sb.WriteByte(' ')
		case !inTag:
			sb.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

// MemoryBackend is the interface for the memory skill's storage.
type MemoryBackend interface {
	Save(key, value string) (string, error)
	Recall(query string) (string, error)
}

type memorySkill struct{ store MemoryBackend }

func NewMemorySkill(store MemoryBackend) Skill { return &memorySkill{store: store} }
func (s *memorySkill) Name() string            { return "memory" }
func (s *memorySkill) Description() string {
	return "Save or recall persistent memories. Use action=save to store key-value pairs, action=recall to search memories by query."
}
func (s *memorySkill) ToolDef() json.RawMessage {
	return MakeToolDef("memory", s.Description(),
		map[string]map[string]string{
			"action": {"type": "string", "description": "save or recall"},
			"key":    {"type": "string", "description": "Memory key (for save)"},
			"value":  {"type": "string", "description": "Memory value (for save)"},
			"query":  {"type": "string", "description": "Search query (for recall)"},
		}, []string{"action"})
}

func (s *memorySkill) Execute(params map[string]string) (string, error) {
	switch params["action"] {
	case "save":
		if params["key"] == "" || params["value"] == "" {
			return "", fmt.Errorf("key and value are required for save")
		}
		return s.store.Save(params["key"], params["value"])
	case "recall":
		if params["query"] == "" {
			return "", fmt.Errorf("query is required for recall")
		}
		return s.store.Recall(params["query"])
	default:
		return "", fmt.Errorf("action must be 'save' or 'recall'")
	}
}

type forgeSkill struct {
	skillsDir string
	registry  *Registry
}

func NewForgeSkill(skillsDir string, registry *Registry) Skill {
	return &forgeSkill{skillsDir: skillsDir, registry: registry}
}

func (s *forgeSkill) Name() string { return "forge" }
func (s *forgeSkill) Description() string {
	return "Create a new skill by generating a SKILL.md file. The new skill becomes immediately available."
}
func (s *forgeSkill) ToolDef() json.RawMessage {
	return MakeToolDef("forge", s.Description(),
		map[string]map[string]string{
			"name":           {"type": "string", "description": "Skill name (alphanumeric and underscores)"},
			"description":    {"type": "string", "description": "What the skill does"},
			"command":        {"type": "string", "description": "Command to run (e.g. python3 script.py)"},
			"args":           {"type": "string", "description": "JSON array of argument templates (e.g. [\"{{prompt}}\"])"},
			"schema":         {"type": "string", "description": "JSON object of parameter definitions"},
			"script_content": {"type": "string", "description": "Content of the script file to create"},
		}, []string{"name", "description", "command"})
}

func (s *forgeSkill) Execute(params map[string]string) (string, error) {
	name := params["name"]
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	dir := filepath.Join(s.skillsDir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", name))
	sb.WriteString(fmt.Sprintf("description: %s\n", params["description"]))
	cmdParts := strings.Fields(params["command"])
	sb.WriteString("command: [")
	for i, p := range cmdParts {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%q", p))
	}
	sb.WriteString("]\n")
	if args := params["args"]; args != "" {
		sb.WriteString(fmt.Sprintf("args: %s\n", args))
	}
	if schema := params["schema"]; schema != "" {
		var schemaMap map[string]interface{}
		if err := json.Unmarshal([]byte(schema), &schemaMap); err == nil {
			sb.WriteString("schema:\n")
			for k, v := range schemaMap {
				if m, ok := v.(map[string]interface{}); ok {
					sb.WriteString(fmt.Sprintf("  %s:\n", k))
					for sk, sv := range m {
						sb.WriteString(fmt.Sprintf("    %s: %v\n", sk, sv))
					}
				}
			}
		}
	}
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("# %s\n\nForged by LocalKin agent.\n", name))
	skillPath := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(sb.String()), 0644); err != nil {
		return "", err
	}
	if script := params["script_content"]; script != "" {
		scriptName := "run.py"
		if len(cmdParts) > 1 {
			scriptName = cmdParts[len(cmdParts)-1]
		}
		scriptPath := filepath.Join(dir, scriptName)
		if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
			return "", err
		}
	}
	ext, err := LoadExternalSkill(skillPath)
	if err == nil {
		s.registry.Register(ext)
	}
	return fmt.Sprintf("Forged skill '%s' at %s", name, skillPath), nil
}
