package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	gosync "sync"
	"time"

	"github.com/sudiptadeb/synckosh/bolt"
	"github.com/sudiptadeb/synckosh/server"

	"projectb/sync"
)

// syncServer is a sync server for the tests (shell/e2e/sync.cjs, docs/sync.md): synckosh's
// server over https in this process, as the memd server serves a bucket, at /sync with its
// token as a bearer or at /sync/<token>; any other path is a web page, not a bucket. Sync's
// requests reach it as https://memd.example.com, the name the mocks use, and as the server
// to start with (sync.StartServer), which no test may reach for real, and trust its
// certificate; any other host is dialled as it is. It counts every request it gets, and a
// test changes how it answers: a token signed out, every connection dropped, every request
// held, or every request answered with a failure.
type syncServer struct {
	ts *httptest.Server

	mu       gosync.Mutex
	srv      *server.Server
	requests int
	refused  map[string]bool
	down     bool
	held     chan struct{}
	failing  int
	// For a reset: the store, its path, and who a token is.
	data *bolt.Store
	path string
	auth server.Auth
}

// startSyncServer starts the server on its store in path, kept across restarts, and has sync's
// requests trust it.
func startSyncServer(path string) *syncServer {
	data, err := bolt.OpenStore(path)
	if err != nil {
		log.Fatalf("-sync-server: %v", err)
	}
	s := &syncServer{refused: map[string]bool{}, data: data, path: path}
	s.auth = server.Bearer(func(token string) (string, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.refused[token] {
			return "", errors.New("signed out")
		}
		return "bucket-" + token, nil
	})
	s.srv = server.New(data, s.auth)
	s.ts = httptest.NewTLSServer(http.HandlerFunc(s.serve))
	// The test certificate names example.com, which every host here is checked as.
	t := s.ts.Client().Transport.(*http.Transport).Clone()
	t.TLSClientConfig.ServerName = "example.com"
	listening := s.ts.Listener.Addr().String()
	t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr == syncHost+":443" || addr == startHost+":443" {
			addr = listening
		}
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	sync.HTTPClient = &http.Client{Transport: t, Timeout: sync.HTTPClient.Timeout}
	log.Printf("sync server on https://%s, at %s", syncHost, listening)
	return s
}

// syncHost is the name sync reaches the server by.
const syncHost = "memd.example.com"

// startHost is the host of the server to start with.
var startHost = func() string {
	u, _ := url.Parse(sync.StartServer)
	return u.Host
}()

func (s *syncServer) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requests++
	down, held, failing, srv := s.down, s.held, s.failing, s.srv
	s.mu.Unlock()
	if down {
		if c, _, err := w.(http.Hijacker).Hijack(); err == nil {
			c.Close()
		}
		return
	}
	if held != nil {
		select {
		case <-held:
		case <-r.Context().Done():
			return
		}
	}
	if failing != 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(failing)
		w.Write([]byte(`{"error":"the test server fails"}`))
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/sync/")
	if !ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<!doctype html><title>Not a bucket</title><p>A web page, not a sync bucket."))
		return
	}
	r = r.Clone(r.Context())
	r.URL.Path, r.URL.RawPath = "/"+rest, ""
	if token, path, ok := strings.Cut(rest, "/"); ok && token != "v1" {
		r.URL.Path = "/" + path
		r.Header.Set("Authorization", "Bearer "+token)
	}
	srv.ServeHTTP(w, r)
}

// control is /mock/sync: GET says the server's address and how many requests it has had;
// POST changes how it answers, with refuse=<token> signing a token out, down=1 or 0 dropping
// every connection or not, slow=1 holding every request until slow=0, fail=<status> answering
// every request with that status as synckosh's server answers a failure, in JSON, until
// fail=0, stalled=<milliseconds> setting how long sync waits with nothing arriving before
// it ends a request as stalled, and reset=1 emptying the server, key and all.
func (s *syncServer) control(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == http.MethodPost {
		if t := r.FormValue("refuse"); t != "" {
			s.refused[t] = true
		}
		if d := r.FormValue("down"); d != "" {
			s.down = d == "1"
		}
		// reset=1 empties the server: every bucket is new, with no items and no key, as a
		// bucket reset on the server's own page is.
		if r.FormValue("reset") == "1" {
			// In place, so the server is as empty after the app starts again.
			s.data.Close()
			os.Remove(s.path)
			data, err := bolt.OpenStore(s.path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			s.data, s.srv = data, server.New(data, s.auth)
		}
		if f := r.FormValue("fail"); f != "" {
			s.failing, _ = strconv.Atoi(f)
		}
		if ms, err := strconv.Atoi(r.FormValue("stalled")); err == nil && ms > 0 {
			sync.StalledAfter = time.Duration(ms) * time.Millisecond
		}
		switch r.FormValue("slow") {
		case "1":
			if s.held == nil {
				s.held = make(chan struct{})
			}
		case "0":
			if s.held != nil {
				close(s.held)
				s.held = nil
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"address": "https://" + syncHost, "requests": s.requests})
}
