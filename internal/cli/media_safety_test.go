package cli

import (
	"strings"
	"testing"
)

func TestUpdateUserMediaUnknownUserOn52(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version": "5.2.2",
		"user.get":        []map[string]any{{"userid": "22", "alias": "Admin"}, {"userid": "23", "alias": "ops"}},
		"mediatype.get":   []map[string]any{{"mediatypeid": "1", "name": "Email", "type": "0"}},
	})
	defer cleanup()
	_, _, err := runCLI(t, "update_user_media", "zz_no_such_user", "Email", "x@example.invalid", "--yes")
	if err == nil || !strings.Contains(err.Error(), "user not found") {
		t.Fatalf("got %v", err)
	}
	if h.count("user.update") != 0 {
		t.Fatal("user.update must not be sent")
	}
}

func mediaUserMock(version string, medias []map[string]any) *rpcHandler {
	login := "username"
	if version < "5.4" {
		login = "alias"
	}
	return &rpcHandler{
		resps: map[string]any{
			"apiinfo.version": version,
			"mediatype.get": []map[string]any{
				{"mediatypeid": "1", "name": "Email", "type": "0"},
				{"mediatypeid": "4", "name": "Telegram", "type": "4"},
			},
			"user.update": map[string]any{"userids": []string{"23"}},
		},
		respond: func(method string, p map[string]any) (any, bool) {
			if method != "user.get" {
				return nil, false
			}
			if _, byID := p["userids"]; byID {
				if medias == nil {
					return rpcErr{Message: "boom"}, true
				}
				return []map[string]any{{"userid": "23", "medias": medias}}, true
			}
			return []map[string]any{{"userid": "23", login: "ops"}}, true
		},
	}
}

func TestUpdateUserMediaPreservesOthersAndUsesMedias(t *testing.T) {
	h, cleanup := setupMock(t, mediaUserMock("7.4.14", []map[string]any{
		{"mediaid": "9", "userid": "23", "mediatypeid": "4", "sendto": "12345", "active": "0", "severity": "63", "period": "1-7,00:00-24:00", "provisioned": "0"},
		{"mediaid": "10", "userid": "23", "mediatypeid": "1", "sendto": []string{"old@example.invalid"}, "active": "0", "severity": "63", "period": "1-7,00:00-24:00", "provisioned": "0"},
	}))
	defer cleanup()
	if _, _, err := runCLI(t, "update_user_media", "ops", "Email", "new@example.invalid", "--yes"); err != nil {
		t.Fatal(err)
	}
	p := h.last("user.update")
	if _, bad := p["user_medias"]; bad {
		t.Fatal("must send medias, not user_medias")
	}
	medias, _ := p["medias"].([]any)
	if len(medias) != 2 {
		t.Fatalf("both media must be kept, got %v", medias)
	}
	for _, m := range medias {
		mm := m.(map[string]any)
		if _, bad := mm["mediaid"]; bad {
			t.Fatalf("read-only mediaid sent: %v", mm)
		}
		switch mm["mediatypeid"] {
		case "1":
			if s, _ := mm["sendto"].([]any); len(s) != 1 || s[0] != "new@example.invalid" {
				t.Fatalf("email sendto must be updated array, got %#v", mm["sendto"])
			}
		case "4":
			if mm["sendto"] != "12345" {
				t.Fatalf("other media changed: %v", mm)
			}
		}
	}
}

func TestUpdateUserMediaFetchErrorAborts(t *testing.T) {
	h, cleanup := setupMock(t, mediaUserMock("7.4.14", nil))
	defer cleanup()
	if _, _, err := runCLI(t, "update_user_media", "ops", "Email", "new@example.invalid", "--yes"); err == nil {
		t.Fatal("fetch error must abort")
	}
	if h.count("user.update") != 0 {
		t.Fatal("user.update must not be sent after a failed media fetch")
	}
}

func TestUpdateUserMediaRefusesAmbiguousType(t *testing.T) {
	h, cleanup := setupMock(t, mediaUserMock("7.4.14", []map[string]any{
		{"mediatypeid": "1", "sendto": []string{"a@example.invalid"}, "active": "0", "severity": "63", "period": "1-7,00:00-24:00"},
		{"mediatypeid": "1", "sendto": []string{"b@example.invalid"}, "active": "0", "severity": "63", "period": "1-7,00:00-24:00"},
	}))
	defer cleanup()
	_, _, err := runCLI(t, "update_user_media", "ops", "Email", "c@example.invalid", "--yes")
	if err == nil || !strings.Contains(err.Error(), "refusing to guess") {
		t.Fatalf("got %v", err)
	}
	if h.count("user.update") != 0 {
		t.Fatal("no update on ambiguous media")
	}
}

func TestUpdateUserMediaSeverityRange(t *testing.T) {
	_, cleanup := setupMock(t, mediaUserMock("7.4.14", []map[string]any{}))
	defer cleanup()
	if _, _, err := runCLI(t, "update_user_media", "ops", "Email", "c@example.invalid", "--severity", "64"); err == nil || !strings.Contains(err.Error(), "--severity") {
		t.Fatalf("got %v", err)
	}
}
