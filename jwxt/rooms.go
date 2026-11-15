package jwxt

import (
	"context"
	"net/url"
)

// Rooms fetches xxjsjy.aspx (N121611) and triggers Button2 ("空教室查询").
func (c *Client) Rooms(ctx context.Context, overrides url.Values) (PageSnapshot, error) {
	path := c.BuildPagePath("xxjsjy.aspx", "N121611")
	snap, err := c.Snapshot(ctx, path)
	if err != nil {
		return PageSnapshot{}, err
	}
	form := url.Values{"Button2": {"空教室查询"}}
	for k, vs := range overrides {
		form[k] = append([]string(nil), vs...)
	}
	return c.PostBack(ctx, path, &snap, form)
}
