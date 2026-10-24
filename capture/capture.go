// Package capture writes a term's timetables as plain files:
//
//	<dir>/<term>/term.json
//	<dir>/<term>/periods.json
//	<dir>/<term>/catalog.json
//	<dir>/<term>/classes/<bh_id>.json
//
// A class file that already exists is not fetched again, so an interrupted
// run resumes where it stopped.
package capture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/hicancan/zf-jwxt/zf"
)

type Options struct {
	Dir         string
	Workers     int
	Delay       time.Duration
	MaxRelogins int
}

type Result struct {
	Term    string   `json:"term"`
	Classes int      `json:"classes"`
	Fetched int      `json:"fetched"`
	Skipped int      `json:"skipped"`
	Failed  []string `json:"failed"`
}

// session serialises re-logins so concurrent workers share one new session.
type session struct {
	mu       sync.Mutex
	client   *zf.Client
	relogins int
	max      int
	epoch    int
}

func (s *session) relogin(ctx context.Context, seen int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.epoch != seen {
		return nil
	}
	if s.relogins >= s.max {
		return errors.New("capture: session keeps expiring; giving up")
	}
	s.relogins++
	if err := s.client.Login(ctx); err != nil {
		return err
	}
	s.epoch++
	return nil
}

// call runs fn, re-logging in on an expired session and retrying transport
// failures (a tunnel drops its streams every few minutes).
func (s *session) call(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 0; attempt < 6; attempt++ {
		s.mu.Lock()
		epoch := s.epoch
		s.mu.Unlock()
		err = fn()
		switch {
		case err == nil:
			return nil
		case errors.Is(err, zf.ErrLoginRejected):
			return err
		case errors.Is(err, zf.ErrSessionExpired):
			if rerr := s.relogin(ctx, epoch); rerr != nil {
				return rerr
			}
		default:
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 2 * time.Second):
			}
		}
	}
	return err
}

var sensitiveKey = regexp.MustCompile(`(?i)^(userModel|queryModel|cookie|token|password|phone|mobile|sjh|lxdh|email|dzyx)$`)

// Scrub drops account and contact fields the server echoes into every answer.
func Scrub(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			if !sensitiveKey.MatchString(k) {
				out[k] = Scrub(item)
			}
		}
		return out
	case []any:
		for i := range t {
			t[i] = Scrub(t[i])
		}
		return t
	}
	return v
}

// Timetables expects client to be logged in already.
func Timetables(ctx context.Context, client *zf.Client, term zf.Term, opt Options) (Result, error) {
	s := &session{client: client, max: opt.MaxRelogins}
	root := filepath.Join(opt.Dir, term.String())
	if err := os.MkdirAll(filepath.Join(root, "classes"), 0o755); err != nil {
		return Result{}, err
	}

	var catalog []map[string]any
	if err := s.call(ctx, func() (err error) { catalog, err = client.Catalog(ctx, term); return }); err != nil {
		return Result{}, fmt.Errorf("catalog: %w", err)
	}
	sort.Slice(catalog, func(i, j int) bool { return fmt.Sprint(catalog[i]["bh_id"]) < fmt.Sprint(catalog[j]["bh_id"]) })
	var periods []map[string]any
	if err := s.call(ctx, func() (err error) { periods, err = client.Periods(ctx, term); return }); err != nil {
		return Result{}, fmt.Errorf("periods: %w", err)
	}
	observed := time.Now().UTC().Format(time.RFC3339)
	for name, v := range map[string]any{
		"term.json":    map[string]any{"year": term.Year, "number": term.Number, "label": term.String(), "observed_at": observed},
		"catalog.json": Scrub(toAny(catalog)),
		"periods.json": Scrub(toAny(periods)),
	} {
		if err := writeJSON(filepath.Join(root, name), v); err != nil {
			return Result{}, err
		}
	}

	res := Result{Term: term.String(), Classes: len(catalog)}
	jobs := make(chan map[string]any)
	var mu sync.Mutex
	var wg sync.WaitGroup
	workers := max(1, opt.Workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobs {
				id := fmt.Sprint(entry["bh_id"])
				var tt map[string]any
				err := s.call(ctx, func() (err error) { tt, err = client.ClassTimetable(ctx, term, entry); return })
				if err == nil {
					err = writeJSON(filepath.Join(root, "classes", id+".json"), Scrub(tt))
				}
				mu.Lock()
				if err != nil {
					res.Failed = append(res.Failed, id)
					log.Printf("class %s: %v", id, err)
				} else {
					res.Fetched++
				}
				done := res.Fetched + len(res.Failed)
				mu.Unlock()
				if done%50 == 0 {
					log.Printf("%d/%d fetched", done, len(catalog)-res.Skipped)
				}
				time.Sleep(opt.Delay)
			}
		}()
	}
	for _, entry := range catalog {
		id := fmt.Sprint(entry["bh_id"])
		if id == "" || id == "<nil>" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, "classes", id+".json")); err == nil {
			res.Skipped++
			continue
		}
		select {
		case jobs <- entry:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	wg.Wait()
	sort.Strings(res.Failed)
	if res.Failed == nil {
		res.Failed = []string{}
	}
	return res, ctx.Err()
}

func toAny[T any](v []T) []any {
	out := make([]any, len(v))
	for i := range v {
		out[i] = v[i]
	}
	return out
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
