package jwxt

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strings"
)

// Option is a <select> option on an ASP.NET WebForms page.
type Option struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Selected bool   `json:"selected,omitempty"`
}

// CurriculumFilters describes the dropdowns available on pyjh.aspx.
type CurriculumFilters struct {
	Grades   []Option `json:"grades"`
	Colleges []Option `json:"colleges"`
	Majors   []Option `json:"majors"`
}

// CurriculumRow preserves all 16 columns from pyjh.aspx's #DBGrid table.
type CurriculumRow struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Credits    string `json:"credits"`
	Hours      string `json:"hours"`
	ExamMode   string `json:"examMode"`
	Nature     string `json:"nature"`
	Category   string `json:"category"`
	Term       string `json:"term"`
	CourseInfo string `json:"courseInfo,omitempty"`
	MinorFlag  string `json:"minorFlag,omitempty"`
	Direction  string `json:"direction,omitempty"`
	GroupCode  string `json:"groupCode,omitempty"`
	ModuleCode string `json:"moduleCode,omitempty"`
	PassStatus string `json:"passStatus,omitempty"`
	Weeks      string `json:"weeks,omitempty"`
	ShortTerm  string `json:"shortTerm,omitempty"`
}

var (
	selectPattern = regexp.MustCompile(`(?s)<select[^>]*name="(nj|xymc|zymc)"[^>]*>(.*?)</select>`)
	optPattern    = regexp.MustCompile(`(?s)<option[^>]*value="([^"]*)"[^>]*>(.*?)</option>`)
	gridPattern   = regexp.MustCompile(`(?s)<table[^>]*id="DBGrid"[^>]*>(.*?)</table>`)
	trPattern     = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	tdPattern     = regexp.MustCompile(`(?s)<t[dh][^>]*>(.*?)</t[dh]>`)
	tagPattern    = regexp.MustCompile(`<[^>]+>`)
)

// CurriculumPage fetches pyjh.aspx and parses the current filters and visible rows.
func (c *Client) CurriculumPage(ctx context.Context) (CurriculumFilters, []CurriculumRow, error) {
	path := "/pyjh.aspx?xh=" + url.QueryEscape(c.username) + "&gnmkdm=N121607"
	doc, err := c.GetPage(ctx, path)
	if err != nil {
		return CurriculumFilters{}, nil, err
	}
	return parseFilters(doc), parseGrid(doc), nil
}

// CurriculumByMajor selects a grade, college code, and major code on pyjh.aspx
// and returns the resulting curriculum rows (single page or all pages).
func (c *Client) CurriculumByMajor(ctx context.Context, grade, college, major string, allPages bool) ([]CurriculumRow, error) {
	rows, _, err := c.fetchCurriculumMajor(ctx, grade, college, major, allPages)
	return rows, err
}

func (c *Client) fetchCurriculumMajor(ctx context.Context, grade, college, major string, allPages bool) ([]CurriculumRow, string, error) {
	path := "/pyjh.aspx?xh=" + url.QueryEscape(c.username) + "&gnmkdm=N121607"
	doc, err := c.GetPage(ctx, path)
	if err != nil {
		return nil, "", err
	}
	vs := extractViewState(doc)
	if college != "" {
		doc, err = c.PostForm(ctx, path, url.Values{
			"__EVENTTARGET":   {"xymc"},
			"__EVENTARGUMENT": {""},
			"__VIEWSTATE":     {vs},
			"nj":              {grade},
			"xymc":            {college},
			"zymc":            {""},
			"xq":              {encodeGBK("全部")},
			"kcxz":            {encodeGBK("全部")},
		}, path)
		if err != nil {
			return nil, "", err
		}
		vs = extractViewState(doc)
	}
	if major != "" {
		doc, err = c.PostForm(ctx, path, url.Values{
			"__EVENTTARGET":   {"zymc"},
			"__EVENTARGUMENT": {""},
			"__VIEWSTATE":     {vs},
			"nj":              {grade},
			"xymc":            {college},
			"zymc":            {major},
			"xq":              {encodeGBK("全部")},
			"kcxz":            {encodeGBK("全部")},
		}, path)
		if err != nil {
			return nil, "", err
		}
		vs = extractViewState(doc)
	}
	all := parseGrid(doc)
	if !allPages {
		return all, doc, nil
	}
	seen := map[string]bool{}
	for _, r := range all {
		seen[r.Code+"|"+r.Term+"|"+r.Direction] = true
	}
	for page := 2; page <= 50; page++ {
		if strings.Contains(doc, `id="dpDBGrid_btnNextPage" disabled="disabled"`) {
			break
		}
		nextDoc, err := c.PostForm(ctx, path, url.Values{
			"__EVENTTARGET":   {"dpDBGrid:btnNextPage"},
			"__EVENTARGUMENT": {""},
			"__VIEWSTATE":     {vs},
			"nj":              {grade},
			"xymc":            {college},
			"zymc":            {major},
			"xq":              {encodeGBK("全部")},
			"kcxz":            {encodeGBK("全部")},
		}, path)
		if err != nil {
			return all, doc, err
		}
		nextRows := parseGrid(nextDoc)
		added := 0
		for _, r := range nextRows {
			key := r.Code + "|" + r.Term + "|" + r.Direction
			if !seen[key] {
				seen[key] = true
				all = append(all, r)
				added++
			}
		}
		if added == 0 {
			break
		}
		doc = nextDoc
		vs = extractViewState(doc)
	}
	return all, doc, nil
}

func parseFilters(doc string) CurriculumFilters {
	var f CurriculumFilters
	for _, sm := range selectPattern.FindAllStringSubmatch(doc, -1) {
		var opts []Option
		for _, om := range optPattern.FindAllStringSubmatch(sm[2], -1) {
			val := strings.TrimSpace(html.UnescapeString(om[1]))
			lbl := cleanCell(om[2])
			if val == "" && lbl == "" {
				continue
			}
			opts = append(opts, Option{Value: val, Label: lbl})
		}
		switch sm[1] {
		case "nj":
			f.Grades = opts
		case "xymc":
			f.Colleges = opts
		case "zymc":
			f.Majors = opts
		}
	}
	return f
}

func parseGrid(doc string) []CurriculumRow {
	gm := gridPattern.FindStringSubmatch(doc)
	if gm == nil {
		return nil
	}
	var rows []CurriculumRow
	for i, tr := range trPattern.FindAllStringSubmatch(gm[1], -1) {
		if i == 0 {
			continue // header row
		}
		cells := tdPattern.FindAllStringSubmatch(tr[1], -1)
		if len(cells) < 8 {
			continue
		}
		col := func(idx int) string {
			if idx >= len(cells) {
				return ""
			}
			return cleanCell(cells[idx][1])
		}
		code := col(0)
		if code == "" || code == "课程代码" {
			continue
		}
		rows = append(rows, CurriculumRow{
			Code:       code,
			Name:       col(1),
			Credits:    col(2),
			Hours:      col(3),
			ExamMode:   col(4),
			Nature:     col(5),
			Category:   col(6),
			Term:       col(7),
			CourseInfo: col(8),
			MinorFlag:  col(9),
			Direction:  col(10),
			GroupCode:  col(11),
			ModuleCode: col(12),
			PassStatus: col(13),
			Weeks:      col(14),
			ShortTerm:  col(15),
		})
	}
	return rows
}

func cleanCell(raw string) string {
	s := tagPattern.ReplaceAllString(raw, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.TrimSpace(s)
}
