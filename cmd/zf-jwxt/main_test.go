package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hicancan/zf-jwxt/jwxt"
)

func TestCurriculumPartialFiltersCLI(t *testing.T) {
	t.Setenv("ZF_USERNAME", "test-student")
	t.Setenv("ZF_PASSWORD", "")
	for _, test := range []struct {
		name, grade, college, target, wantGrade, wantCollege string
	}{
		{"grade", "2026", "", "nj", "2026", "14"},
		{"college", "", "03", "xymc", "2024", "03"},
		{"grade and college", "2026", "03", "xymc", "2026", "03"},
	} {
		t.Run(test.name, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					fmt.Fprint(w, `<input name="__VIEWSTATE" value="first"><select name="nj"><option value="2024" selected="selected">2024</option></select><select name="xymc"><option value="14" selected="selected">14</option></select><select name="zymc"><option value="1401">1401</option></select>`)
					return
				}
				posts++
				_ = r.ParseForm()
				if r.PostForm.Get("__EVENTTARGET") != test.target || r.PostForm.Get("nj") != test.wantGrade || r.PostForm.Get("xymc") != test.wantCollege || r.PostForm.Get("__VIEWSTATE") != "first" {
					t.Errorf("flags did not reach the WebForms selection: target=%q grade=%q college=%q", r.PostForm.Get("__EVENTTARGET"), r.PostForm.Get("nj"), r.PostForm.Get("xymc"))
				}
				fmt.Fprintf(w, `<select name="nj"><option value="%s" selected="selected">%s</option></select><select name="xymc"><option value="%s" selected="selected">%s</option></select><select name="zymc"><option value="new-major">new-major</option></select>`, test.wantGrade, test.wantGrade, test.wantCollege, test.wantCollege)
			}))
			defer server.Close()
			out := filepath.Join(t.TempDir(), "curriculum.json")
			args := []string{"curriculum", "-base", server.URL, "-cookie", "test=fixture", "-out", out}
			if test.grade != "" {
				args = append(args, "-grade", test.grade)
			}
			if test.college != "" {
				args = append(args, "-college", test.college)
			}
			dispatchJwxt(args)
			body, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var response struct {
				Filters jwxt.CurriculumFilters `json:"filters"`
			}
			if err := json.Unmarshal(body, &response); err != nil {
				t.Fatal(err)
			}
			if posts != 1 || len(response.Filters.Majors) != 1 || response.Filters.Majors[0].Value != "new-major" {
				t.Fatalf("partial filters returned the initial majors: posts=%d filters=%+v", posts, response.Filters)
			}
		})
	}
}
