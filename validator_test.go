package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func nuValidatorResponse(messages string) string {
	return fmt.Sprintf(`{"messages":[%s]}`, messages)
}

func setupValidator(t *testing.T, statusCode int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(func() { srv.Close() })
	return srv
}

func TestValidateSkipsNonHTMLCSS(t *testing.T) {
	validateHTML = true
	validateCSS = true
	output := result{URL: "https://example.com/file.js"}
	reader := strings.NewReader("console.log('hi')")

	got := validate(output, reader, "application/javascript")

	if len(got.Errors) != 0 || len(got.ValidationErrors) != 0 {
		t.Errorf("expected no errors for non-HTML/CSS, got %v / %v", got.Errors, got.ValidationErrors)
	}
}

func TestValidateSkipsHTMLWhenDisabled(t *testing.T) {
	validateHTML = false
	validateCSS = true
	output := result{URL: "https://example.com/"}
	reader := strings.NewReader("<html></html>")

	got := validate(output, reader, "text/html; charset=utf-8")

	if len(got.Errors) != 0 || len(got.ValidationErrors) != 0 {
		t.Errorf("expected no errors when validateHTML=false, got %v / %v", got.Errors, got.ValidationErrors)
	}
}

func TestValidateSkipsCSSWhenDisabled(t *testing.T) {
	validateHTML = true
	validateCSS = false
	output := result{URL: "https://example.com/style.css"}
	reader := strings.NewReader("body { color: red; }")

	got := validate(output, reader, "text/css")

	if len(got.Errors) != 0 || len(got.ValidationErrors) != 0 {
		t.Errorf("expected no errors when validateCSS=false, got %v / %v", got.Errors, got.ValidationErrors)
	}
}

func TestValidateHTMLNoErrors(t *testing.T) {
	srv := setupValidator(t, 200, nuValidatorResponse(""))
	htmlValidator = srv.URL + "?out=json"
	validateHTML = true
	errorsProcessed.Store(0)

	output := result{URL: "https://example.com/"}
	reader := strings.NewReader("<html><body></body></html>")

	got := validate(output, reader, "text/html; charset=utf-8")

	if len(got.ValidationErrors) != 0 {
		t.Errorf("expected no validation errors, got %v", got.ValidationErrors)
	}
}

func TestValidateHTMLWithErrors(t *testing.T) {
	body := nuValidatorResponse(`{"type":"error","lastLine":1,"lastColumn":6,"message":"Stray end tag"}`)
	srv := setupValidator(t, 200, body)
	htmlValidator = srv.URL + "?out=json"
	validateHTML = true
	errorsProcessed.Store(0)

	output := result{URL: "https://example.com/"}
	reader := strings.NewReader("<html></div></html>")

	got := validate(output, reader, "text/html; charset=utf-8")

	if len(got.ValidationErrors) != 1 {
		t.Fatalf("expected 1 validation error, got %d", len(got.ValidationErrors))
	}
	if got.ValidationErrors[0].Message != "Stray end tag" {
		t.Errorf("unexpected error message: %q", got.ValidationErrors[0].Message)
	}
}

func TestValidateWarningsRespectFlag(t *testing.T) {
	body := nuValidatorResponse(`{"type":"info","lastLine":1,"lastColumn":6,"message":"Consider adding a lang attribute"}`)
	srv := setupValidator(t, 200, body)
	htmlValidator = srv.URL + "?out=json"
	validateHTML = true
	errorsProcessed.Store(0)

	output := result{URL: "https://example.com/"}

	showWarnings = false
	got := validate(output, strings.NewReader("<html></html>"), "text/html; charset=utf-8")
	if len(got.ValidationErrors) != 0 {
		t.Errorf("expected no warnings when showWarnings=false, got %v", got.ValidationErrors)
	}

	showWarnings = true
	got = validate(output, strings.NewReader("<html></html>"), "text/html; charset=utf-8")
	if len(got.ValidationErrors) != 1 {
		t.Errorf("expected 1 warning when showWarnings=true, got %d", len(got.ValidationErrors))
	}
}

func TestValidatorNonOKStatus(t *testing.T) {
	srv := setupValidator(t, 503, "Service Unavailable")
	htmlValidator = srv.URL + "?out=json"
	validateHTML = true
	errorsProcessed.Store(0)

	output := result{URL: "https://example.com/"}
	reader := strings.NewReader("<html></html>")

	got := validate(output, reader, "text/html; charset=utf-8")

	if len(got.Errors) == 0 {
		t.Error("expected an error for non-200 validator response")
	}
}

func TestValidatorInvalidJSON(t *testing.T) {
	srv := setupValidator(t, 200, "not json")
	htmlValidator = srv.URL + "?out=json"
	validateHTML = true
	errorsProcessed.Store(0)

	output := result{URL: "https://example.com/"}
	reader := strings.NewReader("<html></html>")

	got := validate(output, reader, "text/html; charset=utf-8")

	if len(got.Errors) == 0 {
		t.Error("expected an error for invalid JSON response")
	}
}

func TestValidatorNetworkError(t *testing.T) {
	// Point to a closed server
	htmlValidator = "http://127.0.0.1:1/nonexistent?out=json"
	validateHTML = true
	errorsProcessed.Store(0)

	output := result{URL: "https://example.com/"}

	got := validate(output, strings.NewReader("<html></html>"), "text/html; charset=utf-8")

	if len(got.Errors) == 0 {
		t.Error("expected an error for unreachable validator")
	}
}

// Ensure validate reads from body correctly via the io.Reader interface
func TestValidateCSSWithErrors(t *testing.T) {
	body := nuValidatorResponse(`{"type":"error","lastLine":2,"lastColumn":1,"message":"Property doesnt-exist doesn't exist"}`)
	srv := setupValidator(t, 200, body)
	htmlValidator = srv.URL + "?out=json"
	validateCSS = true
	errorsProcessed.Store(0)

	output := result{URL: "https://example.com/style.css", Type: "text/css"}

	var called bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	})
	srv2 := httptest.NewServer(handler)
	defer srv2.Close()
	htmlValidator = srv2.URL + "?out=json"

	got := validate(output, strings.NewReader("body { doesnt-exist: red; }"), "text/css")

	if !called {
		t.Error("expected validator to be called")
	}
	if len(got.ValidationErrors) != 1 {
		t.Errorf("expected 1 validation error, got %d", len(got.ValidationErrors))
	}
}
