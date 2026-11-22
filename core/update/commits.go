package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Commits names a Flatpak repository's commits by their versions, for an update portal that
// knows only commits (docs/research/linux.md, 5): the linux.json the release publishes beside
// the repository, in the same deploy, listing every commit the repository keeps on the
// channel, newest first. A commit it doesn't list, or a file that can't be read, has no
// version, and the pages say "An update" instead.
type Commits struct {
	URL     string
	Channel string

	mu       sync.Mutex
	versions map[string]string
	read     time.Time
}

// commitsReread is how soon a commit the file didn't list may read it again: a release made
// just now, the file not yet in the cache the last read had.
const commitsReread = time.Minute

// VersionOf is commit's version, reading the file when it doesn't know the commit; ctx is the
// updater's round, which the person's Cancel or the app quitting ends.
func (c *Commits) VersionOf(ctx context.Context, commit string) string {
	c.mu.Lock()
	v, known := c.versions[commit]
	stale := time.Since(c.read) >= commitsReread
	c.mu.Unlock()
	if known || !stale {
		return v
	}
	versions, err := c.fetch(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.read = time.Now()
	if err != nil {
		return ""
	}
	c.versions = versions
	return versions[commit]
}

func (c *Commits) fetch(ctx context.Context) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", c.URL, resp.Status)
	}
	var file map[string][]struct {
		Commit  string `json:"commit"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		return nil, fmt.Errorf("%s: %w", c.URL, err)
	}
	versions := map[string]string{}
	for _, r := range file[c.Channel] {
		versions[r.Commit] = r.Version
	}
	return versions, nil
}
