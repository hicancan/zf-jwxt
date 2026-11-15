package jwglxt

import (
	"context"
	"encoding/json"
	"net/url"
)

// Enrollments queries /xkcx/xkmdcx_cxXkmdcxIndex.html (93 raw fields per row).
func (c *Client) Enrollments(ctx context.Context, t Term, params url.Values, showCount, page int) (PagedResponse, error) {
	form := cloneValues(params)
	ApplyTerm(form, t)
	return c.PagedQuery(ctx, "/xkcx/xkmdcx_cxXkmdcxIndex.html?doType=query&gnmkdm=N255010", form, showCount, page)
}

// SelectedCourses queries /xsxk/zzxkyzb_cxZzxkYzbChoosedDisplay.html.
func (c *Client) SelectedCourses(ctx context.Context, t Term, params url.Values) (json.RawMessage, error) {
	var err error
	if t.Year == 0 && !params.Has("xkxnm") {
		if t, err = c.CurrentTerm(ctx); err != nil {
			return nil, err
		}
	}
	form := cloneValues(params)
	if !form.Has("xkxnm") && t.Year > 0 {
		form.Set("xkxnm", t.xnm())
	}
	if !form.Has("xkxqm") && t.xqm() != "" {
		form.Set("xkxqm", t.xqm())
	}
	return c.PostRaw(ctx, "/xsxk/zzxkyzb_cxZzxkYzbChoosedDisplay.html?gnmkdm=N253512", form)
}
