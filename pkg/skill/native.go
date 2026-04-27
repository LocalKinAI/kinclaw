package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// envDenylist are exact env var names to strip from shell environment.
var envDenylist = map[string]bool{
	"ANTHROPIC_API_KEY": true, "OPENAI_API_KEY": true,
	"AWS_SECRET_ACCESS_KEY": true, "AWS_SESSION_TOKEN": true,
	"GITHUB_TOKEN": true, "GH_TOKEN": true,
	"GOOGLE_API_KEY": true, "AZURE_API_KEY": true,
	"DATABASE_URL": true, "REDIS_URL": true,
}

func SafeEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		k := strings.SplitN(e, "=", 2)[0]
		if envDenylist[k] {
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

// blockedPatterns are regex patterns for dangerous shell commands.
var blockedPatterns = []*regexp.Regexp{
	// Destructive filesystem
	regexp.MustCompile(`rm\s+-[a-z]*r[a-z]*f[a-z]*\s+/`),
	regexp.MustCompile(`mkfs\.`),
	regexp.MustCompile(`dd\s+if=`),
	regexp.MustCompile(`:\(\)\s*\{`), // fork bomb

	// System control
	regexp.MustCompile(`\b(shutdown|reboot|halt|poweroff)\b`),

	// Command obfuscation / injection
	regexp.MustCompile(`\bbash\s+-c\b`),
	regexp.MustCompile(`\beval\s+`),
	regexp.MustCompile(`\|\s*(bash|sh|python[23]?|perl|ruby|node)\b`),
	regexp.MustCompile(`\bcurl\s+.*\|\s*(ba)?sh\b`),
	regexp.MustCompile(`\bwget\s+.*\|\s*(ba)?sh\b`),

	// Reverse shells / data exfiltration
	regexp.MustCompile(`\bnc\s+-[a-z]*e\b`),
	regexp.MustCompile(`/dev/tcp/`),
	regexp.MustCompile(`\bbase64\s+--decode\b`),

	// Sensitive paths
	regexp.MustCompile(`\.(ssh|aws|gnupg)/`),
	regexp.MustCompile(`\.(env|bashrc|zshrc|bash_profile)\b`),
}

func (s *shellSkill) Execute(params map[string]string) (string, error) {
	command := params["command"]
	if command == "" {
		return "", fmt.Errorf("command is required")
	}
	for _, pat := range blockedPatterns {
		if pat.MatchString(command) {
			return "", fmt.Errorf("blocked: dangerous command pattern '%s'", pat.String())
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

func NewFileReadSkill() Skill              { return &fileReadSkill{} }
func (s *fileReadSkill) Name() string      { return "file_read" }
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

func NewFileWriteSkill() Skill              { return &fileWriteSkill{} }
func (s *fileWriteSkill) Name() string      { return "file_write" }
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

func NewFileEditSkill() Skill              { return &fileEditSkill{} }
func (s *fileEditSkill) Name() string      { return "file_edit" }
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

func NewWebFetchSkill() Skill              { return &webFetchSkill{} }
func (s *webFetchSkill) Name() string      { return "web_fetch" }
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
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return false
	}
	if host == "localhost" || host == "127.0.0.1" || host == "0.0.0.0" {
		return true
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return true
		}
		if ip.Equal(net.ParseIP("169.254.169.254")) {
			return true // cloud metadata endpoint
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

type MemoryBackend interface {
	Save(key, value string) (string, error)
	Recall(query string) (string, error)
}

type memorySkill struct{ store MemoryBackend }

func NewMemorySkill(store MemoryBackend) Skill  { return &memorySkill{store: store} }
func (s *memorySkill) Name() string             { return "memory" }
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

func NewForgeSkill(skillsDir string, registry *Registry) Skill { return &forgeSkill{skillsDir: skillsDir, registry: registry} }
func (s *forgeSkill) Name() string                             { return "forge" }
func (s *forgeSkill) Description() string {
	return "Create a new skill by generating a SKILL.md file in skills/<name>/. " +
		"The new skill becomes immediately available next round." +
		"\n\n" +
		"**WHEN to forge**: AFTER you successfully completed a multi-step task that's likely to be " +
		"requested again in a parameterized form. Examples: `calc_compute`, `notes_create`, " +
		"`reminders_add`, `safari_open_url`, `play_song`." +
		"\n\n" +
		"**Choose the SHORTEST execution path** — don't translate the UI-driving you just did into " +
		"a script. Most Apple apps have direct AppleScript / shell APIs that skip the UI entirely:\n" +
		"  - Reminders: `osascript -e 'tell application \"Reminders\" to make new reminder with properties {name:\"$1\"}'`\n" +
		"  - Notes:    `osascript -e 'tell application \"Notes\" to make new note with properties {name:\"$1\", body:\"$2\"}'`\n" +
		"  - Music:    `osascript -e 'tell application \"Music\" to play'`\n" +
		"  - Calc:     `bc <<<\"$1\"` (headless math; no UI needed)\n" +
		"  - Safari:   `osascript -e 'tell application \"Safari\" to open location \"$1\"'`\n" +
		"  - Spotlight: `mdfind \"$1\"`\n" +
		"\n" +
		"Only fall back to UI-driving (input keystrokes, ui click) if the app has no scripting API " +
		"and no relevant CLI." +
		"\n\n" +
		"**HARD rules for the `command` field — kernel REJECTS forge calls that violate these**:\n" +
		"  - command[0] MUST be a real binary in $PATH: `sh`, `bash`, `python3`, `osascript`, `curl`, " +
		"`jq`, `awk`, `mdfind`, `defaults`, `open`, `bc`, etc.\n" +
		"  - command[0] must NEVER be a kinclaw skill name (`ui`, `input`, `screen`, `record`, `shell`, " +
		"`tts`, `stt`, `web_*`, `forge`, `learn`, `app_open_clean`). Those live INSIDE kinclaw — " +
		"there's no `ui` binary in $PATH. Calling them via subprocess always fails silently and " +
		"produces a skill that lies about success forever after.\n" +
		"  - For multi-line shell, use the `[sh, -c, \"<your script>\", \"_\"]` pattern. The trailing " +
		"`\"_\"` is important — it gives the script `$0=_` so user args become `$1, $2, ...` cleanly.\n" +
		"\n" +
		"**A correct minimal `reminders_add`** (no UI, 3 lines):\n" +
		"```yaml\n" +
		"command: [osascript, -e]\n" +
		"args: [\"tell application \\\"Reminders\\\" to make new reminder with properties {name:\\\"{{title}}\\\"}\"]\n" +
		"schema: { title: { type: string, required: true } }\n" +
		"```\n" +
		"Robust because it bypasses the UI entirely — no AX flake, no first-launch modal." +
		"\n\n" +
		"**When NOT to forge**: task isn't actually working yet (forge proven recipes only); " +
		"single-shot non-parameterizable (\"today buy milk\" no, \"add reminder titled X\" yes); " +
		"same name already exists. Kernel pre-flight checks command[0] is in $PATH; if you get " +
		"`forge rejected:` back, fix the recipe and retry."
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

// internalSkillNames is the set of names that must NEVER appear as
// command[0] in a forged SKILL.md. They are kinclaw's own internal
// skills — there's no executable by these names in $PATH, so a forge
// that puts e.g. `command: ["ui", ...]` produces a SKILL.md that
// silently fails on every call. Pre-flight check below rejects it.
var internalSkillNames = map[string]bool{
	"ui": true, "input": true, "screen": true, "record": true,
	"shell": true, "tts": true, "stt": true, "forge": true,
	"learn": true, "memory": true, "clone": true,
	"file_read": true, "file_write": true, "file_edit": true,
	"web_fetch": true, "web_search": true, "web_browse": true,
	"app_open_clean": true,
}

// validateForgeCommand catches the most common LLM mistake when
// forging skills: putting an internal kinclaw skill name as
// command[0] (e.g. `command: ["ui", "action=click", ...]`). Those
// don't exist in $PATH; the resulting SKILL.md would be broken from
// day 1 and silently lie about success forever after.
//
// Also catches typos that don't resolve to anything in $PATH so the
// agent gets a clear "command not found" message instead of shipping
// a broken skill.
func validateForgeCommand(cmdParts []string) error {
	if len(cmdParts) == 0 {
		return fmt.Errorf("command must not be empty")
	}
	cmd0 := cmdParts[0]
	if internalSkillNames[cmd0] {
		return fmt.Errorf(
			"command[0] = %q is a kinclaw internal skill, not a shell binary. "+
				"Forged skills run in a subprocess and can't call kinclaw skills directly. "+
				"Use real OS tools instead: `osascript` for AppleScript, `sh -c` for shell, "+
				"`python3` for scripts, `curl` for HTTP, `mdfind` for Spotlight, etc.",
			cmd0)
	}
	if _, err := exec.LookPath(cmd0); err != nil {
		return fmt.Errorf(
			"command[0] = %q not found in $PATH. Use a real executable "+
				"(sh, bash, python3, osascript, curl, jq, awk, mdfind, open, bc, ...).",
			cmd0)
	}
	return nil
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
	// Pre-flight: refuse to write a SKILL.md whose command can't run.
	if err := validateForgeCommand(cmdParts); err != nil {
		// Clean up the empty dir so we don't leave a half-state turd.
		os.Remove(dir)
		return "", fmt.Errorf("forge rejected: %w", err)
	}
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
