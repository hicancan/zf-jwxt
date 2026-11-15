package jwxt

import (
	"context"
	"net/url"
)

// Textbooks fetches xsjccx.aspx (N121608).
func (c *Client) Textbooks(ctx context.Context, overrides url.Values) (PageSnapshot, error) {
	path := c.BuildPagePath("xsjccx.aspx", "N121608")
	snap, err := c.Snapshot(ctx, path)
	if err != nil || len(overrides) == 0 {
		return snap, err
	}
	return c.PostBack(ctx, path, &snap, overrides)
}

// Profile fetches xsgrxx.aspx (N121501).
func (c *Client) Profile(ctx context.Context) (PageSnapshot, error) {
	return c.Snapshot(ctx, c.BuildPagePath("xsgrxx.aspx", "N121501"))
}

// Competitions fetches XKJS.aspx (N121302).
func (c *Client) Competitions(ctx context.Context, overrides url.Values) (PageSnapshot, error) {
	path := c.BuildPagePath("XKJS.aspx", "N121302")
	snap, err := c.Snapshot(ctx, path)
	if err != nil || len(overrides) == 0 {
		return snap, err
	}
	return c.PostBack(ctx, path, &snap, overrides)
}

// ThesisTopics fetches lw_xsxt.aspx (N121703).
func (c *Client) ThesisTopics(ctx context.Context, overrides url.Values) (PageSnapshot, error) {
	path := c.BuildPagePath("lw_xsxt.aspx", "N121703")
	snap, err := c.Snapshot(ctx, path)
	if err != nil || len(overrides) == 0 {
		return snap, err
	}
	return c.PostBack(ctx, path, &snap, overrides)
}

// TeachingPlans fetches jsskjhscgl.aspx (N123108).
func (c *Client) TeachingPlans(ctx context.Context, overrides url.Values) (PageSnapshot, error) {
	path := c.BuildPagePath("jsskjhscgl.aspx", "N123108")
	snap, err := c.Snapshot(ctx, path)
	if err != nil || len(overrides) == 0 {
		return snap, err
	}
	return c.PostBack(ctx, path, &snap, overrides)
}
