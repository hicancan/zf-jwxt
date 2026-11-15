// Command zf-jwxt is a school-agnostic, full-field CLI for both modern
// ZhengFang (jwglxt) and classic ASP.NET ZhengFang (jwxt) systems.
//
//	zf-jwxt jwglxt <subcommand> [flags]
//	zf-jwxt jwxt   <subcommand> [flags]
//	zf-jwxt jwglxt batch [flags] < requests.jsonl > answers.jsonl
//
// Authentication supports direct login (ZF_USERNAME / ZF_PASSWORD, plus
// -captcha-model or -captcha-cmd for classic jwxt) or external session cookie
// injection via -cookie / -cookie-cmd (e.g. `cas-sso cookies -url ...`).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/hicancan/zf-jwxt/captcha"
	"github.com/hicancan/zf-jwxt/jwglxt"
	"github.com/hicancan/zf-jwxt/jwxt"
)

var version = "dev"

type kvFlags url.Values

func (k kvFlags) String() string { return url.Values(k).Encode() }
func (k kvFlags) Set(v string) error {
	key, val, ok := strings.Cut(v, "=")
	if !ok || key == "" {
		return fmt.Errorf("expected key=value, got %q", v)
	}
	url.Values(k).Add(key, val)
	return nil
}

type authFlags struct {
	base         string
	proxy        string
	cookie       string
	cookieCmd    string
	captchaModel string
	captchaCmd   string
}

func bindAuthFlags(fs *flag.FlagSet, defaultBase string, withCaptcha bool) *authFlags {
	a := &authFlags{}
	fs.StringVar(&a.base, "base", defaultBase, "system base URL")
	fs.StringVar(&a.proxy, "proxy", "", "http:// or socks5:// proxy")
	fs.StringVar(&a.cookie, "cookie", "", "raw Cookie header string (skips login)")
	fs.StringVar(&a.cookieCmd, "cookie-cmd", "", "external command returning JSON cookies or Cookie header")
	if withCaptcha {
		fs.StringVar(&a.captchaModel, "captcha-model", "", "MCNN v2 captcha model (.bin)")
		fs.StringVar(&a.captchaCmd, "captcha-cmd", "", "external command reading captcha bytes on stdin")
	}
	return a
}

func main() {
	log.SetFlags(log.Ltime)
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "jwglxt":
		dispatchJwglxt(os.Args[2:])
	case "jwxt":
		dispatchJwxt(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println("zf-jwxt", version)
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: zf-jwxt jwglxt <"+jwglxtCommands+"> [flags]")
	fmt.Fprintln(os.Stderr, "       zf-jwxt jwxt   <curriculum|grades|level-exams|makeup-exams|schedule|class-schedule|teacher-schedule|time-schedule|rooms|textbooks|profile|competitions|thesis-topics|teaching-plans|page> [flags]")
	os.Exit(2)
}

func readExternalCookies(ctx context.Context, cmdLine string) ([]jwglxt.Cookie, string, error) {
	parts := strings.Fields(cmdLine)
	if len(parts) == 0 {
		return nil, "", errors.New("empty -cookie-cmd")
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("cookie-cmd %q failed: %w", cmdLine, err)
	}
	trimmed := strings.TrimSpace(string(out))
	var envelope struct {
		Cookies []jwglxt.Cookie `json:"cookies"`
		Cookie  string          `json:"cookie"`
	}
	if strings.HasPrefix(trimmed, "{") && json.Unmarshal([]byte(trimmed), &envelope) == nil {
		if len(envelope.Cookies) > 0 {
			return envelope.Cookies, "", nil
		}
		if envelope.Cookie != "" {
			return nil, envelope.Cookie, nil
		}
	}
	var list []jwglxt.Cookie
	if strings.HasPrefix(trimmed, "[") && json.Unmarshal([]byte(trimmed), &list) == nil {
		return list, "", nil
	}
	return nil, trimmed, nil
}

func openJwglxt(ctx context.Context, a *authFlags) *jwglxt.Client {
	if a.base == "" {
		log.Fatal("need -base")
	}
	user, pass := os.Getenv("ZF_USERNAME"), os.Getenv("ZF_PASSWORD")
	c, err := jwglxt.New(a.base, a.proxy, user, pass)
	if err != nil {
		log.Fatal(err)
	}
	if a.cookie != "" {
		c.InjectCookieHeader(a.cookie)
		return c
	}
	if a.cookieCmd != "" {
		cks, hdr, err := readExternalCookies(ctx, a.cookieCmd)
		if err != nil {
			log.Fatal(err)
		}
		if len(cks) > 0 {
			c.InjectCookies(cks)
		} else {
			c.InjectCookieHeader(hdr)
		}
		return c
	}
	if user == "" || pass == "" {
		log.Fatal("need ZF_USERNAME and ZF_PASSWORD, or -cookie / -cookie-cmd")
	}
	if err := c.Login(ctx); err != nil {
		log.Fatal(err)
	}
	return c
}

func openJwxt(ctx context.Context, a *authFlags) *jwxt.Client {
	if a.base == "" {
		log.Fatal("need -base")
	}
	user, pass := os.Getenv("ZF_USERNAME"), os.Getenv("ZF_PASSWORD")
	c, err := jwxt.New(a.base, a.proxy, user, pass)
	if err != nil {
		log.Fatal(err)
	}
	if a.cookie != "" {
		c.InjectCookieHeader(a.cookie)
		return c
	}
	if a.cookieCmd != "" {
		cks, hdr, err := readExternalCookies(ctx, a.cookieCmd)
		if err != nil {
			log.Fatal(err)
		}
		if len(cks) > 0 {
			out := make([]jwxt.Cookie, len(cks))
			for i, ck := range cks {
				out[i] = jwxt.Cookie{Name: ck.Name, Value: ck.Value, Domain: ck.Domain, Path: ck.Path}
			}
			c.InjectCookies(out)
		} else {
			c.InjectCookieHeader(hdr)
		}
		return c
	}
	if user == "" || pass == "" {
		log.Fatal("need ZF_USERNAME and ZF_PASSWORD, or -cookie / -cookie-cmd")
	}
	solver, err := buildCaptchaSolver(a.captchaModel, a.captchaCmd)
	if err != nil {
		log.Fatal(err)
	}
	var loginErr error
	for attempt := 1; attempt <= 4; attempt++ {
		loginErr = c.Login(ctx, solver)
		if loginErr == nil || !errors.Is(loginErr, jwxt.ErrCaptchaRejected) {
			break
		}
		log.Printf("captcha rejected (attempt %d/4), retrying", attempt)
	}
	if loginErr != nil {
		log.Fatal(loginErr)
	}
	return c
}

func buildCaptchaSolver(modelPath, cmdLine string) (jwxt.CaptchaSolver, error) {
	if modelPath != "" {
		rec, err := captcha.Load(modelPath)
		if err != nil {
			return nil, err
		}
		return func(_ context.Context, img []byte) (string, error) {
			return rec.Recognize(img)
		}, nil
	}
	if cmdLine != "" {
		parts := strings.Fields(cmdLine)
		return func(ctx context.Context, img []byte) (string, error) {
			cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
			cmd.Stdin = bytes.NewReader(img)
			out, err := cmd.Output()
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(string(out)), nil
		}, nil
	}
	return nil, errors.New("need -cookie, -cookie-cmd, -captcha-model, or -captcha-cmd for classic jwxt")
}

func writeJSONOut(outFile string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	data = append(data, '\n')
	if outFile != "" {
		if err := os.WriteFile(outFile, data, 0o644); err != nil {
			log.Fatal(err)
		}
		return
	}
	_, _ = os.Stdout.Write(data)
}

// jwglxtRequest is one question to jwglxt, from flags or from a line of a batch.
type jwglxtRequest struct {
	ID      string            `json:"id,omitempty"`
	Command string            `json:"command"`
	Term    string            `json:"term,omitempty"`
	Kind    string            `json:"kind,omitempty"`
	Path    string            `json:"path,omitempty"`
	Limit   int               `json:"limit,omitempty"`
	Page    int               `json:"page,omitempty"`
	All     bool              `json:"all,omitempty"`
	Week    int               `json:"week,omitempty"`
	Weekday int               `json:"weekday,omitempty"`
	Period  int               `json:"period,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
}

func (r jwglxtRequest) values() url.Values {
	v := url.Values{}
	for k, val := range r.Params {
		v.Set(k, val)
	}
	return v
}

func (r jwglxtRequest) term() (jwglxt.Term, error) {
	if r.Term == "" {
		return jwglxt.Term{}, nil
	}
	return jwglxt.ParseTerm(r.Term)
}

const jwglxtCommands = "term|dict|profile|curriculum|exec-plan|schedule|class-schedules|class-timetable|periods|weekly-schedule|rooms|free-rooms|grades|credit-stats|exams|unscheduled-exams|enrollments|selected-courses|textbooks|evaluations|raw|batch"

// runJwglxt answers one request on a signed-in client.
func runJwglxt(ctx context.Context, c *jwglxt.Client, r jwglxtRequest) (any, error) {
	t, err := r.term()
	if err != nil {
		return nil, err
	}
	pv, limit, page := r.values(), r.Limit, r.Page
	if limit == 0 {
		limit = 100
	}
	if page == 0 {
		page = 1
	}
	all := func(path string) (any, error) {
		items, err := c.PagedQueryAll(ctx, path, pv, limit, 0)
		return map[string]any{"total": len(items), "items": items}, err
	}
	switch r.Command {
	case "term":
		return c.CurrentTerm(ctx)
	case "dict":
		if r.Kind == "" {
			return nil, errors.New("dict needs a kind")
		}
		return c.Dict(ctx, jwglxt.DictKind(r.Kind), pv, limit, page, r.All)
	case "profile":
		return c.Profile(ctx)
	case "curriculum":
		if r.All {
			return all("/xspyfagl/xspyfaxxwh_cxGrpyfakcxxIndex.html?doType=query&gnmkdm=N1532")
		}
		return c.CurriculumCourses(ctx, pv, limit, page)
	case "exec-plan":
		if r.All {
			return all("/jxzxjhgl/jxzxjhxxwh_cxXsyxxfwhIndex.html?doType=query&gnmkdm=N2511")
		}
		return c.ExecPlanCourses(ctx, pv, limit, page)
	case "schedule":
		return c.PersonalSchedule(ctx, t, pv)
	case "class-schedules":
		if r.All {
			return c.Catalog(ctx, t)
		}
		return c.ClassSchedules(ctx, t, pv, limit, page)
	case "class-timetable":
		entry := map[string]any{}
		for k, v := range r.Params {
			entry[k] = v
		}
		return c.ClassTimetable(ctx, t, entry)
	case "periods":
		return c.Periods(ctx, t)
	case "weekly-schedule":
		return c.WeeklySchedule(ctx, t, pv)
	case "rooms":
		return c.RoomsQuery(ctx, pv, limit, page)
	case "free-rooms":
		if r.Week < 1 || r.Weekday < 1 || r.Weekday > 7 || r.Period < 1 {
			return nil, errors.New("free-rooms needs a week, a weekday 1-7 and a period")
		}
		return c.FreeRooms(ctx, t, r.Week, r.Weekday, r.Period)
	case "grades":
		return c.Grades(ctx, t, pv, limit, page)
	case "credit-stats":
		return c.CreditStats(ctx, pv)
	case "exams":
		return c.Exams(ctx, t, pv, limit, page)
	case "unscheduled-exams":
		return c.UnscheduledExams(ctx, t, pv, limit, page)
	case "enrollments":
		return c.Enrollments(ctx, t, pv, limit, page)
	case "selected-courses":
		return c.SelectedCourses(ctx, t, pv)
	case "textbooks":
		return c.Textbooks(ctx, t, pv, limit, page)
	case "evaluations":
		return c.Evaluations(ctx, pv, limit, page)
	case "raw":
		if r.Path == "" {
			return nil, errors.New("raw needs a path")
		}
		if r.All {
			return all(r.Path)
		}
		return c.PostRaw(ctx, r.Path, pv)
	}
	return nil, fmt.Errorf("unknown jwglxt command %q (%s)", r.Command, jwglxtCommands)
}

func dispatchJwglxt(args []string) {
	if len(args) < 1 {
		usage()
	}
	r := jwglxtRequest{Command: args[0]}
	fs := flag.NewFlagSet("jwglxt "+r.Command, flag.ExitOnError)
	auth := bindAuthFlags(fs, "", false)
	fs.StringVar(&r.Term, "term", "", "term YYYY-YYYY-N (the current one when left out)")
	fs.StringVar(&r.Kind, "kind", "", "dictionary kind (colleges,departments,grades,majors,classes,regions,campuses,buildings,weeks,periods,timeslots,exam-names,orgs)")
	fs.StringVar(&r.Path, "path", "", "endpoint path (for raw)")
	fs.IntVar(&r.Limit, "limit", 100, "page size (queryModel.showCount)")
	fs.IntVar(&r.Page, "page", 1, "page number (queryModel.currentPage)")
	fs.BoolVar(&r.All, "all", false, "every page")
	fs.IntVar(&r.Week, "week", 0, "teaching week (for free-rooms)")
	fs.IntVar(&r.Weekday, "weekday", 0, "weekday 1-7 (for free-rooms)")
	fs.IntVar(&r.Period, "period", 0, "period (for free-rooms)")
	workers := fs.Int("workers", 2, "concurrent requests (for batch)")
	delay := fs.Duration("delay", 300*time.Millisecond, "pause after each request per worker (for batch)")
	outFile := fs.String("out", "", "optional JSON output file")
	params := kvFlags(url.Values{})
	fs.Var(params, "param", "request parameter key=value (repeatable)")
	_ = fs.Parse(args[1:])
	r.Params = map[string]string{}
	for k := range params {
		r.Params[k] = url.Values(params).Get(k)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	c := openJwglxt(ctx, auth)
	if r.Command == "batch" {
		if err := batchJwglxt(ctx, c, auth, *workers, *delay); err != nil {
			log.Fatal(err)
		}
		return
	}
	res, err := runJwglxt(ctx, c, r)
	if err != nil {
		log.Fatal(err)
	}
	writeJSONOut(*outFile, res)
}

func dispatchJwxt(args []string) {
	if len(args) < 1 {
		usage()
	}
	sub := args[0]
	fs := flag.NewFlagSet("jwxt "+sub, flag.ExitOnError)
	auth := bindAuthFlags(fs, "", true)
	grade := fs.String("grade", "", "grade filter (nj), e.g. 2024")
	college := fs.String("college", "", "college filter (xymc)")
	major := fs.String("major", "", "major filter (zymc)")
	mode := fs.String("mode", "all", "grades mode: all|term|best|failed")
	pathFlag := fs.String("path", "", "classic .aspx path (for page)")
	eventTarget := fs.String("event", "", "__EVENTTARGET for PostBack (for page)")
	allPages := fs.Bool("all", false, "paginate through all pages")
	postFlag := fs.Bool("post", false, "trigger PostBack on generic page")
	outFile := fs.String("out", "", "optional JSON output file")
	params := kvFlags(url.Values{})
	fs.Var(params, "param", "extra form parameter key=value (repeatable)")
	_ = fs.Parse(args[1:])

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	c := openJwxt(ctx, auth)
	pv := url.Values(params)

	var (
		res any
		err error
	)
	switch sub {
	case "curriculum":
		if *grade != "" && *college != "" && *major != "" {
			var rows []jwxt.CurriculumRow
			rows, err = c.CurriculumByMajor(ctx, *grade, *college, *major, *allPages)
			res = map[string]any{"rows": rows}
		} else {
			filters, rows, e := c.CurriculumPage(ctx)
			err = e
			res = map[string]any{"filters": filters, "rows": rows}
		}
	case "grades":
		res, err = c.Grades(ctx, *mode, pv)
	case "level-exams":
		res, err = c.LevelExams(ctx)
	case "makeup-exams":
		res, err = c.MakeupExams(ctx, pv)
	case "schedule":
		res, err = c.PersonalSchedule(ctx, pv)
	case "class-schedule":
		res, err = c.ClassSchedule(ctx, pv)
	case "teacher-schedule":
		res, err = c.TeacherSchedule(ctx, pv)
	case "time-schedule":
		res, err = c.TimeSchedule(ctx, pv)
	case "rooms":
		res, err = c.Rooms(ctx, pv)
	case "textbooks":
		res, err = c.Textbooks(ctx, pv)
	case "profile":
		res, err = c.Profile(ctx)
	case "competitions":
		res, err = c.Competitions(ctx, pv)
	case "thesis-topics":
		res, err = c.ThesisTopics(ctx, pv)
	case "teaching-plans":
		res, err = c.TeachingPlans(ctx, pv)
	case "page":
		if *pathFlag == "" {
			log.Fatal("need -path")
		}
		if *eventTarget != "" {
			pv.Set("__EVENTTARGET", *eventTarget)
			*postFlag = true
		}
		if *postFlag || len(pv) > 0 {
			res, err = c.PostBack(ctx, *pathFlag, nil, pv)
		} else {
			res, err = c.Snapshot(ctx, *pathFlag)
		}
	default:
		usage()
	}
	if err != nil {
		log.Fatal(err)
	}
	writeJSONOut(*outFile, res)
}
