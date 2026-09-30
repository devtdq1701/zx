package zbxclient

import (
	"context"
	"fmt"
	"strings"
)

type UserRef struct {
	UserID string
	Login  string
}

// ResolveExactUser finds exactly one user by login (alias before 5.4,
// username from 5.4) or numeric userid. Rows are re-checked locally: an
// unknown filter field makes Zabbix return every user.
func (c *Client) ResolveExactUser(ctx context.Context, login string) (UserRef, error) {
	login = strings.TrimSpace(login)
	v54, err := c.APIAtLeast(ctx, 5, 4)
	if err != nil {
		return UserRef{}, err
	}
	field := "alias"
	if v54 {
		field = "username"
	}
	params := map[string]any{"output": []string{"userid", field}}
	if isNumericID(login) {
		params["userids"] = []string{login}
	} else {
		params["filter"] = map[string]string{field: login}
	}
	var rows []map[string]any
	if err := c.Call(ctx, "user.get", params, &rows); err != nil {
		return UserRef{}, fmt.Errorf("user.get: %w", err)
	}
	var users []UserRef
	for _, r := range rows {
		id, name := mediaString(r["userid"]), mediaString(r[field])
		if id == login || name == login {
			users = append(users, UserRef{UserID: id, Login: name})
		}
	}
	return exactlyOne("user", login, users, func(u UserRef) string { return u.Login + " (" + u.UserID + ")" })
}

// GetUserMediasRaw returns the media objects of one user as the API sent them.
func (c *Client) GetUserMediasRaw(ctx context.Context, userID string) ([]map[string]any, error) {
	var rows []struct {
		UserID string           `json:"userid"`
		Medias []map[string]any `json:"medias"`
	}
	if err := c.Call(ctx, "user.get", map[string]any{
		"userids":      []string{userID},
		"output":       []string{"userid"},
		"selectMedias": "extend",
	}, &rows); err != nil {
		return nil, fmt.Errorf("user.get medias: %w", err)
	}
	if len(rows) != 1 || rows[0].UserID != userID {
		return nil, fmt.Errorf("user.get medias: expected user %s, got %d rows", userID, len(rows))
	}
	return rows[0].Medias, nil
}

// EditableMedias turns fetched media into user.update input: only writable
// fields are kept, and provisioned (LDAP/SAML) media are left to Zabbix.
func EditableMedias(raw []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(raw))
	for _, m := range raw {
		if mediaString(m["provisioned"]) == "1" {
			continue
		}
		e := map[string]any{}
		for _, k := range []string{"mediatypeid", "sendto", "active", "severity", "period"} {
			if v, ok := m[k]; ok {
				e[k] = v
			}
		}
		out = append(out, e)
	}
	return out
}

// ResolveExactMediaType matches a media type by ID or case-insensitive name.
func (c *Client) ResolveExactMediaType(ctx context.Context, nameOrID string) (MediaTypeRecord, error) {
	types, err := c.GetMediaTypes(ctx)
	if err != nil {
		return MediaTypeRecord{}, err
	}
	var found []MediaTypeRecord
	for _, t := range types {
		if t.MediaTypeID == nameOrID || strings.EqualFold(t.Name, nameOrID) {
			found = append(found, t)
		}
	}
	return exactlyOne("mediatype", nameOrID, found, func(t MediaTypeRecord) string { return t.Name + " (" + t.MediaTypeID + ")" })
}

// SendToValue shapes sendto for the media type: email media take an array.
func SendToValue(mt MediaTypeRecord, sendTo string) any {
	if mt.Type == "0" {
		return []string{sendTo}
	}
	return sendTo
}

// mediaString renders an API scalar or string array as plain text.
func mediaString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []any:
		parts := make([]string, 0, len(x))
		for _, p := range x {
			parts = append(parts, mediaString(p))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(x)
	}
}
