package jwglxt

import (
	"context"
	"net/url"
)

// Exams queries /kwgl/kscx_cxXsksxxIndex.html (34 raw fields per exam row).
func (c *Client) Exams(ctx context.Context, t Term, params url.Values, showCount, page int) (PagedResponse, error) {
	form := cloneValues(params)
	ApplyTerm(form, t)
	return c.PagedQuery(ctx, "/kwgl/kscx_cxXsksxxIndex.html?doType=query&gnmkdm=N358105", form, showCount, page)
}

// UnscheduledExams queries /kwgl/kscx_cxWpkskcList.html (22 raw fields).
func (c *Client) UnscheduledExams(ctx context.Context, t Term, params url.Values, showCount, page int) (PagedResponse, error) {
	form := cloneValues(params)
	ApplyTerm(form, t)
	return c.PagedQuery(ctx, "/kwgl/kscx_cxWpkskcList.html?doType=query&gnmkdm=N358105", form, showCount, page)
}
