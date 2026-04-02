package main

import (
	"testing"
)

func TestGetHost(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com/path", "example.com"},
		{"http://example.com:8080/path", "example.com:8080"},
		{"https://sub.example.com", "sub.example.com"},
		{"https://example.com", "example.com"},
		{"invalid-url", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := getHost(tt.input)
		if got != tt.want {
			t.Errorf("getHost(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAbsoluteURL(t *testing.T) {
	tests := []struct {
		link     string
		baseLink string
		want     string
		wantErr  bool
	}{
		{"/path/to/page", "https://example.com/start", "https://example.com/path/to/page", false},
		{"page.html", "https://example.com/dir/start.html", "https://example.com/dir/page.html", false},
		{"https://other.com/page", "https://example.com/start", "https://other.com/page", false},
		{"//other.com/page", "https://example.com/start", "https://other.com/page", false},
		{"/page#section", "https://example.com/", "https://example.com/page", false}, // fragment stripped
		{"mailto:foo@bar.com", "https://example.com/", "", true},
		{"javascript:void(0)", "https://example.com/", "", true},
		{"ftp://files.example.com/file", "https://example.com/", "", true},
	}

	for _, tt := range tests {
		baseDomain = "" // reset global between tests
		got, err := absoluteURL(tt.link, tt.baseLink)
		if tt.wantErr {
			if err == nil {
				t.Errorf("absoluteURL(%q, %q): expected error, got %q", tt.link, tt.baseLink, got)
			}
		} else {
			if err != nil {
				t.Errorf("absoluteURL(%q, %q): unexpected error: %v", tt.link, tt.baseLink, err)
			} else if got != tt.want {
				t.Errorf("absoluteURL(%q, %q) = %q, want %q", tt.link, tt.baseLink, got, tt.want)
			}
		}
	}
}

func TestAbsoluteURLSetsBaseDomain(t *testing.T) {
	baseDomain = ""
	_, err := absoluteURL("/page", "https://example.com/start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if baseDomain != "example.com" {
		t.Errorf("baseDomain = %q, want %q", baseDomain, "example.com")
	}
	// Should not overwrite once set
	_, err = absoluteURL("/page", "https://other.com/start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if baseDomain != "example.com" {
		t.Errorf("baseDomain overwritten: got %q, want %q", baseDomain, "example.com")
	}
}

func TestIsMixedContent(t *testing.T) {
	tests := []struct {
		src  string
		ref  string
		want bool
	}{
		{"https://example.com/page", "http://example.com/img.jpg", true},
		{"https://example.com/page", "https://example.com/img.jpg", false},
		{"http://example.com/page", "http://example.com/img.jpg", false},
		{"http://example.com/page", "https://example.com/img.jpg", false},
		{"invalid", "https://example.com/img.jpg", false},
		{"https://example.com/page", "invalid", false},
	}

	for _, tt := range tests {
		got := isMixedContent(tt.src, tt.ref)
		if got != tt.want {
			t.Errorf("isMixedContent(%q, %q) = %v, want %v", tt.src, tt.ref, got, tt.want)
		}
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input string
		num   int
		want  string
	}{
		{"hello world", 5, "he..."},
		{"hello world", 8, "hello..."},
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hi", 3, "hi"},
		{"abcde", 4, "a..."},
		{"abc", 3, "abc"},
	}

	for _, tt := range tests {
		got := truncateString(tt.input, tt.num)
		if got != tt.want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.num, got, tt.want)
		}
	}
}

func TestActionWeight(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"parse", 2},
		{"head", 1},
		{"", 1},
		{"other", 1},
	}

	for _, tt := range tests {
		got := actionWeight(tt.input)
		if got != tt.want {
			t.Errorf("actionWeight(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestExtractStyleURLs(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{`background: url("https://example.com/img.png")`, []string{"https://example.com/img.png"}},
		{`background: url('https://example.com/img.png')`, []string{"https://example.com/img.png"}},
		{`background: url(https://example.com/img.png)`, []string{"https://example.com/img.png"}},
		{`url( /path/to/img.png )`, []string{"/path/to/img.png"}},
		{`url("a.png") url("b.png")`, []string{"a.png", "b.png"}},
		{`no urls here`, []string{}},
		{`url("")`, []string{}},
		{`url('')`, []string{}},
		{`url()`, []string{}},
	}

	for _, tt := range tests {
		got := extractStyleURLs(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("extractStyleURLs(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("extractStyleURLs(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}
