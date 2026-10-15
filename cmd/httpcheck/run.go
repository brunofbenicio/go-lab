package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type options struct {
	method     string
	expect     int // 0 means "any status below 400"
	timeout    time.Duration
	noRedirect bool
	workers    int
}

// result is the outcome of checking one URL.
type result struct {
	target string
	status int // 0 when no response was received
	ok     bool
	detail string // explanation when there was no response
}

// newClient builds an HTTP client for the given options. Keep-alives are off
// because every URL is requested only once.
func newClient(opt options) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true
	client := &http.Client{Timeout: opt.timeout, Transport: transport}
	if opt.noRedirect {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client
}

// check requests target once and classifies the answer.
func check(client *http.Client, opt options, target string) result {
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return result{target: target, detail: "invalid URL: use http:// or https://"}
	}
	req, err := http.NewRequestWithContext(context.Background(), opt.method, target, nil)
	if err != nil {
		return result{target: target, detail: err.Error()}
	}
	resp, err := client.Do(req)
	if err != nil {
		return result{target: target, detail: describe(err, opt.timeout)}
	}
	defer resp.Body.Close()
	// Drain a bounded amount so the connection can be closed cleanly.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	ok := resp.StatusCode < 400
	if opt.expect != 0 {
		ok = resp.StatusCode == opt.expect
	}
	return result{target: target, status: resp.StatusCode, ok: ok}
}

// describe turns a transport error into a short message.
func describe(err error, timeout time.Duration) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout after " + timeout.String()
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err.Error()
	}
	return err.Error()
}

// run is the testable entry point. It returns the process exit status:
// 0 when every URL is acceptable, 1 when at least one is not, 2 for usage errors.
func run(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("httpcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: httpcheck [flags] url ...")
		fs.PrintDefaults()
	}
	var opt options
	fs.StringVar(&opt.method, "method", "GET", "HTTP method to use")
	fs.IntVar(&opt.expect, "expect", 0, "exact status code expected (default: any status below 400)")
	fs.DurationVar(&opt.timeout, "timeout", 5*time.Second, "time limit for each request")
	fs.BoolVar(&opt.noRedirect, "no-redirect", false, "report redirects instead of following them")
	fs.IntVar(&opt.workers, "c", 4, "number of requests in flight at once")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	opt.method = strings.ToUpper(opt.method)
	switch {
	case fs.NArg() == 0:
		fmt.Fprintln(stderr, "httpcheck: at least one URL is required")
		return 2
	case opt.timeout <= 0:
		fmt.Fprintln(stderr, "httpcheck: -timeout must be positive")
		return 2
	case opt.expect != 0 && (opt.expect < 100 || opt.expect > 599):
		fmt.Fprintln(stderr, "httpcheck: -expect must be a status code between 100 and 599")
		return 2
	case opt.workers < 1:
		fmt.Fprintln(stderr, "httpcheck: -c must be at least 1")
		return 2
	}

	// Check concurrently but report in the order the URLs were given.
	client := newClient(opt)
	targets := fs.Args()
	results := make([]result, len(targets))
	slots := make(chan struct{}, opt.workers)
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			results[i] = check(client, opt, target)
		}()
	}
	wg.Wait()

	status := 0
	for _, r := range results {
		label, code := "OK", "ERR"
		if !r.ok {
			label = "FAIL"
			status = 1
		}
		if r.status != 0 {
			code = strconv.Itoa(r.status)
		}
		line := fmt.Sprintf("%-4s %s %s", label, code, r.target)
		if r.detail != "" {
			line += " (" + r.detail + ")"
		}
		fmt.Fprintln(stdout, line)
	}
	return status
}
