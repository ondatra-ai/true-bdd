package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	httpClientTimeout = 30 * time.Second
	maxErrorBodyBytes = 2048
)

// ServerClient is the remote's typed HTTP client for the agent-facing
// harness RPC routes (plan §3.3). Every call is context-aware and
// returns a wrapped error on transport failure or a non-2xx status.
type ServerClient struct {
	baseURL string
	http    *http.Client
}

// NewServerClient builds a ServerClient targeting baseURL (e.g.
// http://127.0.0.1:4517).
func NewServerClient(baseURL string) *ServerClient {
	return &ServerClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: httpClientTimeout},
	}
}

// Register announces this incarnation and returns its session id.
func (c *ServerClient) Register(ctx context.Context, req RegisterRequest) (RegisterResponse, error) {
	var resp RegisterResponse

	err := c.postJSON(ctx, "/api/agent/register", req, &resp)
	if err != nil {
		return RegisterResponse{}, err
	}

	return resp, nil
}

// Poll sends one heartbeat and returns any dispatched run or answer.
func (c *ServerClient) Poll(ctx context.Context, req PollRequest) (PollResponse, error) {
	var resp PollResponse

	err := c.postJSON(ctx, "/api/agent/poll", req, &resp)
	if err != nil {
		return PollResponse{}, err
	}

	return resp, nil
}

// PostEvents uploads a contiguous batch of run events.
func (c *ServerClient) PostEvents(ctx context.Context, req EventsRequest) (EventsResponse, error) {
	var resp EventsResponse

	err := c.postJSON(ctx, "/api/agent/events", req, &resp)
	if err != nil {
		return EventsResponse{}, err
	}

	return resp, nil
}

// PostInventory uploads a folder-scoped inventory snapshot.
func (c *ServerClient) PostInventory(ctx context.Context, req InventoryRequest) (InventoryResponse, error) {
	var resp InventoryResponse

	err := c.postJSON(ctx, "/api/agent/inventory", req, &resp)
	if err != nil {
		return InventoryResponse{}, err
	}

	return resp, nil
}

func (c *ServerClient) postJSON(ctx context.Context, path string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal %s request: %w", path, err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build %s request: %w", path, err)
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("post %s: %w", path, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		snippet, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes))

		return fmt.Errorf("%w: %s %d: %s", errUnexpectedStatus, path, response.StatusCode, string(snippet))
	}

	err = json.NewDecoder(response.Body).Decode(out)
	if err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}

	return nil
}
