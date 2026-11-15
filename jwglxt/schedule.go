package jwglxt

import (
	"context"
	"encoding/json"
	"net/url"
)

func (c *Client) ensureTerm(ctx context.Context, t Term, params url.Values) (Term, error) {
	if t.Year == 0 && !params.Has("xnm") {
		return c.CurrentTerm(ctx)
	}
	return t, nil
}

// PersonalSchedule queries /kbcx/xskbcx_cxXsgrkb.html (top-level metadata +
// 73-field kbList + sjkList + jxhjkcList).
func (c *Client) PersonalSchedule(ctx context.Context, t Term, params url.Values) (json.RawMessage, error) {
	var err error
	if t, err = c.ensureTerm(ctx, t, params); err != nil {
		return nil, err
	}
	form := cloneValues(params)
	ApplyTerm(form, t)
	if !form.Has("kzlx") {
		form.Set("kzlx", "ck")
	}
	return c.PostRaw(ctx, "/kbcx/xskbcx_cxXsgrkb.html?gnmkdm=N2151", form)
}

// ClassSchedules queries /kbdy/bjkbdy_cxBjkbdyTjkbList.html (33 raw fields).
func (c *Client) ClassSchedules(ctx context.Context, t Term, params url.Values, showCount, page int) (PagedResponse, error) {
	var err error
	if t, err = c.ensureTerm(ctx, t, params); err != nil {
		return PagedResponse{}, err
	}
	form := cloneValues(params)
	ApplyTerm(form, t)
	return c.PagedQuery(ctx, "/kbdy/bjkbdy_cxBjkbdyTjkbList.html?gnmkdm=N214505", form, showCount, page)
}

// WeeklySchedule queries /kbcx/xskbcxMobile_cxXsKb.html (week-by-week schedule).
func (c *Client) WeeklySchedule(ctx context.Context, t Term, params url.Values) (json.RawMessage, error) {
	var err error
	if t, err = c.ensureTerm(ctx, t, params); err != nil {
		return nil, err
	}
	form := cloneValues(params)
	ApplyTerm(form, t)
	return c.PostRaw(ctx, "/kbcx/xskbcxMobile_cxXsKb.html?gnmkdm=N2154", form)
}
