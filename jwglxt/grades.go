package jwglxt

import (
	"context"
	"encoding/json"
	"net/url"
)

// Grades queries /cjcx/cjcx_cxXsgrcj.html (70 raw fields per grade row).
func (c *Client) Grades(ctx context.Context, t Term, params url.Values, showCount, page int) (PagedResponse, error) {
	form := cloneValues(params)
	if !form.Has("xnm") {
		form.Set("xnm", "")
	}
	if !form.Has("xqm") {
		form.Set("xqm", "")
	}
	ApplyTerm(form, t)
	return c.PagedQuery(ctx, "/cjcx/cjcx_cxXsgrcj.html?doType=query&gnmkdm=N305005", form, showCount, page)
}

// CreditStats queries /cjcx/cjcx_cxXsxftj.html (student credit statistics).
func (c *Client) CreditStats(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.PostRaw(ctx, "/cjcx/cjcx_cxXsxftj.html?gnmkdm=N305005", params)
}
