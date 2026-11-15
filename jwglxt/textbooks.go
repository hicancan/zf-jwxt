package jwglxt

import (
	"context"
	"net/url"
)

// Textbooks queries /jcydgl/xsjcyd_cxJcydIndex.html (25 raw fields per row).
func (c *Client) Textbooks(ctx context.Context, t Term, params url.Values, showCount, page int) (PagedResponse, error) {
	form := cloneValues(params)
	ApplyTerm(form, t)
	return c.PagedQuery(ctx, "/jcydgl/xsjcyd_cxJcydIndex.html?doType=query&gnmkdm=N253547", form, showCount, page)
}
