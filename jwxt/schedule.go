package jwxt

import (
	"context"
	"net/url"
)

// PersonalSchedule fetches xskbcx.aspx (N121603).
func (c *Client) PersonalSchedule(ctx context.Context, overrides url.Values) (PageSnapshot, error) {
	path := c.BuildPagePath("xskbcx.aspx", "N121603")
	snap, err := c.Snapshot(ctx, path)
	if err != nil || len(overrides) == 0 {
		return snap, err
	}
	return c.PostBack(ctx, path, &snap, overrides)
}

// ClassSchedule fetches tjkbcx.aspx (N121601).
func (c *Client) ClassSchedule(ctx context.Context, overrides url.Values) (PageSnapshot, error) {
	path := c.BuildPagePath("tjkbcx.aspx", "N121601")
	snap, err := c.Snapshot(ctx, path)
	if err != nil || len(overrides) == 0 {
		return snap, err
	}
	return c.PostBack(ctx, path, &snap, overrides)
}

// TeacherSchedule fetches jstjkbcx.aspx (N121602).
func (c *Client) TeacherSchedule(ctx context.Context, overrides url.Values) (PageSnapshot, error) {
	path := c.BuildPagePath("jstjkbcx.aspx", "N121602")
	snap, err := c.Snapshot(ctx, path)
	if err != nil || len(overrides) == 0 {
		return snap, err
	}
	return c.PostBack(ctx, path, &snap, overrides)
}

// TimeSchedule fetches bm_kbcx.aspx (N12312).
func (c *Client) TimeSchedule(ctx context.Context, overrides url.Values) (PageSnapshot, error) {
	path := c.BuildPagePath("bm_kbcx.aspx", "N12312")
	snap, err := c.Snapshot(ctx, path)
	if err != nil || len(overrides) == 0 {
		return snap, err
	}
	return c.PostBack(ctx, path, &snap, overrides)
}
