package zbxclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newRPCServer answers each JSON-RPC method with a canned raw JSON result;
// the literal value "ERROR" returns a JSON-RPC error instead.
func newRPCServer(t *testing.T, results map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		raw, ok := results[req.Method]
		if !ok {
			raw = `[]`
		}
		if raw == "ERROR" {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params.","data":"boom"},"id":` + jsonID(req.ID) + `}`))
			return
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + raw + `,"id":` + jsonID(req.ID) + `}`))
	}))
}

func jsonID(id any) string {
	b, _ := json.Marshal(id)
	return string(b)
}
