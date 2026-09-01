package captcha

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CapSolver is a Solver backed by capsolver.com's REST API
// (createTask/getTaskResult). Requires a paid API key.
type CapSolver struct {
	APIKey string

	// BaseURL defaults to https://api.capsolver.com; overridable so tests
	// can point it at a local mock server.
	BaseURL string
	// HTTPClient defaults to http.DefaultClient.
	HTTPClient *http.Client
	// PollInterval and PollTimeout control how Solve waits for the
	// asynchronous task to complete. Default to 2s and 120s.
	PollInterval time.Duration
	PollTimeout  time.Duration
}

func (c *CapSolver) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return "https://api.capsolver.com"
}

func (c *CapSolver) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *CapSolver) pollInterval() time.Duration {
	if c.PollInterval > 0 {
		return c.PollInterval
	}
	return 2 * time.Second
}

func (c *CapSolver) pollTimeout() time.Duration {
	if c.PollTimeout > 0 {
		return c.PollTimeout
	}
	return 120 * time.Second
}

var _ Solver = (*CapSolver)(nil)

const antiTurnstileTaskType = "AntiTurnstileTaskProxyLess"

// Solve implements Solver. Only Turnstile challenges are supported.
func (c *CapSolver) Solve(ctx context.Context, req SolveRequest) (string, error) {
	if req.Type != Turnstile {
		return "", fmt.Errorf("captcha: CapSolver only supports Turnstile challenges, got %q", req.Type)
	}

	taskID, err := c.createTask(ctx, req)
	if err != nil {
		return "", fmt.Errorf("captcha: createTask failed: %w", err)
	}

	deadline := time.Now().Add(c.pollTimeout())
	for time.Now().Before(deadline) {
		token, ready, err := c.getTaskResult(ctx, taskID)
		if err != nil {
			return "", fmt.Errorf("captcha: getTaskResult failed: %w", err)
		}
		if ready {
			return token, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(c.pollInterval()):
		}
	}
	return "", fmt.Errorf("captcha: timed out waiting for solve after %s", c.pollTimeout())
}

type capSolverTask struct {
	Type       string `json:"type"`
	WebsiteURL string `json:"websiteURL"`
	WebsiteKey string `json:"websiteKey"`
}

type createTaskRequest struct {
	ClientKey string        `json:"clientKey"`
	Task      capSolverTask `json:"task"`
}

type capSolverError struct {
	ErrorID          int    `json:"errorId"`
	ErrorCode        string `json:"errorCode"`
	ErrorDescription string `json:"errorDescription"`
}

func (e capSolverError) asError() error {
	if e.ErrorID == 0 {
		return nil
	}
	return fmt.Errorf("%s: %s", e.ErrorCode, e.ErrorDescription)
}

type createTaskResponse struct {
	capSolverError
	TaskID string `json:"taskId"`
}

func (c *CapSolver) createTask(ctx context.Context, req SolveRequest) (string, error) {
	body := createTaskRequest{
		ClientKey: c.APIKey,
		Task: capSolverTask{
			Type:       antiTurnstileTaskType,
			WebsiteURL: req.PageURL,
			WebsiteKey: req.SiteKey,
		},
	}

	var resp createTaskResponse
	if err := c.post(ctx, "/createTask", body, &resp); err != nil {
		return "", err
	}
	if err := resp.asError(); err != nil {
		return "", err
	}
	return resp.TaskID, nil
}

type getTaskResultRequest struct {
	ClientKey string `json:"clientKey"`
	TaskID    string `json:"taskId"`
}

type getTaskResultResponse struct {
	capSolverError
	Status   string `json:"status"`
	Solution struct {
		Token string `json:"token"`
	} `json:"solution"`
}

func (c *CapSolver) getTaskResult(ctx context.Context, taskID string) (token string, ready bool, err error) {
	body := getTaskResultRequest{ClientKey: c.APIKey, TaskID: taskID}
	var resp getTaskResultResponse
	if err := c.post(ctx, "/getTaskResult", body, &resp); err != nil {
		return "", false, err
	}
	if err := resp.asError(); err != nil {
		return "", false, err
	}
	if resp.Status == "ready" {
		return resp.Solution.Token, true, nil
	}
	return "", false, nil
}

func (c *CapSolver) post(ctx context.Context, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(out)
}
