package zbxclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"zx/internal/config"
)

func TestDownloadCombinedGraph(t *testing.T) {
	var loggedIn bool
	var timeUpdated bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.php":
			_ = r.ParseForm()
			if r.FormValue("name") == "admin" && r.FormValue("password") == "secret" {
				loggedIn = true
				http.SetCookie(w, &http.Cookie{Name: "zbx_session", Value: "valid_session_123"})
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		case "/zabbix.php":
			if r.URL.Query().Get("action") == "timeselector.update" {
				timeUpdated = true
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case "/chart.php":
			if !loggedIn {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			// Return dummy valid PNG bytes starting with \x89PNG
			pngBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00}
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	prof := &config.Profile{
		URL:       ts.URL,
		User:      "admin",
		Password:  "secret",
		VerifySSL: false,
	}

	client := NewClient(prof, 5*time.Second)

	tmpDir, err := os.MkdirTemp("", "zx-graph-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outPath := filepath.Join(tmpDir, "graph.png")
	err = client.DownloadCombinedGraph(context.Background(), []string{"1001", "1002"}, "now-7d", "now", 2050, 368, outPath)
	if err != nil {
		t.Fatalf("DownloadCombinedGraph failed: %v", err)
	}

	if !timeUpdated {
		t.Errorf("expected timeselector to be updated")
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading downloaded graph: %v", err)
	}
	if len(data) < 4 || data[0] != 0x89 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
		t.Errorf("invalid PNG header in downloaded file: %v", data[:4])
	}
}
