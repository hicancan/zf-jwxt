package jwxt

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strings"
)

// TableData holds a parsed <table id="..."> with raw rows and header-mapped records.
type TableData struct {
	ID      string              `json:"id"`
	Headers []string            `json:"headers,omitempty"`
	Rows    [][]string          `json:"rows,omitempty"`
	Records []map[string]string `json:"records,omitempty"`
}

// PageSnapshot captures the full form state and all tables of an ASP.NET WebForms page.
type PageSnapshot struct {
	Path      string               `json:"path"`
	ViewState string               `json:"-"`
	Selects   map[string][]Option  `json:"selects,omitempty"`
	Inputs    map[string]string    `json:"inputs,omitempty"`
	Tables    map[string]TableData `json:"tables,omitempty"`
}

var (
	anySelectPat = regexp.MustCompile(`(?s)<select[^>]*name="([^"]+)"[^>]*>(.*?)</select>`)
	anyInputPat  = regexp.MustCompile(`(?i)<input\b([^>]+)>`)
	attrPat      = regexp.MustCompile(`([a-zA-Z0-9_-]+)\s*=\s*"([^"]*)"`)
	anyTablePat  = regexp.MustCompile(`(?s)<table[^>]*\bid="([^"]+)"[^>]*>(.*?)</table>`)
	selectedPat  = regexp.MustCompile(`(?i)\bselected\b`)
)

// BuildPagePath builds "/<aspx>?xh=<user>&gnmkdm=<code...>" for a classic jwxt page.
func (c *Client) BuildPagePath(aspx, gnmkdm string) string {
	path := "/" + strings.TrimLeft(aspx, "/")
	if strings.Contains(path, "?") {
		return path
	}
	q := url.Values{}
	if c.username != "" {
		q.Set("xh", c.username)
	}
	if gnmkdm != "" {
		q.Set("gnmkdm", gnmkdm)
	}
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}

// Snapshot fetches path via GET and parses its ViewState, selects, inputs, and tables.
func (c *Client) Snapshot(ctx context.Context, path string) (PageSnapshot, error) {
	doc, err := c.GetPage(ctx, path)
	if err != nil {
		return PageSnapshot{}, err
	}
	return ParseSnapshot(path, doc), nil
}

// PostBack submits a GBK-encoded WebForms PostBack built from current's form
// state plus UTF-8 overrides.
func (c *Client) PostBack(ctx context.Context, path string, current *PageSnapshot, overrides url.Values) (PageSnapshot, error) {
	var snap PageSnapshot
	if current != nil {
		snap = *current
	} else {
		var err error
		snap, err = c.Snapshot(ctx, path)
		if err != nil {
			return PageSnapshot{}, err
		}
	}

	form := url.Values{}
	if snap.ViewState != "" {
		form.Set("__VIEWSTATE", snap.ViewState)
	}
	for k, v := range snap.Inputs {
		form.Set(k, encodeGBK(v))
	}
	for selName, opts := range snap.Selects {
		if len(opts) > 0 {
			chosen := opts[0].Value
			for _, o := range opts {
				if o.Selected {
					chosen = o.Value
					break
				}
			}
			form.Set(selName, encodeGBK(chosen))
		}
	}
	for k, vs := range overrides {
		encoded := make([]string, len(vs))
		for i, v := range vs {
			encoded[i] = encodeGBK(v)
		}
		form[k] = encoded
	}

	doc, err := c.PostForm(ctx, path, form, path)
	if err != nil {
		return PageSnapshot{}, err
	}
	return ParseSnapshot(path, doc), nil
}

// ParseSnapshot extracts selects, inputs, ViewState, and all <table id="..."> blocks.
func ParseSnapshot(path, doc string) PageSnapshot {
	snap := PageSnapshot{
		Path:      path,
		ViewState: extractViewState(doc),
		Selects:   map[string][]Option{},
		Inputs:    map[string]string{},
		Tables:    map[string]TableData{},
	}

	for _, sm := range anySelectPat.FindAllStringSubmatch(doc, -1) {
		name := sm[1]
		var opts []Option
		for _, om := range optPattern.FindAllStringSubmatch(sm[2], -1) {
			val := strings.TrimSpace(html.UnescapeString(om[1]))
			lbl := cleanCell(om[2])
			opts = append(opts, Option{
				Value:    val,
				Label:    lbl,
				Selected: selectedPat.MatchString(om[0]),
			})
		}
		snap.Selects[name] = opts
	}

	for _, im := range anyInputPat.FindAllStringSubmatch(doc, -1) {
		attrs := map[string]string{}
		for _, am := range attrPat.FindAllStringSubmatch(im[1], -1) {
			attrs[strings.ToLower(am[1])] = html.UnescapeString(am[2])
		}
		name := attrs["name"]
		typ := strings.ToLower(attrs["type"])
		if name == "" || name == "__VIEWSTATE" || name == "__VIEWSTATEGENERATOR" || name == "__EVENTVALIDATION" {
			continue
		}
		if typ == "submit" || typ == "button" || typ == "image" || typ == "reset" {
			continue
		}
		if _, exists := snap.Inputs[name]; !exists {
			snap.Inputs[name] = attrs["value"]
		}
	}

	for _, tm := range anyTablePat.FindAllStringSubmatch(doc, -1) {
		id := tm[1]
		td := TableData{ID: id}
		for rIdx, tr := range trPattern.FindAllStringSubmatch(tm[2], -1) {
			cells := tdPattern.FindAllStringSubmatch(tr[1], -1)
			if len(cells) == 0 {
				continue
			}
			row := make([]string, len(cells))
			for i, c := range cells {
				row[i] = cleanCell(c[1])
			}
			if rIdx == 0 {
				td.Headers = row
				continue
			}
			td.Rows = append(td.Rows, row)
			if len(td.Headers) > 0 && len(row) == len(td.Headers) {
				rec := make(map[string]string, len(row))
				for i, h := range td.Headers {
					if h != "" {
						rec[h] = row[i]
					}
				}
				td.Records = append(td.Records, rec)
			}
		}
		if len(td.Headers) > 0 || len(td.Rows) > 0 {
			snap.Tables[id] = td
		}
	}

	return snap
}
