package skill

import (
	"fmt"
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
