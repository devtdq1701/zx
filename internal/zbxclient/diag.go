package zbxclient

import (
	"context"
	"fmt"
)

type MediaTypeTestParams struct {
	MediaTypeID string `json:"mediatypeid"`
	SendTo      string `json:"sendto"`
	Subject     string `json:"subject"`
	Message     string `json:"message"`
}

type MediaTypeTestResult struct {
	Success bool   `json:"result"`
	Error   string `json:"error,omitempty"`
}

// TestMediaType sends a test message using the specified media type via mediatype.test.
func (c *Client) TestMediaType(ctx context.Context, p MediaTypeTestParams) (*MediaTypeTestResult, error) {
	if p.MediaTypeID == "" {
		return nil, fmt.Errorf("mediatypeid is required")
	}
	var res MediaTypeTestResult
	if err := c.Call(ctx, "mediatype.test", p, &res); err != nil {
		return nil, fmt.Errorf("mediatype.test: %w", err)
	}
	return &res, nil
}

// ExecuteItemNow triggers immediate item data collection via task.create (type 6).
func (c *Client) ExecuteItemNow(ctx context.Context, itemID string) error {
	if itemID == "" {
		return fmt.Errorf("itemid is required")
	}
	payload := map[string]any{
		"type": 6,
		"request": map[string]string{
			"itemid": itemID,
		},
	}
	var res struct {
		TaskIDs []string `json:"taskids"`
	}
	if err := c.Call(ctx, "task.create", payload, &res); err != nil {
		return fmt.Errorf("task.create: %w", err)
	}
	return nil
}
