package skill

import "fmt"

// CircuitBreaker detects when a skill is going nowhere and forces human
// escalation instead of letting the LLM burn through all tool rounds.
// Four triggers:
//  1. Same skill + same error 3 consecutive times (tight error loop)
//  2. Same skill fails 3 times total, regardless of error or intervening
//     successes from other skills (catches forge↔broken_skill cycles)
//  3. Same skill returns the same successful output 3 consecutive times
//     (no-progress loop — e.g. `ui find` returning "no elements matching"
//     three times in a row, or `ui focused_app` returning Terminal three
//     times after osascript activate calls).
//  4. Any single skill called more than `cbUsageMax` times this turn
//     (over-iteration — the agent is stuck "verifying" or "fixing" in a
//     way that's not making progress; healthy demos use ui 3-4 times,
//     not 8+).
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

	// Total calls per skill this turn — catches over-iteration even
	// when each call has different params/output (the LLM bouncing
	// between ui tree → ui find → ui click → ui read trying to "fix"
	// a verification ambiguity).
	usage map[string]int
}

// NewCircuitBreaker returns a fresh circuit breaker for a chat session.
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		failures: make(map[string]int),
		usage:    make(map[string]int),
	}
}

const (
	cbThreshold = 3
	// Output snippet length used for "same result repeating" detection.
	// Long enough to disambiguate near-identical responses, short enough
	// to keep tree dumps from being treated as different on whitespace.
	cbOutputSnippet = 200
	// Per-turn call cap per skill. A healthy demo uses ui 3-4 times
	// (tree + click_sequence + occasional read). 8+ means the agent
	// is grinding on verification or trying to "fix" something that
	// isn't broken from the kernel's perspective.
	cbUsageMax = 8
)

// Record inspects a batch of tool results and returns tripped=true with
// an escalation message if any of the four triggers fires.
func (cb *CircuitBreaker) Record(results []ToolResult) (tripped bool, msg string) {
	for _, r := range results {
		// Trigger 4 — total per-skill usage this turn. Counted before
		// branching on Err so both successful and failing calls add up.
		cb.usage[r.Name]++
		if cb.usage[r.Name] >= cbUsageMax {
			msg = fmt.Sprintf(
				"[SYSTEM] Skill %q has been called %d times this turn — enough. STOP calling THIS skill, but the user's task is NOT done.\n\n"+
					"What to do RIGHT NOW (in this order):\n"+
					"  1. If you've already gathered enough material to answer / write a report → SYNTHESIZE NOW. For research tasks: file_write the markdown report + reply to the user with the path + TL;DR. For action tasks: do the next concrete step that delivers the result.\n"+
					"  2. If you need more info but THIS skill keeps failing → switch to a DIFFERENT skill or different params (your soul's protocol probably has a fallback chain).\n"+
					"  3. Only ask the user for guidance as a LAST resort, after trying (1) or (2).\n\n"+
					"DO NOT just reply with a status message ('I'm working on it...', 'Let me think about this...'). The user is waiting for the deliverable. Produce the deliverable now using what you've gathered.",
				r.Name, cb.usage[r.Name],
			)
			cb.usage[r.Name] = 0
			return true, msg
		}

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
