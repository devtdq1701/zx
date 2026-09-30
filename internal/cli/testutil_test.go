package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"zx/internal/config"
)

// rpcErr makes the mock answer with a JSON-RPC error object.
type rpcErr struct{ Message string }

type rpcHandler struct {
	mu       sync.Mutex
	calls    []string
	requests []map[string]any
	resps    map[string]any
	// respond, when set, may answer a call based on its params; returning
	// false falls back to resps.
	respond func(method string, params map[string]any) (any, bool)
}

func (h *rpcHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
		ID     int             `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	h.mu.Lock()
	defer h.mu.Unlock()
	var p map[string]any
	_ = json.Unmarshal(req.Params, &p)
	h.calls = append(h.calls, req.Method)
	h.requests = append(h.requests, p)

	var result any
	handled := false
	if h.respond != nil {
		result, handled = h.respond(req.Method, p)
	}
	if !handled {
		var ok bool
		if result, ok = h.resps[req.Method]; !ok {
			result = []any{}
		}
	}
	resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	if e, ok := result.(rpcErr); ok {
		resp["error"] = map[string]any{"code": -32602, "message": e.Message, "data": e.Message}
	} else {
		resp["result"] = result
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *rpcHandler) count(method string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.calls {
		if c == method {
			n++
		}
	}
	return n
}

// last returns the params of the most recent call to method, or nil.
func (h *rpcHandler) last(method string) map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.calls) - 1; i >= 0; i-- {
		if h.calls[i] == method {
			return h.requests[i]
		}
	}
	return nil
}

func setupMock(t *testing.T, h *rpcHandler) (*rpcHandler, func()) {
	t.Helper()
	ts := httptest.NewServer(h)
	prev := testProfile
	testProfile = &config.Profile{URL: ts.URL, Token: "test-token"}
	resetFlags(rootCmd)
	return h, func() {
		ts.Close()
		testProfile = prev
		resetFlags(rootCmd)
	}
}

func setupMockClient(t *testing.T, resps map[string]any) (*rpcHandler, func()) {
	return setupMock(t, &rpcHandler{resps: resps})
}

// runCLI executes one command on a clean rootCmd and returns stdout/stderr.
func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()
	err := executeLine(context.Background(), args)
	return out.String(), errb.String(), err
}

func httptestServer(t *testing.T, h *rpcHandler) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}
