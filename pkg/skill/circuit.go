package skill

import "fmt"

// CircuitBreaker detects when a skill keeps failing and forces human
// escalation instead of letting the LLM burn through all tool rounds.
// Two triggers:
//  1. Same skill + same error 3 consecutive times (tight loop)
//  2. Same skill fails 3 times total, regardless of error or intervening
//     successes from other skills (catches forge↔broken_skill cycles)
//
// Create one per chat session (not global).
type CircuitBreaker struct {
	lastSkill string
	lastError string
	consec    int
	failures  map[string]int
}

// NewCircuitBreaker returns a fresh circuit breaker for a chat session.
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{failures: make(map[string]int)}
}

const cbThreshold = 3

// Record inspects a batch of tool results and returns tripped=true with
// an escalation message if either trigger fires.
func (cb *CircuitBreaker) Record(results []ToolResult) (tripped bool, msg string) {
	for _, r := range results {
		if r.Err != nil {
			cb.failures[r.Name]++
			errStr := r.Err.Error()
			if r.Name == cb.lastSkill && errStr == cb.lastError {
				cb.consec++
			} else {
				cb.lastSkill = r.Name
				cb.lastError = errStr
				cb.consec = 1
			}
			if cb.failures[r.Name] >= cbThreshold {
				msg = fmt.Sprintf(
					"[SYSTEM] Skill %q has failed %d times in this session.\nStop retrying this skill. Explain the problem to the user and ask for guidance.",
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
					"[SYSTEM] Skill %q has failed %d consecutive times with the same error:\n  %s\nStop retrying this approach. Explain the problem to the user and ask for guidance.",
					cb.lastSkill, cb.consec, cb.lastError,
				)
				delete(cb.failures, cb.lastSkill)
				cb.lastSkill = ""
				cb.lastError = ""
				cb.consec = 0
				return true, msg
			}
		} else {
			cb.lastSkill = ""
			cb.lastError = ""
			cb.consec = 0
		}
	}
	return false, ""
}
