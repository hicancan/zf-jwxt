package jwglxt

import (
	"context"
	"net/url"
)

// Evaluations queries /xspjgl/xspj_cxXspjIndex.html (33 raw fields per row).
func (c *Client) Evaluations(ctx context.Context, params url.Values, showCount, page int) (PagedResponse, error) {
	return c.PagedQuery(ctx, "/xspjgl/xspj_cxXspjIndex.html?doType=query&gnmkdm=N401605", params, showCount, page)
}
