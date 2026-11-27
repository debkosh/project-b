package mockengine

import (
	"context"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"syscall"

	"projectb/engine"
)

// fetches is what the mock keeps of fetches: those running, and every one asked.
type fetches struct {
	mu      sync.Mutex
	running map[engine.FetchID]context.CancelFunc
	asked   []engine.FetchRequest
}

// Fetch fetches the address over plain HTTP, with no cookies, and keeps the engine's rules:
// only http and https, and no server on a more private address than the request is made
// from. Fetched follows on the main thread.
func (e *Engine) Fetch(f engine.FetchRequest) {
	ctx, cancel := context.WithCancel(context.Background())
	e.fetches.mu.Lock()
	if e.fetches.running == nil {
		e.fetches.running = map[engine.FetchID]context.CancelFunc{}
	}
	e.fetches.running[f.ID] = cancel
	e.fetches.asked = append(e.fetches.asked, f)
	e.fetches.mu.Unlock()
	go func() {
		got := FetchFile(ctx, f)
		e.Post(func() {
			e.fetches.mu.Lock()
			_, running := e.fetches.running[f.ID]
			delete(e.fetches.running, f.ID)
			e.fetches.mu.Unlock()
			cancel()
			if running {
				e.listener.Fetched(got)
			}
		})
	}()
}

// CancelFetch stops a fetch, whose file goes.
func (e *Engine) CancelFetch(id engine.FetchID) {
	e.fetches.mu.Lock()
	cancel := e.fetches.running[id]
	delete(e.fetches.running, id)
	e.fetches.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Fetches is every fetch asked of the mock, in order. Any goroutine.
func (e *Engine) Fetches() []engine.FetchRequest {
	e.fetches.mu.Lock()
	defer e.fetches.mu.Unlock()
	return append([]engine.FetchRequest{}, e.fetches.asked...)
}

var errRefused = errors.New("refused")

// FetchFile is the mock's fetch itself, for a test with no engine running: it fetches f
// into its file and returns how it ended, keeping the engine's rules.
func FetchFile(ctx context.Context, f engine.FetchRequest) engine.Fetched {
	out := engine.Fetched{ID: f.ID, Outcome: engine.FetchRefused, Space: engine.SpacePublic, URL: f.URL}
	allowed := func(u *url.URL) bool { return (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" }
	u, err := url.Parse(f.URL)
	if err != nil || !allowed(u) {
		return out
	}
	var space engine.AddressSpace
	dialer := &net.Dialer{Control: func(network, address string, _ syscall.RawConn) error {
		host, _, _ := net.SplitHostPort(address)
		space = engine.SpaceOf(host)
		if f.From != engine.SpaceAny && engine.MorePrivate(space, f.From) {
			return errRefused
		}
		return nil
	}}
	client := &http.Client{
		Transport: &http.Transport{DialContext: dialer.DialContext, DisableKeepAlives: true},
		CheckRedirect: func(r *http.Request, _ []*http.Request) error {
			if !allowed(r.URL) {
				return errRefused
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", f.URL, nil)
	if err != nil {
		return out
	}
	if f.Page != "" {
		req.Header.Set("Referer", f.Page)
	}
	if f.Range != "" {
		req.Header.Set("Range", "bytes="+f.Range)
	}
	res, err := client.Do(req)
	if err != nil {
		if !errors.Is(err, errRefused) {
			out.Outcome = engine.FetchConnection
		}
		return out
	}
	defer res.Body.Close()
	out.Status, out.Space, out.URL = res.StatusCode, space, res.Request.URL.String()
	out.Mime, _, _ = mime.ParseMediaType(res.Header.Get("Content-Type"))
	if _, params, err := mime.ParseMediaType(res.Header.Get("Content-Disposition")); err == nil {
		out.Name = params["filename"]
	}
	if res.StatusCode >= 400 {
		out.Outcome = engine.FetchServer
		return out
	}
	file, err := os.OpenFile(f.Path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err == nil {
		_, err = io.Copy(file, res.Body)
		if cerr := file.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		os.Remove(f.Path)
		out.Outcome = engine.FetchConnection
		return out
	}
	out.Outcome = engine.FetchOK
	return out
}
