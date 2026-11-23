package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sudiptadeb/synckosh/client"
)

// What can go wrong reaching a bucket, each of which the settings page words its own way
// (Problem). A request's error wraps one of these when the server couldn't be reached, didn't
// answer as a bucket, or refused the token.
var (
	// ErrAddress is an address Propose doesn't take.
	ErrAddress = errors.New("sync: not a sync address")
	// ErrUnreachable is a request that got no answer.
	ErrUnreachable = errors.New("sync: couldn't reach the server")
	// ErrServer is the server answering that it failed: it was reached.
	ErrServer = errors.New("sync: the server answered with an error")
	// ErrStalled is a request with nothing arriving or going for StalledAfter: it is ended.
	ErrStalled = errors.New("sync: nothing arrived for 30 seconds")
	// ErrNotBucket is an answer that isn't a bucket's: not found, or not JSON.
	ErrNotBucket = errors.New("sync: the address isn't a sync bucket")
	// ErrToken is the server refusing the token, or wanting one when none was given: it signed
	// this device out.
	ErrToken = errors.New("sync: the server didn't accept the token")
)

// StalledAfter is how long a request goes on with nothing arriving and nothing going before
// it is ended as stalled, the same as the updater's, a download's and an import's
// (docs/ux-guide.md). A run that takes long while its requests are answered never stalls.
var StalledAfter = 30 * time.Second

// Problem names what err is for the settings page: "address", "unreachable",
// "server", "stalled", "notBucket", "token", "reset", or "" for anything else. "reset" is
// synckosh's client.ErrBucketReset: the bucket holds no sealed copy of this device's key any
// more, so it was emptied, or set up again from another device, and this device sends
// nothing until sync is set up again here.
func Problem(err error) string {
	if errors.Is(err, client.ErrBucketReset) {
		return "reset"
	}
	for _, p := range []struct {
		err  error
		name string
	}{{ErrAddress, "address"}, {ErrUnreachable, "unreachable"}, {ErrServer, "server"}, {ErrStalled, "stalled"}, {ErrNotBucket, "notBucket"}, {ErrToken, "token"}} {
		if errors.Is(err, p.err) {
			return p.name
		}
	}
	return ""
}

// Words is err in words, for the page and the timeline: a request that failed reads as the
// request and then the cause, "GET v1/pull?after=1: EOF", as synckosh words an answer it
// refuses. An address may be a secret itself, as one that holds a token in its path is, and Go
// names the whole address in a request's error, so the request is named from v1 on; and what the page's
// sentence already says of the failure isn't said again.
func Words(err error) string {
	text := err.Error()
	var request *url.Error
	if !errors.As(err, &request) {
		return text
	}
	call := request.URL
	if u, e := url.Parse(request.URL); e == nil && u.Host != "" {
		call = u.Scheme + "://" + u.Host
		if i := strings.LastIndex(u.RequestURI(), "/v1/"); i >= 0 {
			call = u.RequestURI()[i+1:]
		}
	}
	cause := request.Err.Error()
	for _, named := range []error{ErrUnreachable, ErrServer, ErrNotBucket} {
		cause = strings.TrimPrefix(cause, named.Error()+": ")
	}
	cause = strings.TrimPrefix(cause, "sync: ")
	return strings.ReplaceAll(text, request.Error(), strings.ToUpper(request.Op)+" "+call+": "+cause)
}

// checked is c with every answer looked at first: what the server's answer says of the bucket
// becomes one of the errors above, so a call's error says which. Each request is told to
// asking as it goes out and to note once it is over, for Sync details.
func checked(c *http.Client, asking, note func(Request)) *http.Client {
	out := *c
	base := c.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	out.Transport = checking{base, asking, note}
	return &out
}

type checking struct {
	base         http.RoundTripper
	asking, note func(Request)
}

// RoundTrip measures the request round its answer: a request with no answer, or one refused,
// is over at once, and one answered when its body has been read and closed. It watches the
// request too: with nothing going or arriving for StalledAfter it is ended, as ErrStalled.
func (t checking) RoundTrip(r *http.Request) (*http.Response, error) {
	began := time.Now()
	q := Request{At: began.UnixMilli(), Method: r.Method, Path: r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:], Out: max(r.ContentLength, 0)}
	t.asking(q)
	ctx, end := context.WithCancelCause(r.Context())
	watch := time.AfterFunc(StalledAfter, func() { end(ErrStalled) })
	moved := func() { watch.Reset(StalledAfter) }
	stalled := func(err error) error {
		if err != nil && context.Cause(ctx) == ErrStalled {
			return ErrStalled
		}
		return err
	}
	over := func() {
		watch.Stop()
		end(nil)
		q.Took = time.Since(began).Milliseconds()
		t.note(q)
	}
	r = r.WithContext(ctx)
	if r.Body != nil {
		r.Body = &watched{r.Body, moved}
	}
	resp, err := t.answer(r)
	if resp == nil {
		var refused *statusError
		if errors.As(err, &refused) {
			q.Status = refused.status
		}
		err = stalled(err)
		over()
		return nil, err
	}
	moved()
	q.Status = resp.StatusCode
	resp.Body = &counted{ReadCloser: resp.Body, moved: moved, failed: stalled, over: func(n int64) { q.In = n; over() }}
	return resp, nil
}

// watched tells moved as a request's body goes out.
type watched struct {
	io.ReadCloser
	moved func()
}

func (w *watched) Read(p []byte) (int, error) {
	n, err := w.ReadCloser.Read(p)
	if n > 0 {
		w.moved()
	}
	return n, err
}

// statusError is a problem the server's answer named, with the answer's status.
type statusError struct {
	error
	status int
}

func (e *statusError) Unwrap() error { return e.error }

// counted counts an answer's bytes, telling moved as they arrive, names a read that failed
// through failed, and tells over once, when it is closed.
type counted struct {
	io.ReadCloser
	n      int64
	moved  func()
	failed func(error) error
	over   func(int64)
}

func (c *counted) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	c.n += int64(n)
	if n > 0 {
		c.moved()
	}
	if err != nil && err != io.EOF {
		err = c.failed(err)
	}
	return n, err
}

func (c *counted) Close() error {
	if c.over != nil {
		c.over(c.n)
		c.over = nil
	}
	return c.ReadCloser.Close()
}

func (t checking) answer(r *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(r)
	if err != nil {
		// A run cut short is no problem of the server's, and a stalled one RoundTrip names.
		if r.Context().Err() != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	var problem error
	switch kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		problem = ErrToken
	case resp.StatusCode >= 500:
		problem = fmt.Errorf("%w: %s", ErrServer, resp.Status)
	case resp.StatusCode == http.StatusNotFound || kind != "application/json":
		problem = fmt.Errorf("%w: %s, %s", ErrNotBucket, resp.Status, kind)
	default:
		return resp, nil
	}
	resp.Body.Close()
	return nil, &statusError{problem, resp.StatusCode}
}
