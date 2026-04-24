package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LocalKinAI/kinclaw/pkg/soul"
	"gopkg.in/yaml.v3"
)

type SkillMeta struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Command     []string          `yaml:"command"`
	Args        []string          `yaml:"args"`
	Schema      map[string]Schema `yaml:"schema"`
	Timeout     int               `yaml:"timeout"`
}

type Schema struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

type ExternalSkill struct {
	meta    SkillMeta
	dir     string
	toolDef json.RawMessage
}

func LoadExternalSkill(path string) (*ExternalSkill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rawYAML, _, err := soul.SplitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("parsing SKILL.md %s: %w", path, err)
	}
	var meta SkillMeta
	if err := yaml.Unmarshal(rawYAML, &meta); err != nil {
		return nil, fmt.Errorf("parsing SKILL.md YAML: %w", err)
	}
	if meta.Name == "" || meta.Description == "" || len(meta.Command) == 0 {
		return nil, fmt.Errorf("SKILL.md must have name, description, and command")
	}
	if meta.Timeout <= 0 {
		meta.Timeout = 30
	}
	props := make(map[string]map[string]string)
	var required []string
	for k, v := range meta.Schema {
		props[k] = map[string]string{"type": v.Type, "description": v.Description}
		if v.Required {
			required = append(required, k)
		}
	}
	toolDef := MakeToolDef(meta.Name, meta.Description, props, required)
	return &ExternalSkill{meta: meta, dir: filepath.Dir(path), toolDef: toolDef}, nil
}

func (s *ExternalSkill) Name() string            { return s.meta.Name }
func (s *ExternalSkill) Description() string      { return s.meta.Description }
func (s *ExternalSkill) ToolDef() json.RawMessage { return s.toolDef }

func (s *ExternalSkill) Execute(params map[string]string) (string, error) {
	args := make([]string, len(s.meta.Args))
	for i, a := range s.meta.Args {
		resolved := a
		for k, v := range params {
			resolved = strings.ReplaceAll(resolved, "{{"+k+"}}", v)
		}
		args[i] = resolved
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.meta.Timeout)*time.Second)
	defer cancel()
	cmdArgs := append(s.meta.Command[1:], args...)
	cmd := exec.CommandContext(ctx, s.meta.Command[0], cmdArgs...)
	cmd.Dir = s.dir
	cmd.Env = append(SafeEnv(), "SKILL_DIR="+s.dir)
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

func LoadExternalSkills(dir string) ([]*ExternalSkill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var skills []*ExternalSkill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			continue
		}
		ext, err := LoadExternalSkill(skillPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [warning: skipping %s: %v]\n", skillPath, err)
			continue
		}
		skills = append(skills, ext)
	}
	return skills, nil
}
