package scenarios

import (
 "net/http"
 "net/http/httptest"
 "net/url"
 "testing"
 "time"

 "fixture/buildtests/tests/mcp/steps"
 "github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

func New(t *testing.T, id string) *bddgo.Run[steps.State] {
 t.Helper()
 server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("X-Session-Ticket", "0123456789abcdef0123456789abcdef")
  w.Header().Set("X-MCP-Protocol", "2026-10-22")
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(http.StatusOK)
  _, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2026-10-22"}}`))
 }))
 t.Cleanup(server.Close)
 address, err := url.Parse(server.URL)
 if err != nil { t.Fatal(err) }
 t.Setenv("MCP_PORT", address.Port())
 suite, err := bddgo.New[steps.State](bddgo.Options{Registry: "../../docs/scenarios.yaml", Architecture: "../../docs/architecture/architecture.yaml", Suite: "mcp-service"})
 if err != nil { t.Fatal(err) }
 suite.Init(func(_ *bddgo.World) (*steps.State, error) {
  return &steps.State{BaseURL: server.URL, Client: &http.Client{Timeout: 10*time.Second}}, nil
 })
 steps.Register(suite)
 return suite.Scenario(t, id)
}
