package jwglxt

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Term identifies a ZhengFang school year and term (1, 2 or 3).
type Term struct {
	Year   int `json:"year"`
	Number int `json:"number"`
}

func (t Term) String() string { return fmt.Sprintf("%d-%d-%d", t.Year, t.Year+1, t.Number) }
func (t Term) xnm() string    { return strconv.Itoa(t.Year) }
func (t Term) xqm() string    { return termCodes[t.Number] }

var termCodes = map[int]string{1: "3", 2: "12", 3: "16"}

// ParseTerm parses "2026-2027-1" into a Term.
func ParseTerm(s string) (Term, error) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) != 3 {
		return Term{}, fmt.Errorf("jwglxt: invalid term %q", s)
	}
	y1, err1 := strconv.Atoi(parts[0])
	y2, err2 := strconv.Atoi(parts[1])
	n, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil || y2 != y1+1 || termCodes[n] == "" {
		return Term{}, fmt.Errorf("jwglxt: invalid term %q", s)
	}
	return Term{Year: y1, Number: n}, nil
}

var selectedOption = regexp.MustCompile(`<option[^>]*selected[^>]*value="([^"]*)"|value="([^"]*)"[^>]*selected`)

// CurrentTerm reads the server's active term from the timetable page.
func (c *Client) CurrentTerm(ctx context.Context) (Term, error) {
	page, err := c.GetPage(ctx, "/kbdy/bjkbdy_cxBjkbdyIndex.html?gnmkdm=N214505&layout=default")
	if err != nil {
		return Term{}, err
	}
	values := map[string]string{}
	for _, name := range []string{"xnm", "xqm"} {
		idx := strings.Index(page, `name="`+name+`"`)
		if idx < 0 {
			return Term{}, fmt.Errorf("jwglxt: term select %s not found", name)
		}
		end := strings.Index(page[idx:], "</select>")
		if end < 0 {
			return Term{}, fmt.Errorf("jwglxt: unclosed select %s", name)
		}
		m := selectedOption.FindStringSubmatch(page[idx : idx+end])
		if m == nil {
			return Term{}, fmt.Errorf("jwglxt: no selected option for %s", name)
		}
		values[name] = m[1] + m[2]
	}
	year, err := strconv.Atoi(values["xnm"])
	if err != nil {
		return Term{}, fmt.Errorf("jwglxt: invalid xnm %q", values["xnm"])
	}
	for number, code := range termCodes {
		if code == values["xqm"] {
			return Term{Year: year, Number: number}, nil
		}
	}
	return Term{}, fmt.Errorf("jwglxt: unknown term code %q", values["xqm"])
}

// ApplyTerm sets xnm/xqm on form if t is non-zero.
func ApplyTerm(form url.Values, t Term) {
	if t.Year > 0 {
		form.Set("xnm", t.xnm())
	}
	if code := t.xqm(); code != "" {
		form.Set("xqm", code)
	}
}
