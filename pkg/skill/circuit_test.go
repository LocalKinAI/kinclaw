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

// TestCircuitBreaker_OverIterationTrips covers trigger 4: a single
// skill called many times in one turn — even with different params
// and different outputs — signals over-verification or a fix-and-
// retry loop. Live observation: demo runs where the LLM bounced
// between ui tree → ui find → ui read → ui click → ui tree to
// "fix" an ambiguous verification, blowing through 10+ ui calls.
func TestCircuitBreaker_OverIterationTrips(t *testing.T) {
	cb := NewCircuitBreaker()
	// First 7 calls (varied output to avoid trigger 3): no trip.
	for i := 0; i < 7; i++ {
		out := fmt.Sprintf("variant %d", i)
		tripped, _ := cb.Record([]ToolResult{{Name: "ui", Output: out}})
		if tripped {
			t.Fatalf("trip on call %d, expected to wait until cbUsageMax=8", i+1)
		}
	}
	tripped, msg := cb.Record([]ToolResult{{Name: "ui", Output: "variant 7"}})
	if !tripped {
		t.Fatal("expected trip at 8th call to same skill in one turn")
	}
	if !strings.Contains(msg, "called") {
		t.Errorf("expected message to mention call count, got: %s", msg)
	}
}

// TestCircuitBreaker_OverIterationCountsAcrossOutcomes — failures and
// successes both count toward the per-turn usage cap.
func TestCircuitBreaker_OverIterationCountsAcrossOutcomes(t *testing.T) {
	cb := NewCircuitBreaker()
	for i := 0; i < 7; i++ {
		var r ToolResult
		if i%2 == 0 {
			r = ToolResult{Name: "shell", Output: fmt.Sprintf("ok %d", i)}
		} else {
			r = ToolResult{Name: "shell", Err: fmt.Errorf("err %d", i)}
		}
		cb.Record([]ToolResult{r})
	}
	tripped, _ := cb.Record([]ToolResult{{Name: "shell", Output: "ok again"}})
	if !tripped {
		t.Fatal("8th call (mixed outcomes) should trip the usage cap")
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
