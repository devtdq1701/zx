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

func TestAddUserMediaDefaultsToDryRun(t *testing.T) {
	h, cleanup := setupMock(t, mediaUserMock("7.4.14", []map[string]any{}))
	defer cleanup()
	out, _, err := runCLI(t, "add_user_media", "ops", "--mediatype", "Email", "--sendto", "x@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "DRY-RUN") {
		t.Fatalf("expected dry-run preview, got %q", out)
	}
	if n := h.count("user.update"); n != 0 {
		t.Fatalf("default must not write, got %d user.update", n)
	}
}

func TestAddUserMediaDeprecatedDryrunStillPreviews(t *testing.T) {
	h, cleanup := setupMock(t, mediaUserMock("7.4.14", []map[string]any{}))
	defer cleanup()
	if _, _, err := runCLI(t, "add_user_media", "ops", "--mediatype", "Email", "--sendto", "x@example.invalid", "--dryrun", "--yes"); err != nil {
		t.Fatal(err)
	}
	if n := h.count("user.update"); n != 0 {
		t.Fatalf("--dryrun must never write, got %d user.update", n)
	}
}

func TestAddUserMediaYesWritesOnce(t *testing.T) {
	h, cleanup := setupMock(t, mediaUserMock("7.4.14", []map[string]any{}))
	defer cleanup()
	if _, _, err := runCLI(t, "add_user_media", "ops", "--mediatype", "Email", "--sendto", "x@example.invalid", "--severity", "0", "--yes"); err != nil {
		t.Fatal(err)
	}
	if n := h.count("user.update"); n != 1 {
		t.Fatalf("want one user.update, got %d", n)
	}
	medias, _ := h.last("user.update")["medias"].([]any)
	if len(medias) != 1 || medias[0].(map[string]any)["severity"] != float64(0) {
		t.Fatalf("explicit --severity 0 must be kept, got %v", medias)
	}
}

func TestAddUserMediaSeverityRange(t *testing.T) {
	for _, sev := range []string{"64", "-1"} {
		h, cleanup := setupMock(t, mediaUserMock("7.4.14", []map[string]any{}))
		_, _, err := runCLI(t, "add_user_media", "ops", "--mediatype", "Email", "--sendto", "x@example.invalid", "--severity", sev, "--yes")
		if err == nil || !strings.Contains(err.Error(), "severity") {
			t.Fatalf("%s: got %v", sev, err)
		}
		if n := h.count("user.update"); n != 0 {
			t.Fatalf("%s: user.update sent", sev)
		}
		cleanup()
	}
}

func telegramMock() *rpcHandler {
	return &rpcHandler{resps: map[string]any{
		"mediatype.get": []map[string]any{{"mediatypeid": "4", "name": "Telegram", "type": "4",
			"parameters": []map[string]any{{"name": "api_token", "value": ""}}}},
		"mediatype.create": map[string]any{"mediatypeids": []string{"99"}},
	}}
}

func TestCreateTelegramDefaultsToDryRunAndMasksToken(t *testing.T) {
	h, cleanup := setupMock(t, telegramMock())
	defer cleanup()
	out, _, err := runCLI(t, "create_telegram_mediatype", "zz_tg", "000000000:FAKESECRET")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "DRY-RUN") {
		t.Fatalf("expected dry-run preview, got %q", out)
	}
	if strings.Contains(out, "FAKESECRET") {
		t.Fatalf("token leaked in preview: %q", out)
	}
	if n := h.count("mediatype.create"); n != 0 {
		t.Fatalf("default must not create, got %d", n)
	}
}

func TestCreateTelegramYesCreatesOnce(t *testing.T) {
	h, cleanup := setupMock(t, telegramMock())
	defer cleanup()
	out, _, err := runCLI(t, "create_telegram_mediatype", "zz_tg", "000000000:FAKESECRET", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if n := h.count("mediatype.create"); n != 1 {
		t.Fatalf("want one mediatype.create, got %d", n)
	}
	if strings.Contains(out, "FAKESECRET") {
		t.Fatalf("token leaked in output: %q", out)
	}
}

func TestUpdateUserMediaKeepsUnsetFields(t *testing.T) {
	h, cleanup := setupMock(t, mediaUserMock("7.4.14", []map[string]any{
		{"mediatypeid": "1", "sendto": []string{"old@example.invalid"}, "active": "1", "severity": "48", "period": "1-5,08:00-17:00"},
	}))
	defer cleanup()
	if _, _, err := runCLI(t, "update_user_media", "ops", "Email", "new@example.invalid", "--yes"); err != nil {
		t.Fatal(err)
	}
	medias, _ := h.last("user.update")["medias"].([]any)
	if len(medias) != 1 {
		t.Fatalf("got %v", medias)
	}
	m := medias[0].(map[string]any)
	if m["active"] != "1" || m["severity"] != "48" || m["period"] != "1-5,08:00-17:00" {
		t.Fatalf("unset flags must keep existing values, got %v", m)
	}
	if s, _ := m["sendto"].([]any); len(s) != 1 || s[0] != "new@example.invalid" {
		t.Fatalf("sendto must be replaced, got %#v", m["sendto"])
	}
}

func TestUpdateUserMediaOverridesExplicitFields(t *testing.T) {
	h, cleanup := setupMock(t, mediaUserMock("7.4.14", []map[string]any{
		{"mediatypeid": "1", "sendto": []string{"old@example.invalid"}, "active": "1", "severity": "48", "period": "1-5,08:00-17:00"},
	}))
	defer cleanup()
	if _, _, err := runCLI(t, "update_user_media", "ops", "Email", "new@example.invalid", "--severity", "12", "--active=true", "--yes"); err != nil {
		t.Fatal(err)
	}
	m := h.last("user.update")["medias"].([]any)[0].(map[string]any)
	if m["active"] != "0" || m["severity"] != float64(12) || m["period"] != "1-5,08:00-17:00" {
		t.Fatalf("explicit flags must override only themselves, got %v", m)
	}
}
