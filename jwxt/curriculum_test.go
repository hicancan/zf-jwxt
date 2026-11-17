package jwxt

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseFiltersAndGrid(t *testing.T) {
	sample := `
<select name="nj">
  <option value="2024">2024</option>
  <option value="2023">2023</option>
</select>
<select name="xymc">
  <option value="01">通信与信息工程学院</option>
</select>
<select name="zymc">
  <option value="0101">通信工程</option>
</select>
<table id="DBGrid">
  <tr><td>课程代码</td><td>课程名称</td><td>学分</td><td>周学时</td><td>考核方式</td><td>课程性质</td><td>课程类别</td><td>开课学期</td><td>专业方向</td><td>课组编号</td><td>周次</td></tr>
  <tr><td>B0101011S</td><td>高等数学A（上）</td><td>5.0</td><td>5.0-0.0</td><td>考试</td><td>必修课</td><td>公共基础课</td><td>1</td><td></td><td></td><td>01-16</td></tr>
</table>`

	filters := parseFilters(sample)
	if len(filters.Grades) != 2 || filters.Grades[0].Value != "2024" {
		t.Fatalf("unexpected grades: %+v", filters.Grades)
	}
	if len(filters.Colleges) != 1 || filters.Colleges[0].Label != "通信与信息工程学院" {
		t.Fatalf("unexpected colleges: %+v", filters.Colleges)
	}
	if len(filters.Majors) != 1 || filters.Majors[0].Value != "0101" {
		t.Fatalf("unexpected majors: %+v", filters.Majors)
	}

	rows := parseGrid(sample)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Code != "B0101011S" || rows[0].Name != "高等数学A（上）" || rows[0].Credits != "5.0" || rows[0].Term != "1" {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
}

func TestGBKRoundTrip(t *testing.T) {
	src := "学生全部"
	enc := encodeGBK(src)
	if enc == src {
		t.Fatal("expected GBK bytes to differ from UTF-8")
	}
	if got := decodeGBK([]byte(enc)); got != src {
		t.Fatalf("got %q, want %q", got, src)
	}
}

func curriculumPage(grade, college, major, state, code string, last bool) string {
	option := func(value, chosen string) string {
		selected := ""
		if value == chosen {
			selected = ` selected="selected"`
		}
		return fmt.Sprintf(`<option value="%s"%s>%s</option>`, value, selected, value)
	}
	disabled := ""
	if last {
		disabled = ` disabled="disabled"`
	}
	return fmt.Sprintf(`<input name="__VIEWSTATE" value="%s">
<select name="nj">%s%s%s</select>
<select name="xymc">%s%s</select>
<select name="zymc">%s</select>
<table id="DBGrid"><tr><td>课程代码</td><td>课程名称</td><td>学分</td><td>学时</td><td>考核方式</td><td>课程性质</td><td>课程类别</td><td>开课学期</td></tr>
<tr><td>%s</td><td>测试课程</td><td>2</td><td>32</td><td>考试</td><td>必修</td><td>基础</td><td>1</td></tr></table>
<input id="dpDBGrid_btnNextPage"%s>`, state, option("2023", grade), option("2024", grade), option("2026", grade), option("03", college), option("14", college), option(major, major), code, disabled)
}

func TestCurriculumPageSelection(t *testing.T) {
	for _, test := range []struct {
		name, grade, college, target, wantGrade, wantCollege string
	}{
		{"defaults", "", "", "", "2024", "14"},
		{"grade only", "2026", "", "nj", "2026", "14"},
		{"college only", "", "03", "xymc", "2024", "03"},
		{"grade and college", "2026", "03", "xymc", "2026", "03"},
		{"explicit current selection", "2024", "14", "xymc", "2024", "14"},
	} {
		t.Run(test.name, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/pyjh.aspx" || r.URL.Query().Get("xh") != "test-student" || r.URL.Query().Get("gnmkdm") != "N121607" {
					t.Errorf("unexpected curriculum path: %s", r.URL)
				}
				if r.Method == http.MethodGet {
					fmt.Fprint(w, encodeGBK(curriculumPage("2024", "14", "1401", "first", "initial", true)))
					return
				}
				posts++
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				for field, want := range map[string]string{"__EVENTTARGET": test.target, "__VIEWSTATE": "first", "nj": test.wantGrade, "xymc": test.wantCollege, "zymc": "", "xq": encodeGBK("全部"), "kcxz": encodeGBK("全部")} {
					if got := r.PostForm.Get(field); got != want {
						t.Errorf("%s=%q, want %q", field, got, want)
					}
				}
				major := test.wantCollege + test.wantGrade
				fmt.Fprint(w, encodeGBK(curriculumPage(test.wantGrade, test.wantCollege, major, "selected", "selected", true)))
			}))
			defer server.Close()
			client, err := New(server.URL, "", "test-student", "")
			if err != nil {
				t.Fatal(err)
			}
			filters, rows, err := client.CurriculumPage(context.Background(), test.grade, test.college)
			if err != nil {
				t.Fatal(err)
			}
			wantPosts, wantMajor, wantCode := 1, test.wantCollege+test.wantGrade, "selected"
			if test.target == "" {
				wantPosts, wantMajor, wantCode = 0, "1401", "initial"
			}
			if posts != wantPosts || selectedOption(filters.Grades) != test.wantGrade || selectedOption(filters.Colleges) != test.wantCollege || len(filters.Majors) != 1 || filters.Majors[0].Value != wantMajor || !filters.Majors[0].Selected || len(rows) != 1 || rows[0].Code != wantCode {
				t.Fatalf("posts=%d filters=%+v rows=%+v", posts, filters, rows)
			}
		})
	}
}

func TestCurriculumMajorPostBackAndPagination(t *testing.T) {
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, encodeGBK(curriculumPage("2024", "14", "1401", "first", "initial", false)))
			return
		}
		posts++
		_ = r.ParseForm()
		targets := []string{"xymc", "zymc", "dpDBGrid:btnNextPage"}
		states := []string{"first", "college", "major"}
		if posts > len(targets) {
			t.Error("pagination continued after the last page")
			http.Error(w, "unexpected page", http.StatusBadRequest)
			return
		}
		major := ""
		if posts > 1 {
			major = "0301"
		}
		for field, want := range map[string]string{"__EVENTTARGET": targets[posts-1], "__VIEWSTATE": states[posts-1], "nj": "2026", "xymc": "03", "zymc": major} {
			if got := r.PostForm.Get(field); got != want {
				t.Errorf("post %d %s=%q, want %q", posts, field, got, want)
			}
		}
		state, code := "college", "college-row"
		if posts == 2 {
			state, code = "major", "course-1"
		} else if posts == 3 {
			state, code = "last", "course-2"
		}
		fmt.Fprint(w, encodeGBK(curriculumPage("2026", "03", "0301", state, code, posts == 3)))
	}))
	defer server.Close()
	client, err := New(server.URL, "", "test-student", "")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := client.CurriculumByMajor(context.Background(), "2026", "03", "0301", true)
	if err != nil {
		t.Fatal(err)
	}
	if posts != 3 || len(rows) != 2 || rows[0].Code != "course-1" || rows[1].Code != "course-2" {
		t.Fatalf("posts=%d rows=%+v", posts, rows)
	}
}

func TestCurriculumSelectionSessionExpiry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			http.Redirect(w, r, "/default2.aspx", http.StatusFound)
			return
		}
		fmt.Fprint(w, encodeGBK(curriculumPage("2024", "14", "1401", "first", "initial", true)))
	}))
	defer server.Close()
	client, _ := New(server.URL, "", "test-student", "")
	if _, _, err := client.CurriculumPage(context.Background(), "2026", "03"); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("session error was lost: %v", err)
	}
}
