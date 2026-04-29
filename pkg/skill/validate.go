package skill

import "fmt"

// ValidateSkillMeta runs the forge quality gate v2 checks against parsed
// SkillMeta. Returns nil if the skill passes, otherwise returns the first
// failure the gate finds.
//
// Used by:
//   - the forge skill itself (after JSON→struct round-trip — see Execute
//     in native.go)
//   - the harvest pipeline (third-party SKILL.md ingestion)
//
// The four rules applied here (the v1.2.1 forge gate v2 set, minus #1
// "JSON args round-trip" which is forge-internal):
//
//  1. Name matches identifier pattern (no hyphens / spaces / unicode).
//  2. command[0] resolves in $PATH and isn't an internal kinclaw skill name.
//  3. osascript -e flags pair with a script string.
//  4. No hardcoded screen coordinates ("click at {N, M}").
//  5. Every {{var}} in args is declared in the schema.
//
// Pure check — does NOT touch the filesystem, does NOT execute anything.
// Caller decides what to do with a returned error (forge rejects the
// write; harvest moves the candidate to a "rejected" pile + logs why).
func ValidateSkillMeta(meta SkillMeta) error {
	if !forgeNamePattern.MatchString(meta.Name) {
		return fmt.Errorf("name %q must match %s", meta.Name, forgeNamePattern.String())
	}
	if err := validateForgeCommand(meta.Command); err != nil {
		return err
	}
	// validateForgeArgs only checks key presence in schema — value type
	// doesn't matter, so we lift our typed schema map into the loose
	// map[string]interface{} shape it expects.
	schemaMap := make(map[string]interface{}, len(meta.Schema))
	for k := range meta.Schema {
		schemaMap[k] = nil
	}
	return validateForgeArgs(meta.Command, meta.Args, schemaMap)
}
