package jwglxt

import (
	"context"
	"net/url"
)

// CurriculumCourses queries /xspyfagl/xspyfaxxwh_cxGrpyfakcxxIndex.html
// (29 raw fields per course; supports filtering by njdm_id, kch, etc.).
func (c *Client) CurriculumCourses(ctx context.Context, params url.Values, showCount, page int) (PagedResponse, error) {
	return c.PagedQuery(ctx, "/xspyfagl/xspyfaxxwh_cxGrpyfakcxxIndex.html?doType=query&gnmkdm=N1532", params, showCount, page)
}

// ExecPlanCourses queries /jxzxjhgl/jxzxjhxxwh_cxXsyxxfwhIndex.html
// (teaching execution plan course pool, 7 raw fields per row).
func (c *Client) ExecPlanCourses(ctx context.Context, params url.Values, showCount, page int) (PagedResponse, error) {
	return c.PagedQuery(ctx, "/jxzxjhgl/jxzxjhxxwh_cxXsyxxfwhIndex.html?doType=query&gnmkdm=N2511", params, showCount, page)
}
