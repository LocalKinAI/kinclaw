//go:build darwin

package skill

import "testing"

// TestIsDestructiveTarget covers the second-line safety net for ui
// click. The first line — refusing on ambiguous matches — runs against
// real kinax AX trees and is exercised in integration tests; this
// function is pure-string and table-testable here.
func TestIsDestructiveTarget(t *testing.T) {
	tests := []struct {
		name  string
		role  string
		title string
		want  bool
	}{
		// Destructive AX roles — refuse regardless of title.
		{"close_button_role", "AXCloseButton", "", true},
		{"close_button_role_with_title", "AXCloseButton", "close calculator", true},
		{"minimize_button_role", "AXMinimizeButton", "", true},
		{"fullscreen_button_role", "AXFullScreenButton", "", true},

		// Destructive English titles (word-boundary semantics).
		{"title_close_exact", "AXButton", "Close", true},
		{"title_close_lowercase", "AXButton", "close", true},
		{"title_close_window", "AXButton", "Close Window", true},
		{"title_close_tab", "AXButton", "Close Tab", true},
		{"title_quit", "AXMenuItem", "Quit", true},
		{"title_quit_app", "AXMenuItem", "Quit Calculator", true},
		{"title_exit", "AXButton", "Exit", true},
		{"title_log_out", "AXMenuItem", "Log Out Jacky", true},
		{"title_sign_out", "AXButton", "Sign Out", true},

		// Non-destructive English — must pass.
		{"title_close_up_view", "AXButton", "Close-up View", false},
		{"title_closed_captions", "AXMenuItem", "Closed Captions", false},
		{"title_quitter", "AXButton", "Quitter", false},
		{"title_save", "AXButton", "Save", false},
		{"title_open", "AXButton", "Open", false},
		{"title_calculator", "AXButton", "Calculator", false},

		// Conservative-bias false positives: "Close <noun>" is treated
		// as destructive even when the noun isn't a window/app (e.g.
		// Instagram's "Close Friends" feature). The matcher prefers
		// false-refuse over false-accept; the LLM always has force=true
		// as the explicit override. Documented to avoid the test being
		// "fixed" later in a way that weakens the guard.
		{"title_close_friends_conservative_match", "AXButton", "Close Friends", true},

		// Destructive Chinese titles (substring match).
		{"title_zh_close", "AXButton", "关闭", true},
		{"title_zh_close_window", "AXButton", "关闭窗口", true},
		{"title_zh_quit", "AXMenuItem", "退出", true},
		{"title_zh_quit_app", "AXMenuItem", "退出 Chrome", true},
		{"title_zh_logout", "AXMenuItem", "注销", true},
		{"title_zh_end", "AXButton", "结束任务", true},

		// Non-destructive Chinese.
		{"title_zh_save", "AXButton", "保存", false},
		{"title_zh_open", "AXButton", "打开", false},

		// Empty / whitespace title with non-destructive role.
		{"empty_title", "AXButton", "", false},
		{"whitespace_title", "AXButton", "   ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDestructiveTarget(tt.role, tt.title)
			if got != tt.want {
				t.Errorf("isDestructiveTarget(%q, %q) = %v, want %v",
					tt.role, tt.title, got, tt.want)
			}
		})
	}
}
