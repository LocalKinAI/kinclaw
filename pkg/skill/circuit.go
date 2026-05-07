package skill

import "fmt"

// CircuitBreaker detects when a skill is going nowhere and forces human
// escalation instead of letting the LLM burn through all tool rounds.
// Three triggers:
//  1. Same skill + same error 3 consecutive times (tight error loop)
//  2. Same skill fails 3 times total, regardless of error or intervening
//     successes from other skills (catches forge↔broken_skill cycles)
//  3. Same skill returns the same successful output 3 consecutive times
//     (no-progress loop — e.g. `ui find` returning "no elements matching"
//     three times in a row, or `ui focused_app` returning Terminal three
//     times after osascript activate calls).
//
// **Removed in v1.12.1**: a fourth trigger that capped per-skill calls
// at 8 per turn. It was designed for ui-driven Pilot tasks where 8+
// invocations of `ui find` / `ui tree` indicate "verifying-without-
// progress" — but it actively harmed throughput tasks where 8+ calls
// are healthy work (researcher running knowledge_search across 8
// masters / web_search across 8 different queries / pubmed_search
// across 8 specialty journals). The other three triggers already
// catch the genuinely-stuck cases: same error → Trigger 1, same
// output → Trigger 3, repeated failures → Trigger 2. A throughput
// loop where every call returns DIFFERENT material trips none of
// the three remaining triggers because nothing's actually stuck —
// which is the right behavior.
//
// Triggers are intentionally generic. They don't know what the LLM is
// trying to do; they just notice that the world isn't changing in
// response to the agent's actions and ask the LLM to replan. False
// positives are possible but the kernel only emits a [SYSTEM] hint —
// it doesn't block the next call — so the LLM can ignore when warranted.
//
// Create one per chat session (not global).
type CircuitBreaker struct {
	// Error-loop tracking.
	lastSkill string
	lastError string
	consec    int
	failures  map[string]int

	// No-progress tracking. We compare a snippet of the successful
	// output against the previous one from the same skill; identical
	// snippets in a row are the loop signal.
	lastOutSkill string
	lastOutSnip  string
	consecOut    int
}

// NewCircuitBreaker returns a fresh circuit breaker for a chat session.
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		failures: make(map[string]int),
	}
}

const (
	cbThreshold = 3
	// Output snippet length used for "same result repeating" detection.
	// Long enough to disambiguate near-identical responses, short enough
	// to keep tree dumps from being treated as different on whitespace.
	cbOutputSnippet = 200
)

// Record inspects a batch of tool results and returns tripped=true with
// an escalation message if any of the three triggers fires.
func (cb *CircuitBreaker) Record(results []ToolResult) (tripped bool, msg string) {
	for _, r := range results {
		if r.Err != nil {
			// Error path — feeds triggers 1 and 2, resets trigger 3.
			cb.failures[r.Name]++
			errStr := r.Err.Error()
			if r.Name == cb.lastSkill && errStr == cb.lastError {
				cb.consec++
			} else {
				cb.lastSkill = r.Name
				cb.lastError = errStr
				cb.consec = 1
			}
			cb.lastOutSkill = ""
			cb.lastOutSnip = ""
			cb.consecOut = 0
			if cb.failures[r.Name] >= cbThreshold {
				msg = fmt.Sprintf(
					"[SYSTEM] Skill %q has failed %d times in this session.\nSTOP retrying THIS specific skill — but the task is NOT over. Pivot to an alternative path: a fallback skill suggested by the failing skill's error message, a different tool entirely, or different params. Your soul's protocol may have a documented fallback chain (e.g. web_search → web_scrape → web_fetch). Only ask the user for guidance if you've exhausted alternatives, not as a first response to this notice.",
					r.Name, cb.failures[r.Name],
				)
				delete(cb.failures, r.Name)
				cb.lastSkill = ""
				cb.lastError = ""
				cb.consec = 0
				return true, msg
			}
			if cb.consec >= cbThreshold {
				msg = fmt.Sprintf(
					"[SYSTEM] Skill %q has failed %d consecutive times with the same error:\n  %s\nSTOP retrying THIS approach — but the task is NOT over. The error message above usually contains a fallback hint; if not, pivot to a different skill or different params. Only ask the user for guidance after trying an alternative.",
					cb.lastSkill, cb.consec, cb.lastError,
				)
				delete(cb.failures, cb.lastSkill)
				cb.lastSkill = ""
				cb.lastError = ""
				cb.consec = 0
				return true, msg
			}
			continue
		}

		// Success path — resets error trackers, feeds trigger 3.
		cb.lastSkill = ""
		cb.lastError = ""
		cb.consec = 0

		snip := r.Output
		if len(snip) > cbOutputSnippet {
			snip = snip[:cbOutputSnippet]
		}
		if r.Name == cb.lastOutSkill && snip == cb.lastOutSnip {
			cb.consecOut++
		} else {
			cb.lastOutSkill = r.Name
			cb.lastOutSnip = snip
			cb.consecOut = 1
		}
		if cb.consecOut >= cbThreshold {
			msg = fmt.Sprintf(
				"[SYSTEM] Skill %q has returned the same result %d times in a row — looks like a no-progress loop. STOP repeating this exact call — but the task is NOT over. Replan: try a different matcher, different params, or a different skill entirely. Ask the user only after trying an alternative. Last result snippet:\n  %s",
				r.Name, cb.consecOut, snip,
			)
			cb.lastOutSkill = ""
			cb.lastOutSnip = ""
			cb.consecOut = 0
			return true, msg
		}
	}
	return false, ""
}
