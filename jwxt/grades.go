package jwxt

import (
	"context"
	"net/url"
)

// Grades fetches xscj_gc.aspx and triggers the requested query button:
// "all" -> Button2 (历年成绩), "term" -> Button1 (学期成绩),
// "best" -> Button5 (课程最高成绩), "failed" -> Button6 (未通过成绩).
func (c *Client) Grades(ctx context.Context, mode string, overrides url.Values) (PageSnapshot, error) {
	path := c.BuildPagePath("xscj_gc.aspx", "N121605")
	snap, err := c.Snapshot(ctx, path)
	if err != nil {
		return PageSnapshot{}, err
	}
	form := url.Values{}
	for k, vs := range overrides {
		form[k] = append([]string(nil), vs...)
	}
	switch mode {
	case "", "all":
		form.Set("Button2", "历年成绩")
	case "term":
		form.Set("Button1", "按学期查询")
	case "best":
		form.Set("Button5", "课程最高成绩")
	case "failed":
		form.Set("Button6", "未通过成绩")
	default:
		form.Set("Button2", "历年成绩")
	}
	return c.PostBack(ctx, path, &snap, form)
}

// LevelExams fetches xsdjkscx.aspx (N121606).
func (c *Client) LevelExams(ctx context.Context) (PageSnapshot, error) {
	return c.Snapshot(ctx, c.BuildPagePath("xsdjkscx.aspx", "N121606"))
}

// MakeupExams fetches XsBkKsCx.aspx (N121617) with optional PostBack filters.
func (c *Client) MakeupExams(ctx context.Context, overrides url.Values) (PageSnapshot, error) {
	path := c.BuildPagePath("XsBkKsCx.aspx", "N121617")
	snap, err := c.Snapshot(ctx, path)
	if err != nil || len(overrides) == 0 {
		return snap, err
	}
	return c.PostBack(ctx, path, &snap, overrides)
}
