package zf

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const timetableModule = "N214505"

// Term is a ZhengFang term: Year is the starting calendar year (xnm) and
// Number is 1, 2 or 3.
type Term struct {
	Year   int `json:"year"`
	Number int `json:"number"`
}

var termCodes = map[int]string{1: "3", 2: "12", 3: "16"}

func (t Term) xnm() string { return strconv.Itoa(t.Year) }
func (t Term) xqm() string { return termCodes[t.Number] }

// String is the conventional label, e.g. "2026-2027-1".
func (t Term) String() string { return fmt.Sprintf("%d-%d-%d", t.Year, t.Year+1, t.Number) }

// ParseTerm accepts "2026-2027-1" or "2026-1".
func ParseTerm(s string) (Term, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 && len(parts) != 3 {
		return Term{}, fmt.Errorf("zf: term %q: want YYYY-YYYY-N or YYYY-N", s)
	}
	year, err1 := strconv.Atoi(parts[0])
	number, err2 := strconv.Atoi(parts[len(parts)-1])
	if err1 != nil || err2 != nil || termCodes[number] == "" {
		return Term{}, fmt.Errorf("zf: term %q is not valid", s)
	}
	return Term{Year: year, Number: number}, nil
}

var selectedPattern = regexp.MustCompile(`(?s)<select[^>]*name="(xnm|xqm)"[^>]*>.*?</select>`)
var optionSelected = regexp.MustCompile(`<option[^>]*value="([^"]+)"[^>]*selected`)

// CurrentTerm reads the term the timetable page preselects.
func (c *Client) CurrentTerm(ctx context.Context) (Term, error) {
	page, err := c.GetPage(ctx, "/kbdy/bjkbdy_cxBjkbdyIndex.html?gnmkdm="+timetableModule+"&layout=default")
	if err != nil {
		return Term{}, err
	}
	values := map[string]string{}
	for _, m := range selectedPattern.FindAllStringSubmatch(page, -1) {
		if s := optionSelected.FindStringSubmatch(m[0]); s != nil {
			values[m[1]] = s[1]
		}
	}
	year, err := strconv.Atoi(values["xnm"])
	if err != nil {
		return Term{}, fmt.Errorf("zf: current term not found on timetable page")
	}
	for number, code := range termCodes {
		if code == values["xqm"] {
			return Term{Year: year, Number: number}, nil
		}
	}
	return Term{}, fmt.Errorf("zf: unknown term code %q", values["xqm"])
}

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
		return nil, fmt.Errorf("zf: catalog truncated: declared %d, received %d", resp.TotalResult, len(resp.Items))
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
