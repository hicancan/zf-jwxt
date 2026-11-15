package jwglxt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestInjectCookiesAndPagedQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ck, err := r.Cookie("JSESSIONID")
		if err != nil || ck.Value != "abc" {
			http.Error(w, "unauthenticated", http.StatusFound)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("queryModel.showCount") != "25" || r.FormValue("queryModel.currentPage") != "2" {
			t.Fatalf("unexpected paging params: %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"currentPage": 2,
			"totalPage":   3,
			"totalCount":  60,
			"items": []map[string]any{
				{"kch": "B01", "custom_extra_field": "kept_verbatim"},
			},
		})
	}))
	defer srv.Close()

	c, err := New(srv.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	c.InjectCookieHeader("JSESSIONID=abc; route=r1")

	res, err := c.PagedQuery(context.Background(), "/cjcx/cjcx_cxXsgrcj.html", url.Values{}, 25, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.CurrentPage != 2 || res.TotalCount != 60 || len(res.Items) != 1 {
		t.Fatalf("unexpected paged result: %+v", res)
	}
	var row map[string]any
	if err := json.Unmarshal(res.Items[0], &row); err != nil {
		t.Fatal(err)
	}
	if row["custom_extra_field"] != "kept_verbatim" {
		t.Fatalf("expected raw extra field preserved, got %+v", row)
	}
}
