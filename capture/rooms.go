package capture

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/hicancan/zf-jwxt/zf"
)

// RoomResult summarises a free-room capture.
type RoomResult struct {
	Term   string   `json:"term"`
	Slots  int      `json:"slots"`
	Rooms  int      `json:"rooms"`
	Failed []string `json:"failed"`
}

// The room fields kept in rooms.json; the rest is page furniture.
var roomFields = []string{"cd_id", "cdmc", "cdbh", "cdlb_id", "cdlbmc", "zws", "kszws1", "jxlmc", "lh", "lch", "xqh_id", "xqmc", "sydxmc", "bz"}

// Rooms writes, for the given weeks of a term, which rooms are free in every
// period of every day, and every room those answers name:
//
//	<dir>/<term>/rooms.json               every room seen, by cd_id (kept across runs)
//	<dir>/<term>/free/<week>-<weekday>.json   {"periods": {"1": [cd_id, …], …}}
//
// Expects client to be logged in already.
func Rooms(ctx context.Context, client *zf.Client, term zf.Term, weeks []int, opt Options) (RoomResult, error) {
	s := &session{client: client, max: opt.MaxRelogins}
	root := filepath.Join(opt.Dir, term.String())
	if err := os.MkdirAll(filepath.Join(root, "free"), 0o755); err != nil {
		return RoomResult{}, err
	}
	var periods []map[string]any
	if err := s.call(ctx, func() (err error) { periods, err = client.Periods(ctx, term); return }); err != nil {
		return RoomResult{}, fmt.Errorf("periods: %w", err)
	}
	if len(periods) == 0 {
		return RoomResult{}, fmt.Errorf("periods: the term has none")
	}

	registry := map[string]map[string]any{}
	if b, err := os.ReadFile(filepath.Join(root, "rooms.json")); err == nil {
		var known []map[string]any
		if json.Unmarshal(b, &known) == nil {
			for _, r := range known {
				registry[fmt.Sprint(r["cd_id"])] = r
			}
		}
	}

	type slot struct{ week, weekday, period int }
	type day struct{ week, weekday int }
	free := map[day]map[string][]string{}
	res := RoomResult{Term: term.String()}
	var mu sync.Mutex
	jobs := make(chan slot)
	var wg sync.WaitGroup
	for range max(1, opt.Workers) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				var items []map[string]any
				err := s.call(ctx, func() (err error) { items, err = client.FreeRooms(ctx, term, j.week, j.weekday, j.period); return })
				mu.Lock()
				if err != nil {
					res.Failed = append(res.Failed, fmt.Sprintf("%d-%d-%d", j.week, j.weekday, j.period))
					log.Printf("week %d day %d period %d: %v", j.week, j.weekday, j.period, err)
				} else {
					ids := make([]string, 0, len(items))
					for _, item := range items {
						id := fmt.Sprint(item["cd_id"])
						ids = append(ids, id)
						room := map[string]any{}
						for _, k := range roomFields {
							if v, ok := item[k]; ok && v != nil && v != "" {
								room[k] = v
							}
						}
						registry[id] = room
					}
					sort.Strings(ids)
					d := day{j.week, j.weekday}
					if free[d] == nil {
						free[d] = map[string][]string{}
					}
					free[d][strconv.Itoa(j.period)] = ids
					res.Slots++
				}
				if done := res.Slots + len(res.Failed); done%100 == 0 {
					log.Printf("%d slots", done)
				}
				mu.Unlock()
				time.Sleep(opt.Delay)
			}
		}()
	}
feed:
	for _, week := range weeks {
		for weekday := 1; weekday <= 7; weekday++ {
			for period := 1; period <= len(periods); period++ {
				select {
				case jobs <- slot{week, weekday, period}:
				case <-ctx.Done():
					break feed
				}
			}
		}
	}
	close(jobs)
	wg.Wait()

	observed := time.Now().UTC().Format(time.RFC3339)
	for d, byPeriod := range free {
		// A day is written only when every period was answered, so a file never mixes old and new.
		if len(byPeriod) != len(periods) {
			continue
		}
		v := map[string]any{"week": d.week, "weekday": d.weekday, "observed_at": observed, "periods": byPeriod}
		if err := writeJSON(filepath.Join(root, "free", fmt.Sprintf("%d-%d.json", d.week, d.weekday)), v); err != nil {
			return res, err
		}
	}
	rooms := make([]map[string]any, 0, len(registry))
	for _, r := range registry {
		rooms = append(rooms, r)
	}
	sort.Slice(rooms, func(i, j int) bool { return fmt.Sprint(rooms[i]["cd_id"]) < fmt.Sprint(rooms[j]["cd_id"]) })
	if err := writeJSON(filepath.Join(root, "rooms.json"), rooms); err != nil {
		return res, err
	}
	res.Rooms = len(rooms)
	sort.Strings(res.Failed)
	if res.Failed == nil {
		res.Failed = []string{}
	}
	return res, ctx.Err()
}
