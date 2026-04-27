package applifecycle

import (
	"reflect"
	"testing"
)

func TestParseAppleScriptList(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace_only", "   \n  ", nil},
		{
			"single",
			"com.apple.finder",
			[]string{"com.apple.finder"},
		},
		{
			"comma_separated",
			"com.apple.finder, com.apple.Safari, com.tencent.xinWeChat",
			[]string{"com.apple.finder", "com.apple.Safari", "com.tencent.xinWeChat"},
		},
		{
			"trailing_newline",
			"com.apple.finder, com.apple.Safari\n",
			[]string{"com.apple.finder", "com.apple.Safari"},
		},
		{
			"no_space_after_comma",
			"com.apple.finder,com.apple.Safari",
			[]string{"com.apple.finder", "com.apple.Safari"},
		},
		{
			"empty_segments_skipped",
			"com.apple.finder, , com.apple.Safari, ",
			[]string{"com.apple.finder", "com.apple.Safari"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseAppleScriptList(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("parseAppleScriptList(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
