package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no user home: %v", err)
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty_string", "", ""},
		{"absolute_path_unchanged", "/etc/hosts", "/etc/hosts"},
		{"relative_path_unchanged", "subdir/file", "subdir/file"},
		{"bare_tilde", "~", home},
		{"tilde_slash", "~/", home},
		{"tilde_slash_path", "~/Library/Caches/kinclaw", filepath.Join(home, "Library/Caches/kinclaw")},
		{"tilde_slash_dotfile", "~/.kinclaw/auth.json", filepath.Join(home, ".kinclaw/auth.json")},
		// Literal "~user" is left alone — Go's filepath doesn't know
		// about the OS user db, and shells handle it differently.
		{"tilde_user_literal", "~root", "~root"},
		{"tilde_user_with_path", "~root/x", "~root/x"},
		// Tilde inside the path (not at start) must not be touched.
		{"tilde_mid_path", "/var/~/foo", "/var/~/foo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandHome(tt.in); got != tt.want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
