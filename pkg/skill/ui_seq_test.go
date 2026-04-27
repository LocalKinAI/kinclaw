//go:build darwin

package skill

import (
	"reflect"
	"testing"
)

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", []string{}},
		{"1", []string{"1"}},
		{"1,2,3", []string{"1", "2", "3"}},
		{" 1 , 2 , 3 ", []string{"1", "2", "3"}},
		{"1,,2", []string{"1", "2"}}, // empty parts dropped
		{",1,", []string{"1"}},
		{"  ,  ,  ", []string{}}, // all blanks → empty
		{"1+1=", []string{"1+1="}},
		{"1, +, 1, =", []string{"1", "+", "1", "="}},
	}
	for _, tt := range tests {
		got := splitAndTrim(tt.in)
		if got == nil {
			got = []string{}
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitAndTrim(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
