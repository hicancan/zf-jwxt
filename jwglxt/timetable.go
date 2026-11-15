package jwglxt

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

const timetableModule = "N214505"

// Catalog lists every class timetable the term offers.
func (c *Client) Catalog(ctx context.Context, t Term) ([]map[string]any, error) {
	form := url.Values{
		"xnm": {t.xnm()}, "xqm": {t.xqm()},
		"xqh_id": {""}, "njdm_id": {""}, "jg_id": {""}, "zyh_id": {""}, "zyfx_id": {""},
		"bh_id": {""}, "xsdm": {""}, "pyccdm": {""},
		"queryModel.showCount": {"10000"}, "queryModel.currentPage": {"1"},
		"queryModel.sortName": {""}, "queryModel.sortOrder": {"asc"},
	}
	var resp struct {
		Items       []map[string]any `json:"items"`
		TotalResult int              `json:"totalResult"`
	}
	if err := c.PostJSON(ctx, "/kbdy/bjkbdy_cxBjkbdyTjkbList.html?gnmkdm="+timetableModule, form, &resp); err != nil {
		return nil, err
	}
	if resp.TotalResult != len(resp.Items) {
		return nil, fmt.Errorf("jwglxt: catalog truncated: declared %d, received %d", resp.TotalResult, len(resp.Items))
	}
	return resp.Items, nil
}

// Periods returns the daily period clock for the term.
func (c *Client) Periods(ctx context.Context, t Term) ([]map[string]any, error) {
	var out []map[string]any
	err := c.PostJSON(ctx, "/kbdy/bjkbdy_cxRjc.html", url.Values{"xnm": {t.xnm()}, "xqm": {t.xqm()}}, &out)
	return out, err
}

// ClassTimetable returns the timetable of one catalog entry.
func (c *Client) ClassTimetable(ctx context.Context, t Term, entry map[string]any) (map[string]any, error) {
	s := func(k string) string {
		if v, ok := entry[k]; ok && v != nil {
			return fmt.Sprint(v)
		}
		return ""
	}
	kind, display := s("tjkbzdm"), s("tjkbzxsdm")
	if kind == "" {
		kind = "1"
	}
	if display == "" {
		display = "0"
	}
	form := url.Values{
		"xnm": {t.xnm()}, "xqm": {t.xqm()},
		"xnmc": {fmt.Sprintf("%d-%d", t.Year, t.Year+1)}, "xqmmc": {strconv.Itoa(t.Number)},
		"xqh_id": {s("xqh_id")}, "njdm_id": {s("njdm_id")}, "jg_id": {s("jg_id")},
		"zyh_id": {s("zyh_id")}, "zyfx_id": {s("zyfx_id")}, "bh_id": {s("bh_id")},
		"tjkbzdm": {kind}, "tjkbzxsdm": {display},
	}
	var out map[string]any
	err := c.PostJSON(ctx, "/kbdy/bjkbdy_cxBjKb.html?gnmkdm="+timetableModule, form, &out)
	return out, err
}
