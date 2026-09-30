package zbxclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestResolveExactUserIgnoresUnfilteredRowsOn52(t *testing.T) {
	var filterBody string
	c := exactClient(t, func(m string, p json.RawMessage) string {
		switch m {
		case "apiinfo.version":
			return `"5.2.2"`
		case "user.get":
			filterBody = string(p)
			// 5.2 ignores unknown filter fields and returns everyone.
			return `[{"userid":"22","alias":"Admin"},{"userid":"23","alias":"ops"}]`
		}
		return `[]`
	})
	if _, err := c.ResolveExactUser(context.Background(), "zz_no_such_user"); err == nil || !strings.Contains(err.Error(), "user not found") {
		t.Fatalf("unknown user must not resolve, got %v", err)
	}
	if !strings.Contains(filterBody, `"alias"`) {
		t.Fatalf("5.2 must filter by alias, sent %s", filterBody)
	}
	u, err := c.ResolveExactUser(context.Background(), "ops")
	if err != nil || u.UserID != "23" {
		t.Fatalf("got %+v %v", u, err)
	}
}

func TestResolveExactUserUsesUsernameOn74(t *testing.T) {
	var body string
	c := exactClient(t, func(m string, p json.RawMessage) string {
		switch m {
		case "apiinfo.version":
			return `"7.4.14"`
		case "user.get":
			body = string(p)
			return `[{"userid":"23","username":"ops"}]`
		}
		return `[]`
	})
	if _, err := c.ResolveExactUser(context.Background(), "ops"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `"username":"ops"`) {
		t.Fatalf("7.4 must filter by username, sent %s", body)
	}
}

func TestEditableMediasKeepsWritableFieldsOnly(t *testing.T) {
	raw := []map[string]any{
		{"mediaid": "9", "userid": "23", "mediatypeid": "4", "sendto": "12345", "active": "0", "severity": "63", "period": "1-7,00:00-24:00", "provisioned": "0"},
		{"mediaid": "10", "mediatypeid": "1", "sendto": []any{"a@example.invalid"}, "active": "0", "severity": "63", "period": "1-7,00:00-24:00", "provisioned": "1"},
	}
	got := EditableMedias(raw)
	if len(got) != 1 {
		t.Fatalf("provisioned media must be dropped, got %v", got)
	}
	for _, k := range []string{"mediaid", "userid", "provisioned"} {
		if _, ok := got[0][k]; ok {
			t.Fatalf("read-only key %s kept: %v", k, got[0])
		}
	}
	if got[0]["sendto"] != "12345" {
		t.Fatalf("sendto changed: %v", got[0])
	}
}

func TestSendToValueEmailIsArray(t *testing.T) {
	if v, ok := SendToValue(MediaTypeRecord{Type: "0"}, "a@example.invalid").([]string); !ok || v[0] != "a@example.invalid" {
		t.Fatalf("email sendto must be []string, got %#v", SendToValue(MediaTypeRecord{Type: "0"}, "a@example.invalid"))
	}
	if v, ok := SendToValue(MediaTypeRecord{Type: "4"}, "123").(string); !ok || v != "123" {
		t.Fatal("webhook sendto must be string")
	}
	if mediaString([]any{"a", "b"}) != "a,b" || mediaString(nil) != "" || mediaString("x") != "x" {
		t.Fatal("mediaString")
	}
}
