package zbxclient

import (
	"context"
	"fmt"
)

type MediaTypeRecord struct {
	MediaTypeID string `json:"mediatypeid"`
	Name        string `json:"name"`
	Type        string `json:"type"`   // 0=Email, 1=Script, 3=Jabber, 4=Webhook
	Status      string `json:"status"` // 0=Enabled, 1=Disabled
	Description string `json:"description"`
}

type UserMediaRecord struct {
	MediaID     string `json:"mediaid"`
	MediaTypeID string `json:"mediatypeid"`
	MediaName   string `json:"-"`
	SendTo      string `json:"sendto"`
	Active      string `json:"active"` // 0=Enabled, 1=Disabled
	Period      string `json:"period"`
	Severity    string `json:"severity"`
}

func (c *Client) GetMediaTypes(ctx context.Context) ([]MediaTypeRecord, error) {
	var list []MediaTypeRecord
	err := c.Call(ctx, "mediatype.get", map[string]any{
		"output": []string{"mediatypeid", "name", "type", "status", "description"},
	}, &list)
	if err != nil {
		return nil, fmt.Errorf("fetching mediatypes: %w", err)
	}
	return list, nil
}

func (c *Client) GetUserMedia(ctx context.Context, usernameOrID string) ([]UserMediaRecord, error) {
	types, err := c.GetMediaTypes(ctx)
	if err != nil {
		return nil, err
	}
	typeNameMap := make(map[string]string)
	for _, t := range types {
		typeNameMap[t.MediaTypeID] = t.Name
	}

	type rawUser struct {
		UserID   string            `json:"userid"`
		Username string            `json:"username"`
		Medias   []UserMediaRecord `json:"medias"`
	}

	// Try search by username
	var users []rawUser
	err = c.Call(ctx, "user.get", map[string]any{
		"filter":       map[string]string{"username": usernameOrID},
		"selectMedias": "extend",
		"output":       []string{"userid", "username"},
	}, &users)
	if err != nil || len(users) == 0 {
		// Fallback for Zabbix 5.x/legacy field "alias" or by userid
		_ = c.Call(ctx, "user.get", map[string]any{
			"userids":      []string{usernameOrID},
			"selectMedias": "extend",
			"output":       []string{"userid", "username"},
		}, &users)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("user '%s' not found", usernameOrID)
	}

	var results []UserMediaRecord
	for _, m := range users[0].Medias {
		m.MediaName = typeNameMap[m.MediaTypeID]
		results = append(results, m)
	}
	return results, nil
}
