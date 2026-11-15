// batch: many jwglxt requests on one session. Requests arrive as JSON lines on stdin, answers leave as JSON lines on
// stdout in the order they finish, each carrying its request's id. What to ask and where to keep the answers is the
// caller's; this only keeps the session alive (signing in again when it expires, if it can) and retries a dropped
// connection, as one person working through a list would.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/hicancan/zf-jwxt/jwglxt"
)

type batchAnswer struct {
	ID     string `json:"id"`
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// session signs in again once for every worker that finds the session expired at the same time.
type session struct {
	mu       sync.Mutex
	client   *jwglxt.Client
	canLogin bool
	epoch    int
	relogins int
}

const maxRelogins = 5

func (s *session) renew(ctx context.Context, seen int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.epoch != seen {
		return nil
	}
	if !s.canLogin {
		return errors.New("the session expired and was given as a cookie; sign in again and rerun")
	}
	if s.relogins >= maxRelogins {
		return errors.New("the session keeps expiring")
	}
	s.relogins++
	if err := s.client.Login(ctx); err != nil {
		return err
	}
	s.epoch++
	return nil
}

func (s *session) answer(ctx context.Context, r jwglxtRequest) (any, error) {
	var err error
	for attempt := 0; attempt < 6; attempt++ {
		s.mu.Lock()
		epoch := s.epoch
		s.mu.Unlock()
		var res any
		res, err = runJwglxt(ctx, s.client, r)
		switch {
		case err == nil:
			return res, nil
		case errors.Is(err, jwglxt.ErrLoginRejected), ctx.Err() != nil:
			return nil, err
		case errors.Is(err, jwglxt.ErrSessionExpired):
			if rerr := s.renew(ctx, epoch); rerr != nil {
				return nil, rerr
			}
		default:
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 2 * time.Second):
			}
		}
	}
	return nil, err
}

func batchJwglxt(ctx context.Context, c *jwglxt.Client, auth *authFlags, workers int, delay time.Duration) error {
	s := &session{client: c, canLogin: auth.cookie == "" && auth.cookieCmd == ""}
	jobs := make(chan jwglxtRequest)
	out := json.NewEncoder(os.Stdout)
	var mu sync.Mutex
	var fatal error
	var wg sync.WaitGroup
	for range max(1, workers) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range jobs {
				res, err := s.answer(ctx, r)
				a := batchAnswer{ID: r.ID, OK: err == nil, Result: res}
				if err != nil {
					a.Error = err.Error()
				}
				mu.Lock()
				_ = out.Encode(a)
				if errors.Is(err, jwglxt.ErrLoginRejected) && fatal == nil {
					fatal = err
				}
				mu.Unlock()
				time.Sleep(delay)
			}
		}()
	}
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<24)
	line := 0
	var readErr error
	for in.Scan() {
		line++
		var r jwglxtRequest
		if err := json.Unmarshal(in.Bytes(), &r); err != nil {
			readErr = fmt.Errorf("request line %d: %w", line, err)
			break
		}
		if r.Command == "batch" {
			readErr = fmt.Errorf("request line %d: a batch cannot hold a batch", line)
			break
		}
		mu.Lock()
		stop := fatal != nil
		mu.Unlock()
		if stop || ctx.Err() != nil {
			break
		}
		jobs <- r
	}
	close(jobs)
	wg.Wait()
	if readErr == nil {
		readErr = in.Err()
	}
	log.Printf("batch: %d requests", line)
	return errors.Join(fatal, readErr, ctx.Err())
}
