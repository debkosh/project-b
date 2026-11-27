package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// The sources' downloads: each is plain text, at most maxSize bytes, fetched with a deadline
// and tried twice. A source under repoRaw (our own lists' address on main) is read from the
// checkout instead, so a run on a branch publishes the branch's own test lists.
type fetcher struct {
	client    *http.Client
	maxSize   int64
	repoRaw   string
	repoRoot  string
	retryWait time.Duration
}

func (f *fetcher) get(ctx context.Context, url string) ([]byte, error) {
	if f.repoRaw != "" && strings.HasPrefix(url, f.repoRaw) {
		rel := filepath.FromSlash(strings.TrimPrefix(url, f.repoRaw))
		if !filepath.IsLocal(rel) {
			return nil, fmt.Errorf("%s: not inside the checkout", url)
		}
		b, err := os.ReadFile(filepath.Join(f.repoRoot, rel))
		if err != nil {
			return nil, err
		}
		return b, checkText(b, "")
	}
	var err error
	for try := 0; try < 2; try++ {
		if try > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(f.retryWait):
			}
		}
		var b []byte
		var ctype string
		b, ctype, err = f.once(ctx, url)
		if err == nil {
			return b, checkText(b, ctype)
		}
		var perm permanent
		if errors.As(err, &perm) {
			break
		}
	}
	return nil, err
}

type permanent struct{ error }

func (f *fetcher) once(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", permanent{err}
	}
	req.Header.Set("User-Agent", "project-b-lists/1 (+https://github.com/debkosh/project-b)")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("%s: %s", url, resp.Status)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return nil, "", permanent{err}
		}
		return nil, "", err
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, f.maxSize+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(b)) > f.maxSize {
		return nil, "", permanent{fmt.Errorf("%s: larger than %d bytes", url, f.maxSize)}
	}
	return b, resp.Header.Get("Content-Type"), nil
}

// checkText refuses what isn't a list's text: empty, not UTF-8, holding NUL bytes, or a web
// page (an error or sign-in page served with 200).
func checkText(b []byte, ctype string) error {
	if strings.HasPrefix(strings.ToLower(ctype), "text/html") {
		return fmt.Errorf("a web page (%s), not a list", ctype)
	}
	t := bytes.TrimSpace(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")))
	if len(t) == 0 {
		return errors.New("empty")
	}
	if bytes.IndexByte(b, 0) >= 0 || !utf8.Valid(b) {
		return errors.New("not text")
	}
	head := strings.ToLower(string(t[:min(len(t), 64)]))
	if strings.HasPrefix(head, "<!doctype") || strings.HasPrefix(head, "<html") {
		return errors.New("a web page, not a list")
	}
	return nil
}

type fetched struct {
	body []byte
	err  error
}

// fetchAll downloads every URL once, a few at a time.
func (f *fetcher) fetchAll(ctx context.Context, urls []string, workers int) map[string]fetched {
	out := make(map[string]fetched, len(urls))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for _, u := range urls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			b, err := f.get(ctx, u)
			mu.Lock()
			out[u] = fetched{b, err}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

// Where the previous run's files are: the release itself.
type store interface {
	get(name string) ([]byte, error)
}

var errNotFound = errors.New("not found")

// httpStore reads a published release's assets by their download address.
type httpStore struct {
	base   string // ends in /
	client *http.Client
	max    int64
}

func (s httpStore) get(name string) ([]byte, error) {
	resp, err := s.client.Get(s.base + name)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", name, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, s.max+1))
	if err == nil && int64(len(b)) > s.max {
		err = fmt.Errorf("%s: larger than %d bytes", name, s.max)
	}
	return b, err
}

// dirStore reads them from a folder, for local runs and the tests.
type dirStore string

func (d dirStore) get(name string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(string(d), name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, errNotFound
	}
	return b, err
}

// ghStore reads them with gh, which can read a draft release's assets too.
type ghStore struct{ repo, tag string }

func (g ghStore) get(name string) ([]byte, error) {
	var out, stderr bytes.Buffer
	cmd := exec.Command("gh", "release", "download", g.tag, "--repo", g.repo, "--pattern", name, "--output", "-")
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.ToLower(stderr.String())
		if strings.Contains(msg, "not found") || strings.Contains(msg, "no assets match") {
			return nil, errNotFound
		}
		return nil, fmt.Errorf("gh release download %s %s: %v: %s", g.tag, name, err, strings.TrimSpace(stderr.String()))
	}
	return out.Bytes(), nil
}
