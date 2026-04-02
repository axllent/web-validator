package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRobotsAllowedWhenNoRobots(t *testing.T) {
	noRobots = true
	robotsContent = `User-agent: *
Disallow: /`

	if !robotsAllowed("https://example.com/any/path") {
		t.Error("expected robotsAllowed=true when noRobots=true")
	}
}

func TestRobotsAllowedExternalDomain(t *testing.T) {
	noRobots = false
	baseDomain = "example.com"
	robotsContent = `User-agent: *
Disallow: /`

	// External domain should always be allowed (robots.txt only applies to baseDomain)
	if !robotsAllowed("https://other.com/page") {
		t.Error("expected robotsAllowed=true for external domain")
	}
}

func TestRobotsAllowedDisallowedPath(t *testing.T) {
	noRobots = false
	baseDomain = "example.com"
	robotsContent = `User-agent: *
Disallow: /private/`

	if robotsAllowed("https://example.com/private/page") {
		t.Error("expected robotsAllowed=false for disallowed path")
	}
}

func TestRobotsAllowedPermittedPath(t *testing.T) {
	noRobots = false
	baseDomain = "example.com"
	robotsContent = `User-agent: *
Disallow: /private/`

	if !robotsAllowed("https://example.com/public/page") {
		t.Error("expected robotsAllowed=true for permitted path")
	}
}

func TestRobotsAllowedEmptyContent(t *testing.T) {
	noRobots = false
	baseDomain = "example.com"
	robotsContent = ""

	if !robotsAllowed("https://example.com/any/path") {
		t.Error("expected robotsAllowed=true when robots.txt is empty")
	}
}

func TestInitRobotsTxtDisabledByFlag(t *testing.T) {
	noRobots = true
	robotsContent = "existing"

	initRobotsTxt("https://example.com/")

	// Should return early without changing robotsContent
	if robotsContent != "existing" {
		t.Error("initRobotsTxt should not modify robotsContent when noRobots=true")
	}
}

func TestInitRobotsTxtFetchesContent(t *testing.T) {
	expectedContent := "User-agent: *\nDisallow: /admin/"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, expectedContent)
	}))
	defer srv.Close()

	noRobots = false
	robotsContent = ""
	timeoutSeconds = 10

	initRobotsTxt(srv.URL + "/")

	if robotsContent != expectedContent {
		t.Errorf("robotsContent = %q, want %q", robotsContent, expectedContent)
	}
	if noRobots {
		t.Error("noRobots should remain false after successful fetch")
	}
}

func TestInitRobotsTxtFallsBackOnNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	noRobots = false
	robotsContent = ""
	timeoutSeconds = 10

	initRobotsTxt(srv.URL + "/")

	if !noRobots {
		t.Error("expected noRobots=true when robots.txt returns 404")
	}
}

func TestInitRobotsTxtFallsBackOnNetworkError(t *testing.T) {
	noRobots = false
	robotsContent = ""
	timeoutSeconds = 1

	initRobotsTxt("http://127.0.0.1:1/")

	if !noRobots {
		t.Error("expected noRobots=true when robots.txt is unreachable")
	}
}

func TestInitRobotsTxtInvalidURL(t *testing.T) {
	noRobots = false
	robotsContent = ""

	initRobotsTxt("://invalid-url")

	if !noRobots {
		t.Error("expected noRobots=true for invalid base URL")
	}
}
