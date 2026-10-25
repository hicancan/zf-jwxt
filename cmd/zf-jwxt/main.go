// Command zf-jwxt captures data from a ZhengFang (正方) jwglxt deployment.
//
//	zf-jwxt timetable -base http://jwglxt.example.edu.cn -out data [-term 2026-2027-1] [-proxy socks5://127.0.0.1:1080]
//	zf-jwxt rooms     -base http://jwglxt.example.edu.cn -out data -weeks 9-20 [-term …] [-proxy …]
//
// Credentials come from ZF_USERNAME and ZF_PASSWORD. A rejected login is never
// retried, because the server locks accounts after repeated failures.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/hicancan/zf-jwxt/capture"
	"github.com/hicancan/zf-jwxt/zf"
)

var version = "dev"

func main() {
	log.SetFlags(log.Ltime)
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "timetable":
		timetable(os.Args[2:])
	case "rooms":
		rooms(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println("zf-jwxt", version)
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: zf-jwxt timetable -base URL -out DIR [-term YYYY-YYYY-N] [-proxy URL] [-workers N] [-delay D]")
	fmt.Fprintln(os.Stderr, "       zf-jwxt rooms -base URL -out DIR -weeks FIRST-LAST [-term YYYY-YYYY-N] [-proxy URL] [-workers N] [-delay D]")
	os.Exit(2)
}

func timetable(args []string) {
	fs := flag.NewFlagSet("timetable", flag.ExitOnError)
	base := fs.String("base", "", "jwglxt base URL")
	out := fs.String("out", "out", "output directory")
	termFlag := fs.String("term", "current", "term such as 2026-2027-1, or current")
	proxy := fs.String("proxy", "", "http:// or socks5:// proxy")
	workers := fs.Int("workers", 2, "concurrent requests")
	delay := fs.Duration("delay", 300*time.Millisecond, "pause after each class per worker")
	_ = fs.Parse(args)

	user, pass := os.Getenv("ZF_USERNAME"), os.Getenv("ZF_PASSWORD")
	if *base == "" || user == "" || pass == "" {
		log.Fatal("need -base, ZF_USERNAME and ZF_PASSWORD")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client, err := zf.New(*base, *proxy, user, pass)
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Login(ctx); err != nil {
		log.Fatal(err)
	}
	var term zf.Term
	if *termFlag == "current" {
		if term, err = client.CurrentTerm(ctx); err != nil {
			log.Fatal(err)
		}
	} else if term, err = zf.ParseTerm(*termFlag); err != nil {
		log.Fatal(err)
	}
	log.Printf("capturing %s", term)
	res, err := capture.Timetables(ctx, client, term, capture.Options{Dir: *out, Workers: *workers, Delay: *delay, MaxRelogins: 5})
	_ = json.NewEncoder(os.Stdout).Encode(res)
	if err != nil {
		log.Fatal(err)
	}
	if len(res.Failed) > 0 {
		os.Exit(1)
	}
}

// login opens a session from ZF_USERNAME and ZF_PASSWORD and resolves the term.
func login(ctx context.Context, base, proxy, termFlag string) (*zf.Client, zf.Term) {
	user, pass := os.Getenv("ZF_USERNAME"), os.Getenv("ZF_PASSWORD")
	if base == "" || user == "" || pass == "" {
		log.Fatal("need -base, ZF_USERNAME and ZF_PASSWORD")
	}
	client, err := zf.New(base, proxy, user, pass)
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Login(ctx); err != nil {
		log.Fatal(err)
	}
	var term zf.Term
	if termFlag == "current" {
		term, err = client.CurrentTerm(ctx)
	} else {
		term, err = zf.ParseTerm(termFlag)
	}
	if err != nil {
		log.Fatal(err)
	}
	return client, term
}

// parseWeeks accepts "9-20", "9" or "9,10,12".
func parseWeeks(s string) ([]int, error) {
	var weeks []int
	for _, part := range strings.Split(s, ",") {
		first, last, isRange := strings.Cut(strings.TrimSpace(part), "-")
		a, err := strconv.Atoi(first)
		if err != nil || a < 1 {
			return nil, fmt.Errorf("weeks %q", s)
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(last); err != nil || b < a || b > 30 {
				return nil, fmt.Errorf("weeks %q", s)
			}
		}
		for w := a; w <= b; w++ {
			weeks = append(weeks, w)
		}
	}
	return weeks, nil
}

func rooms(args []string) {
	fs := flag.NewFlagSet("rooms", flag.ExitOnError)
	base := fs.String("base", "", "jwglxt base URL")
	out := fs.String("out", "out", "output directory")
	termFlag := fs.String("term", "current", "term such as 2026-2027-1, or current")
	weeksFlag := fs.String("weeks", "", "weeks to query, e.g. 9-20")
	proxy := fs.String("proxy", "", "http:// or socks5:// proxy")
	workers := fs.Int("workers", 3, "concurrent requests")
	delay := fs.Duration("delay", 100*time.Millisecond, "pause after each request per worker")
	_ = fs.Parse(args)
	weeks, err := parseWeeks(*weeksFlag)
	if err != nil || len(weeks) == 0 {
		log.Fatal("need -weeks, e.g. -weeks 9-20")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	client, term := login(ctx, *base, *proxy, *termFlag)
	log.Printf("free rooms of %s, weeks %s", term, *weeksFlag)
	res, err := capture.Rooms(ctx, client, term, weeks, capture.Options{Dir: *out, Workers: *workers, Delay: *delay, MaxRelogins: 5})
	_ = json.NewEncoder(os.Stdout).Encode(res)
	if err != nil {
		log.Fatal(err)
	}
	if len(res.Failed) > 0 {
		os.Exit(1)
	}
}
