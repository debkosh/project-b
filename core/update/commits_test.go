package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// A Flatpak's commits by version: each the file lists, read once; a commit it doesn't list
// is no version, and reads it again only after a while; a file that can't be read is no
// version, and the round's end stops the read.
func TestCommitsNameVersions(t *testing.T) {
	var reads atomic.Int32
	body := `{"stable":[{"commit":"c2","version":"0.4.1"},{"commit":"c1","version":"0.4.0"}],"dev":[{"commit":"d1","version":"0.4.2-dev"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		w.Write([]byte(body))
	}))
	defer srv.Close()
	c := &Commits{URL: srv.URL, Channel: Stable}
	ctx := context.Background()
	if v := c.VersionOf(ctx, "c2"); v != "0.4.1" {
		t.Fatalf("c2: %q", v)
	}
	if v := c.VersionOf(ctx, "c1"); v != "0.4.0" || reads.Load() != 1 {
		t.Fatalf("c1: %q after %d reads", v, reads.Load())
	}
	if v := c.VersionOf(ctx, "d1"); v != "" || reads.Load() != 1 {
		t.Fatalf("another channel's commit: %q after %d reads, want none and no read", v, reads.Load())
	}
	c.read = time.Now().Add(-commitsReread)
	body = `{"stable":[{"commit":"c3","version":"0.4.2"}]}`
	if v := c.VersionOf(ctx, "c3"); v != "0.4.2" || reads.Load() != 2 {
		t.Fatalf("a commit published since: %q after %d reads", v, reads.Load())
	}

	gone := &Commits{URL: srv.URL + "/missing", Channel: Stable}
	srv.Config.Handler = http.NotFoundHandler()
	if v := gone.VersionOf(ctx, "c3"); v != "" {
		t.Fatalf("an unreadable file: %q", v)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	late := &Commits{URL: srv.URL, Channel: Stable}
	if v := late.VersionOf(cancelled, "c3"); v != "" {
		t.Fatalf("a round that ended: %q", v)
	}
}
