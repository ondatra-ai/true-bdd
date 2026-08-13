package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	// httpClientTimeout is generous: the relay holds a poll up to ~5s server
	// side, so the client ceiling must clear that with slack (plan §2).
	httpClientTimeout = 15 * time.Second
	// maxResponseBytes bounds a relay response body defensively.
	maxResponseBytes = 8 * 1024 * 1024
)

// RelayClient is the remote's typed HTTP client for the v2 register / poll /
// reply protocol (plan §2). Every call is context-aware; Poll and Reply return
// the raw HTTP status so the caller can classify reconnect signals via
// queryserver.ClassifyStatus.
type RelayClient struct {
	baseURL string
	http    *http.Client
}

// NewRelayClient builds a RelayClient targeting baseURL (e.g.
// http://127.0.0.1:4517).
func NewRelayClient(baseURL string) *RelayClient {
	return &RelayClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: httpClientTimeout},
	}
}

// Register announces this incarnation and returns the negotiated epoch, reply
// budget, and capability token.
func (c *RelayClient) Register(ctx context.Context, req registerRequest) (registerResult, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return registerResult{}, fmt.Errorf("marshal register: %w", err)
	}

	resp, status, err := c.do(ctx, "/api/agent/register", payload, nil)
	if err != nil {
		return registerResult{}, err
	}

	if status != http.StatusOK {
		return registerResult{}, fmt.Errorf("%w: register %d", errUnexpectedStatus, status)
	}

	var out registerResult

	err = json.Unmarshal(resp, &out)
	if err != nil {
		return registerResult{}, fmt.Errorf("decode register: %w", err)
	}

	return out, nil
}

// Poll issues one long-poll. It returns (item, status, err): a non-nil item on
// a 200 work delivery, nil on a 204 empty hold or a 404 rejection (the caller
// re-registers on 404). The status lets the caller classify the outcome.
func (c *RelayClient) Poll(ctx context.Context, req pollRequest) (*workItem, int, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal poll: %w", err)
	}

	resp, status, err := c.do(ctx, "/api/agent/poll", payload, nil)
	if err != nil {
		return nil, 0, err
	}

	if status != http.StatusOK {
		return nil, status, nil
	}

	var item workItem

	err = json.Unmarshal(resp, &item)
	if err != nil {
		return nil, status, fmt.Errorf("decode work item: %w", err)
	}

	return &item, status, nil
}

// Reply posts the CLI result for one work item. The correlation triple travels
// in headers OUTSIDE the capped body (plan §2), so an over-cap body still
// yields a correlated 413. It returns the reply route's status.
func (c *RelayClient) Reply(
	ctx context.Context,
	sessionID string,
	epoch int,
	token, workID string,
	env replyEnvelope,
) (int, error) {
	payload, err := json.Marshal(env)
	if err != nil {
		return 0, fmt.Errorf("marshal reply: %w", err)
	}

	headers := map[string]string{
		"x-session-id":       sessionID,
		"x-connection-epoch": strconv.Itoa(epoch),
		"x-capability-token": token,
		"x-work-id":          workID,
	}

	_, status, err := c.do(ctx, "/api/agent/reply", payload, headers)
	if err != nil {
		return 0, err
	}

	return status, nil
}

// do performs one JSON POST and returns the response body bytes and status. A
// transport failure is an error; any HTTP status (including 4xx/5xx) is
// returned to the caller for classification.
func (c *RelayClient) do(
	ctx context.Context,
	path string,
	body []byte,
	headers map[string]string,
) ([]byte, int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("build %s request: %w", path, err)
	}

	request.Header.Set("Content-Type", "application/json")

	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("post %s: %w", path, err)
	}
	defer func() { _ = response.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("read %s response: %w", path, err)
	}

	return data, response.StatusCode, nil
}
