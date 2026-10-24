package capture

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/hicancan/zf-jwxt/zf"
)

// fakeServer imitates the parts of jwglxt the capture uses. The first class
// query after login answers with an expired session once.
func fakeServer(t *testing.T, password string) (*httptest.Server, *int32) {
	key, _ := rsa.GenerateKey(rand.Reader, 1024)
	var logins int32
	var expired atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/xtgl/login_slogin.html", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `<input type="hidden" id="csrftoken" name="csrftoken" value="tok"/>`)
			return
		}
		_ = r.ParseForm()
		raw, _ := base64.StdEncoding.DecodeString(r.Form.Get("mm"))
		plain, err := rsa.DecryptPKCS1v15(nil, key, raw)
		if err != nil || string(plain) != password || r.Form.Get("csrftoken") != "tok" {
			fmt.Fprint(w, `<p id="tips" class="bg_danger sl_danger">用户名或密码不正确</p>`)
			return
		}
		atomic.AddInt32(&logins, 1)
		http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: fmt.Sprint(atomic.LoadInt32(&logins)), Path: "/"})
		http.Redirect(w, r, "/xtgl/login_slogin.html", http.StatusFound)
	})
	mux.HandleFunc("/xtgl/login_getPublicKey.html", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"modulus":  base64.StdEncoding.EncodeToString(key.N.Bytes()),
			"exponent": base64.StdEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		})
	})
	authed := func(w http.ResponseWriter, r *http.Request) bool {
		if _, err := r.Cookie("JSESSIONID"); err != nil {
			http.Redirect(w, r, "/xtgl/login_slogin.html", http.StatusFound)
			return false
		}
		w.Header().Set("Content-Type", "application/json;charset=utf-8")
		return true
	}
	mux.HandleFunc("/kbdy/bjkbdy_cxBjkbdyTjkbList.html", func(w http.ResponseWriter, r *http.Request) {
		if authed(w, r) {
			fmt.Fprint(w, `{"items":[{"bh_id":"B2","queryModel":{"x":1}},{"bh_id":"B1"}],"totalResult":2}`)
		}
	})
	mux.HandleFunc("/kbdy/bjkbdy_cxRjc.html", func(w http.ResponseWriter, r *http.Request) {
		if authed(w, r) {
			fmt.Fprint(w, `[{"jcmc":"1","qssj":"08:00","jssj":"08:45"}]`)
		}
	})
	mux.HandleFunc("/kbdy/bjkbdy_cxBjKb.html", func(w http.ResponseWriter, r *http.Request) {
		if !expired.Swap(true) {
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "", Path: "/", MaxAge: -1})
			http.Redirect(w, r, "/xtgl/login_slogin.html", http.StatusFound)
			return
		}
		if authed(w, r) {
			_ = r.ParseForm()
			fmt.Fprintf(w, `{"kbList":[{"kcmc":"课程-%s","sjh":"13800000000"}]}`, r.Form.Get("bh_id"))
		}
	})
	return httptest.NewServer(mux), &logins
}

func TestTimetablesCapturesResumesAndRelogs(t *testing.T) {
	srv, logins := fakeServer(t, "secret")
	defer srv.Close()
	dir := t.TempDir()
	term := zf.Term{Year: 2026, Number: 1}

	client, _ := zf.New(srv.URL, "", "u", "secret")
	if err := client.Login(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := Timetables(context.Background(), client, term, Options{Dir: dir, Workers: 1, MaxRelogins: 2})
	if err != nil || res.Fetched != 2 || len(res.Failed) != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if *logins != 2 {
		t.Fatalf("want one re-login after the expired session, got %d logins", *logins)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "2026-2027-1", "classes", "B1.json"))
	var tt map[string]any
	_ = json.Unmarshal(data, &tt)
	row := tt["kbList"].([]any)[0].(map[string]any)
	if row["kcmc"] != "课程-B1" || row["sjh"] != nil {
		t.Fatalf("unexpected class file %s", data)
	}
	cat, _ := os.ReadFile(filepath.Join(dir, "2026-2027-1", "catalog.json"))
	var entries []map[string]any
	_ = json.Unmarshal(cat, &entries)
	if len(entries) != 2 || entries[0]["bh_id"] != "B1" || entries[1]["queryModel"] != nil {
		t.Fatalf("catalog not sorted or not scrubbed: %s", cat)
	}

	again, err := Timetables(context.Background(), client, term, Options{Dir: dir, Workers: 2, MaxRelogins: 2})
	if err != nil || again.Skipped != 2 || again.Fetched != 0 {
		t.Fatalf("resume: %+v %v", again, err)
	}
}

func TestRejectedLoginIsReported(t *testing.T) {
	srv, logins := fakeServer(t, "secret")
	defer srv.Close()
	client, _ := zf.New(srv.URL, "", "u", "wrong")
	err := client.Login(context.Background())
	if err == nil || *logins != 0 {
		t.Fatalf("want rejection, got %v", err)
	}
}
