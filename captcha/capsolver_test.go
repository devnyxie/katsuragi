package captcha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCapSolver_Solve_Success(t *testing.T) {
	var createBody createTaskRequest
	var pollCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/createTask":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Errorf("failed to decode createTask body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(createTaskResponse{TaskID: "task-123"})
		case "/getTaskResult":
			pollCount++
			if pollCount < 2 {
				_ = json.NewEncoder(w).Encode(getTaskResultResponse{Status: "processing"})
				return
			}
			resp := getTaskResultResponse{Status: "ready"}
			resp.Solution.Token = "solved-token-abc"
			_ = json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	solver := &CapSolver{
		APIKey:       "test-key",
		BaseURL:      server.URL,
		PollInterval: 10 * time.Millisecond,
		PollTimeout:  5 * time.Second,
	}

	token, err := solver.Solve(context.Background(), SolveRequest{
		Type:    Turnstile,
		SiteKey: "0x-sitekey",
		PageURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if token != "solved-token-abc" {
		t.Fatalf("expected token %q, got %q", "solved-token-abc", token)
	}
	if createBody.ClientKey != "test-key" {
		t.Errorf("expected clientKey %q, got %q", "test-key", createBody.ClientKey)
	}
	if createBody.Task.Type != antiTurnstileTaskType {
		t.Errorf("expected task type %q, got %q", antiTurnstileTaskType, createBody.Task.Type)
	}
	if createBody.Task.WebsiteKey != "0x-sitekey" {
		t.Errorf("expected websiteKey %q, got %q", "0x-sitekey", createBody.Task.WebsiteKey)
	}
	if createBody.Task.WebsiteURL != "https://example.com" {
		t.Errorf("expected websiteURL %q, got %q", "https://example.com", createBody.Task.WebsiteURL)
	}
	if pollCount < 2 {
		t.Errorf("expected at least 2 polls (processing then ready), got %d", pollCount)
	}
}

func TestCapSolver_Solve_CreateTaskError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(createTaskResponse{
			capSolverError: capSolverError{ErrorID: 1, ErrorCode: "ERROR_KEY_DENIED_ACCESS", ErrorDescription: "invalid key"},
		})
	}))
	defer server.Close()

	solver := &CapSolver{APIKey: "bad-key", BaseURL: server.URL}
	_, err := solver.Solve(context.Background(), SolveRequest{Type: Turnstile, SiteKey: "x", PageURL: "https://example.com"})
	if err == nil {
		t.Fatalf("expected an error for a denied API key, got none")
	}
}

func TestCapSolver_Solve_UnsupportedType(t *testing.T) {
	solver := &CapSolver{APIKey: "test-key"}
	_, err := solver.Solve(context.Background(), SolveRequest{Type: "recaptcha", SiteKey: "x", PageURL: "https://example.com"})
	if err == nil {
		t.Fatalf("expected an error for an unsupported challenge type, got none")
	}
}

func TestCapSolver_Solve_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/createTask":
			_ = json.NewEncoder(w).Encode(createTaskResponse{TaskID: "task-slow"})
		case "/getTaskResult":
			_ = json.NewEncoder(w).Encode(getTaskResultResponse{Status: "processing"})
		}
	}))
	defer server.Close()

	solver := &CapSolver{
		APIKey:       "test-key",
		BaseURL:      server.URL,
		PollInterval: 10 * time.Millisecond,
		PollTimeout:  100 * time.Millisecond,
	}
	_, err := solver.Solve(context.Background(), SolveRequest{Type: Turnstile, SiteKey: "x", PageURL: "https://example.com"})
	if err == nil {
		t.Fatalf("expected a timeout error when the task never becomes ready, got none")
	}
}
