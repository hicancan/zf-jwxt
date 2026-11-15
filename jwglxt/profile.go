package jwglxt

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

var (
	formGroupPat   = regexp.MustCompile(`(?s)<div[^>]*class="[^"]*form-group[^"]*"[^>]*>(.*?)</div>\s*</div>`)
	labelPat       = regexp.MustCompile(`(?s)<label[^>]*>(.*?)</label>`)
	staticValuePat = regexp.MustCompile(`(?s)<p[^>]*class="[^"]*form-control-static[^"]*"[^>]*>(.*?)</p>`)
	profileRowPat  = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	profileCellPat = regexp.MustCompile(`(?s)<t[hd][^>]*>(.*?)</t[hd]>`)
	profileTagPat  = regexp.MustCompile(`<[^>]+>`)
)

// Profile fetches xsgrxxwh_cxXsgrxx.html and extracts all label/value pairs
// from Bootstrap form-groups and fallback HTML tables.
func (c *Client) Profile(ctx context.Context) (map[string]string, error) {
	htmlStr, err := c.GetPage(ctx, "/xsxxxggl/xsgrxxwh_cxXsgrxx.html?doType=query&gnmkdm=N100801")
	if err != nil {
		return nil, err
	}
	out := map[string]string{}

	// Bootstrap form-group pairs: <label>...</label> ... <p class="form-control-static">...</p>
	labels := labelPat.FindAllStringSubmatchIndex(htmlStr, -1)
	statics := staticValuePat.FindAllStringSubmatchIndex(htmlStr, -1)
	for _, li := range labels {
		k := strings.TrimRight(cleanHTMLCell(htmlStr[li[2]:li[3]]), "：:")
		if k == "" {
			continue
		}
		// Find the closest <p class="form-control-static"> immediately following this label (within 600 bytes).
		for _, si := range statics {
			if si[0] > li[1] && si[0]-li[1] < 600 {
				v := cleanHTMLCell(htmlStr[si[2]:si[3]])
				if v != "" {
					out[k] = v
				}
				break
			}
		}
	}

	// Fallback for table-based layouts
	for _, tr := range profileRowPat.FindAllStringSubmatch(htmlStr, -1) {
		cells := profileCellPat.FindAllStringSubmatch(tr[1], -1)
		for i := 0; i+1 < len(cells); i += 2 {
			k := strings.TrimRight(cleanHTMLCell(cells[i][1]), "：:")
			v := cleanHTMLCell(cells[i+1][1])
			if k != "" && v != "" {
				out[k] = v
			}
		}
	}
	return out, nil
}

// ProfileSemesters queries /xsxxxggl/xsxxwh_cxXqm.html (terms for a year).
func (c *Client) ProfileSemesters(ctx context.Context, xnm string) (json.RawMessage, error) {
	return c.PostRaw(ctx, "/xsxxxggl/xsxxwh_cxXqm.html?gnmkdm=N100801", url.Values{"xnm": {xnm}})
}

func cleanHTMLCell(s string) string {
	s = profileTagPat.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return strings.Join(strings.Fields(s), " ")
}
