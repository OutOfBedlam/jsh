package jsh

import "testing"

func TestCleanPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"foo", "/foo"},
		{"/foo", "/foo"},
		{"foo/", "/foo"},
		{"/foo/", "/foo"},
		{"foo/bar", "/foo/bar"},
		{"/foo/bar/", "/foo/bar"},
		{"foo//bar", "/foo/bar"},
		{"foo/./bar", "/foo/bar"},
		{"foo/../bar", "/bar"},
		{"/", "/"},
		{".", "/"},
		{"", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := CleanPath(tt.input)
			if result != tt.expected {
				t.Errorf("cleanPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
