package zbxclient

import (
	"context"
	"fmt"
)

type MediaTypeRecord struct {
	MediaTypeID string `json:"mediatypeid"`
	Name        string `json:"name"`
	Type        string `json:"type"` // 0=Email, 1=Script, 3=Jabber, 4=Webhook
	Status      string `json:"status"` // 0=Enabled, 1=Disabled
	Description string `json:"description"`
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
