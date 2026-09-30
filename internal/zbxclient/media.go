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
	u, err := c.ResolveExactUser(ctx, usernameOrID)
	if err != nil {
		return nil, err
	}
	raw, err := c.GetUserMediasRaw(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	results := make([]UserMediaRecord, 0, len(raw))
	for _, m := range raw {
		typeID := mediaString(m["mediatypeid"])
		results = append(results, UserMediaRecord{
			MediaID:     mediaString(m["mediaid"]),
			MediaTypeID: typeID,
			MediaName:   typeNameMap[typeID],
			SendTo:      mediaString(m["sendto"]),
			Active:      mediaString(m["active"]),
			Period:      mediaString(m["period"]),
			Severity:    mediaString(m["severity"]),
		})
	}
	return results, nil
}

func (c *Client) CreateTelegramMediaType(ctx context.Context, name, token, parseMode string, dryRun bool) (*MediaTypeRecord, error) {
	var base []struct {
		MediaTypeID      string           `json:"mediatypeid"`
		Parameters       []map[string]any `json:"parameters"`
		Script           string           `json:"script"`
		MessageTemplates []any            `json:"message_templates"`
	}
	err := c.Call(ctx, "mediatype.get", map[string]any{
		"filter":                 map[string]string{"name": "Telegram"},
		"selectParameters":       "extend",
		"selectMessageTemplates": "extend",
	}, &base)
	if err != nil || len(base) == 0 {
		return nil, fmt.Errorf("base Telegram mediatype not found: %w", err)
	}

	maskedToken := "***"
	if len(token) > 8 {
		maskedToken = token[:8] + "***"
	}

	if dryRun {
		return &MediaTypeRecord{
			MediaTypeID: "0",
			Name:        name,
			Type:        "4",
			Status:      "0",
			Description: fmt.Sprintf("Cloned from Telegram (#%s), token=%s, parse_mode=%s", base[0].MediaTypeID, maskedToken, parseMode),
		}, nil
	}

	params := base[0].Parameters
	for _, p := range params {
		if p["name"] == "api_token" {
			p["value"] = token
		} else if p["name"] == "api_parse_mode" && parseMode != "" {
			p["value"] = parseMode
		}
	}

	var res struct {
		MediaTypeIDs []string `json:"mediatypeids"`
	}
	payload := map[string]any{
		"name":              name,
		"type":              4,
		"status":            0,
		"script":            base[0].Script,
		"parameters":        params,
		"message_templates": base[0].MessageTemplates,
	}
	if err := c.Call(ctx, "mediatype.create", payload, &res); err != nil {
		return nil, fmt.Errorf("creating mediatype: %w", err)
	}
	if len(res.MediaTypeIDs) == 0 {
		return nil, fmt.Errorf("no mediatypeid returned from server")
	}
	return &MediaTypeRecord{
		MediaTypeID: res.MediaTypeIDs[0],
		Name:        name,
		Type:        "4",
		Status:      "0",
	}, nil
}

func (c *Client) AddUserMedia(ctx context.Context, usernameOrID, mediatypeNameOrID, sendTo, period string, severity int, enabled, dryRun bool) ([]UserMediaRecord, error) {
	if severity < 0 || severity > 63 {
		return nil, fmt.Errorf("invalid severity %d; expected a bitmask 0..63", severity)
	}
	u, err := c.ResolveExactUser(ctx, usernameOrID)
	if err != nil {
		return nil, err
	}
	mt, err := c.ResolveExactMediaType(ctx, mediatypeNameOrID)
	if err != nil {
		return nil, err
	}
	raw, err := c.GetUserMediasRaw(ctx, u.UserID)
	if err != nil {
		return nil, err
	}

	activeStr := "0"
	if !enabled {
		activeStr = "1"
	}
	if period == "" {
		period = "1-7,00:00-24:00"
	}

	if dryRun {
		current, err := c.GetUserMedia(ctx, u.UserID)
		if err != nil {
			return nil, err
		}
		return append(current, UserMediaRecord{
			MediaID:     "(new)",
			MediaTypeID: mt.MediaTypeID,
			MediaName:   mt.Name,
			SendTo:      sendTo,
			Active:      activeStr,
			Period:      period,
			Severity:    fmt.Sprintf("%d", severity),
		}), nil
	}

	medias := append(EditableMedias(raw), map[string]any{
		"mediatypeid": mt.MediaTypeID,
		"sendto":      SendToValue(mt, sendTo),
		"period":      period,
		"severity":    severity,
		"active":      activeStr,
	})
	var res struct {
		UserIDs []string `json:"userids"`
	}
	if err := c.Call(ctx, "user.update", map[string]any{
		"userid": u.UserID,
		"medias": medias,
	}, &res); err != nil {
		return nil, fmt.Errorf("user.update failed: %w", err)
	}
	return c.GetUserMedia(ctx, u.UserID)
}
