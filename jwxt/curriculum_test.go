package jwxt

import "testing"

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
