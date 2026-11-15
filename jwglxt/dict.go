package jwglxt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// DictKind identifies one of jwglxt's standard dictionary endpoints.
type DictKind string

const (
	DictColleges    DictKind = "colleges"
	DictDepartments DictKind = "departments"
	DictGrades      DictKind = "grades"
	DictMajors      DictKind = "majors"
	DictClasses     DictKind = "classes"
	DictRegions     DictKind = "regions"
	DictCampuses    DictKind = "campuses"
	DictBuildings   DictKind = "buildings"
	DictWeeks       DictKind = "weeks"
	DictPeriods     DictKind = "periods"
	DictTimeslots   DictKind = "timeslots"
	DictExamNames   DictKind = "exam-names"
	DictOrgs        DictKind = "orgs"
)

var pagedDictPaths = map[DictKind]string{
	DictColleges:    "/xtgl/comm_cxXydmPaged.html",
	DictDepartments: "/xtgl/comm_cxKkbmPaged.html",
	DictGrades:      "/xtgl/comm_cxNjPaged.html",
	DictMajors:      "/xtgl/comm_cxZydmPaged.html",
	DictClasses:     "/xtgl/comm_cxBjdmPaged.html",
}

var listDictPaths = map[DictKind]string{
	DictRegions:   "/xsxxxggl/xsxxwh_cxSsxList.html",
	DictCampuses:  "/query/query_cxXqList.html",
	DictBuildings: "/query/query_cxJxlList.html",
	DictWeeks:     "/kbdy/bjkbdy_cxZcxx.html",
	DictPeriods:   "/kbdy/bjkbdy_cxRjc.html",
	DictTimeslots: "/kbdy/bjkbdy_cxRsd.html",
	DictExamNames: "/ksglcommon/common_cxKsmcByXnxq.html",
	DictOrgs:      "/kckgl/kcjbxxwh_cxJglsxj.html",
}

// Dict queries any jwglxt dictionary endpoint and returns raw JSON (either a
// PagedResponse / all-pages bundle for paged dicts, or raw JSON array/object).
func (c *Client) Dict(ctx context.Context, kind DictKind, params url.Values, showCount, page int, allPages bool) (json.RawMessage, error) {
	if path, ok := pagedDictPaths[kind]; ok {
		if allPages {
			items, err := c.PagedQueryAll(ctx, path, params, showCount, 0)
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(map[string]any{"total": len(items), "items": items})
			return json.RawMessage(b), err
		}
		res, err := c.PagedQuery(ctx, path, params, showCount, page)
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(res)
		return json.RawMessage(b), err
	}
	if path, ok := listDictPaths[kind]; ok {
		return c.PostRaw(ctx, path, params)
	}
	return nil, fmt.Errorf("jwglxt: unknown dictionary kind %q", kind)
}
