package skill

import (
	"fmt"
	"strings"
	"testing"
)

func TestCircuitBreaker_NoTrip(t *testing.T) {
	cb := NewCircuitBreaker()
	tripped, _ := cb.Record([]ToolResult{
		{Name: "shell", Output: "", Err: fmt.Errorf("fail 1")},
		{Name: "shell", Output: "", Err: fmt.Errorf("fail 2")},
	})
	if tripped {
		t.Fatal("should not trip after 2 failures")
	}
}

func TestCircuitBreaker_ConsecutiveTrip(t *testing.T) {
	cb := NewCircuitBreaker()
	err := fmt.Errorf("same error")
	for i := 0; i < 2; i++ {
		tripped, _ := cb.Record([]ToolResult{{Name: "shell", Err: err}})
		if tripped {
			t.Fatalf("tripped early at round %d", i)
		}
	}
	tripped, msg := cb.Record([]ToolResult{{Name: "shell", Err: err}})
	if !tripped {
		t.Fatal("should trip after 3 consecutive same-error failures")
	}
	if msg == "" {
		t.Fatal("message should not be empty")
	}
}

func TestCircuitBreaker_CumulativeTrip(t *testing.T) {
	cb := NewCircuitBreaker()
	for i := 0; i < 2; i++ {
		cb.Record([]ToolResult{{Name: "forge", Err: fmt.Errorf("error %d", i)}})
	}
	tripped, msg := cb.Record([]ToolResult{{Name: "forge", Err: fmt.Errorf("error 3")}})
	if !tripped {
		t.Fatal("should trip after 3 cumulative failures")
	}
	if msg == "" {
		t.Fatal("message should not be empty")
	}
}

func TestCircuitBreaker_SuccessResetsConsecutive(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.Record([]ToolResult{{Name: "shell", Err: fmt.Errorf("err")}})
	// Success resets consecutive counter
	cb.Record([]ToolResult{{Name: "shell", Output: "ok"}})
	// After success, consecutive restarts at 0
	cb.Record([]ToolResult{{Name: "shell", Err: fmt.Errorf("err")}})
	tripped, _ := cb.Record([]ToolResult{{Name: "shell", Err: fmt.Errorf("err")}})
	// Cumulative is now 3 (1 before success + 2 after), so it SHOULD trip
	if !tripped {
		t.Fatal("cumulative count should persist across successes and trip at 3")
	}
}

func TestCircuitBreaker_DifferentSkills(t *testing.T) {
	cb := NewCircuitBreaker()
	err := fmt.Errorf("fail")
	cb.Record([]ToolResult{{Name: "shell", Err: err}})
	cb.Record([]ToolResult{{Name: "forge", Err: err}})
	cb.Record([]ToolResult{{Name: "file_read", Err: err}})
	// None should trip — different skills each time
	tripped, _ := cb.Record([]ToolResult{{Name: "web_fetch", Err: err}})
	if tripped {
		t.Fatal("different skills should not trip circuit breaker")
	}
}

func TestCircuitBreaker_ResetAfterTrip(t *testing.T) {
	cb := NewCircuitBreaker()
	err := fmt.Errorf("fail")
	for i := 0; i < 3; i++ {
		cb.Record([]ToolResult{{Name: "shell", Err: err}})
	}
	// After trip, same skill should be able to fail again without immediate trip
	tripped, _ := cb.Record([]ToolResult{{Name: "shell", Err: err}})
	if tripped {
		t.Fatal("should reset after tripping")
	}
}

// TestCircuitBreaker_SameOutputLoopTrips covers trigger 3: the skill
// keeps succeeding but returning the same result, signalling the agent
// is in a no-progress loop. Classic case: `ui find` returning "no
// elements matching X" three times in a row when the LLM keeps trying
// the same matcher.
func TestCircuitBreaker_SameOutputLoopTrips(t *testing.T) {
	cb := NewCircuitBreaker()
	r := ToolResult{Name: "ui", Output: "no elements matching role=AXButton title=\"+\""}
	for i := 0; i < 2; i++ {
		tripped, _ := cb.Record([]ToolResult{r})
		if tripped {
			t.Fatalf("trip on call %d, expected to wait until threshold", i+1)
		}
	}
	tripped, msg := cb.Record([]ToolResult{r})
	if !tripped {
		t.Fatal("expected trip on 3rd identical successful result")
	}
	if !strings.Contains(msg, "no-progress loop") {
		t.Errorf("expected message to mention 'no-progress loop', got: %s", msg)
	}
}

// TestCircuitBreaker_DifferentOutputResets — a different output between
// matching ones must break the streak.
func TestCircuitBreaker_DifferentOutputResets(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.Record([]ToolResult{{Name: "ui", Output: "result A"}})
	cb.Record([]ToolResult{{Name: "ui", Output: "result A"}})
	cb.Record([]ToolResult{{Name: "ui", Output: "result B"}})
	tripped, _ := cb.Record([]ToolResult{{Name: "ui", Output: "result A"}})
	if tripped {
		t.Fatal("trip after non-consecutive same outputs")
	}
}

// TestCircuitBreaker_OutputStreakResetsBetweenSkills — same Output text
// but a different skill name in the middle should reset the streak.
func TestCircuitBreaker_OutputStreakResetsBetweenSkills(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.Record([]ToolResult{{Name: "ui", Output: "same"}})
	cb.Record([]ToolResult{{Name: "ui", Output: "same"}})
	cb.Record([]ToolResult{{Name: "screen", Output: "same"}})
	tripped, _ := cb.Record([]ToolResult{{Name: "ui", Output: "same"}})
	if tripped {
		t.Fatal("trip across different skills should not happen")
	}
}

// TestCircuitBreaker_ThroughputDoesNotTrip — many calls to the same
// skill with VARIED output (e.g. researcher running knowledge_search
// across 12 different masters, web_search across 12 different queries)
// must NOT trip the breaker. This was the failure mode the v1.12.1
// removal of Trigger 4 (per-turn call cap) addressed: research turns
// legitimately make 8-15 calls to a single throughput skill, each
// gathering new material; the kernel mistook that throughput for
// "stuck verifying" and emitted a STOP message that derailed the
// research turn before file_write.
func TestCircuitBreaker_ThroughputDoesNotTrip(t *testing.T) {
	cb := NewCircuitBreaker()
	// 15 calls to knowledge_search, each with different output.
	// Pre-1.12.1 this tripped at call 8.
	for i := 0; i < 15; i++ {
		out := fmt.Sprintf("hits for master %d: ...", i)
		tripped, msg := cb.Record([]ToolResult{{Name: "knowledge_search", Output: out}})
		if tripped {
			t.Fatalf("call %d tripped breaker (varied output, no error) — should not. msg: %s", i+1, msg)
		}
	}
}

// TestCircuitBreaker_ErrorResetsOutputStreak — an error result in the
// middle of an identical-output streak must reset it (the world
// signal changed; the agent might recover).
func TestCircuitBreaker_ErrorResetsOutputStreak(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.Record([]ToolResult{{Name: "ui", Output: "same"}})
	cb.Record([]ToolResult{{Name: "ui", Output: "same"}})
	cb.Record([]ToolResult{{Name: "shell", Err: fmt.Errorf("boom")}})
	tripped, _ := cb.Record([]ToolResult{{Name: "ui", Output: "same"}})
	if tripped {
		t.Fatal("output streak should have been reset by intervening error")
	}
}
