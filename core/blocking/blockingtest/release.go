// Package blockingtest is a lists release for tests: an HTTP server that serves a signed
// manifest, each list's text and the patches between its versions, as the workflow
// publishes them (docs/blocking.md), signed with a key made for the test.
package blockingtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"

	"projectb/blocking"
)

// Release is the release's files, and what was asked of it.
type Release struct {
	// Key signs the manifest; Public is what the app checks it with.
	Key    ed25519.PrivateKey
	Public ed25519.PublicKey
	server *httptest.Server

	mu        sync.Mutex
	files     map[string][]byte
	lists     map[string][]string
	resources map[string][]string
	manifest  []byte
	sig       []byte
	etag      int
	published time.Time
	asked     []string
}

// New starts a release with nothing published; the test's end stops it.
func New(t *testing.T) *Release {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	r := &Release{Key: key, Public: pub, files: map[string][]byte{}, lists: map[string][]string{}, resources: map[string][]string{}, published: time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)}
	r.server = httptest.NewServer(http.HandlerFunc(r.serve))
	t.Cleanup(r.server.Close)
	return r
}

// URL is the release's address, for blocking.Setup's BaseURL.
func (r *Release) URL() string { return r.server.URL + "/" }

func (r *Release) serve(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := strings.TrimPrefix(req.URL.Path, "/")
	etag := `"` + strconv.Itoa(r.etag) + `"`
	if name == "manifest.json" && r.manifest != nil && req.Header.Get("If-None-Match") == etag {
		r.asked = append(r.asked, name+" 304")
		w.WriteHeader(http.StatusNotModified)
		return
	}
	r.asked = append(r.asked, name)
	var b []byte
	switch name {
	case "manifest.json":
		b = r.manifest
		w.Header().Set("ETag", etag)
	case "manifest.json.sig":
		b = r.sig
	default:
		b = r.files[name]
	}
	if b == nil {
		http.NotFound(w, req)
		return
	}
	w.Write(b)
}

// Asked is what was asked for, in order: a file's name, "manifest.json 304" for a manifest
// not changed. It starts again empty.
func (r *Release) Asked() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.asked
	r.asked = nil
	return a
}

// Put adds a list's next text. Publish publishes it.
func (r *Release) Put(id, text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lists[id] = append(r.lists[id], text)
}

// PutResources adds the resources' next JSON.
func (r *Release) PutResources(id, json string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resources[id] = append(r.resources[id], json)
}

// Set replaces a published file's bytes, as a damaged download would have them.
func (r *Release) Set(file string, b []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.files[file] = b
}

// Sum is a text's SHA-256, as the manifest names it.
func Sum(text string) string {
	s := sha256.Sum256([]byte(text))
	return hex.EncodeToString(s[:])
}

// File is the name of a list's text in the release, and Patch of its patch from old.
func File(id, text, ext string) string { return id + "-" + Sum(text)[:16] + ext + ".zst" }

func Patch(id, old, text string) string {
	return id + "-" + Sum(old)[:16] + "-" + Sum(text)[:16] + ".patch.zst"
}

// Publish writes the manifest of each list's last text, its whole file and a patch from
// each of its last eight texts before, and signs it, an hour after the last.
func (r *Release) Publish() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.published = r.published.Add(time.Hour)
	m := blocking.Manifest{Format: 1, Published: r.published, Lists: map[string]blocking.Entry{}, Resources: map[string]blocking.Entry{}}
	put := func(into map[string]blocking.Entry, id string, versions []string, ext string) {
		text := versions[len(versions)-1]
		e := blocking.Entry{SHA256: Sum(text), Size: int64(len(text)), File: File(id, text, ext), Changed: r.published, Patches: map[string]string{}}
		r.files[e.File] = encode([]byte(text), nil)
		for i := max(0, len(versions)-9); i < len(versions)-1; i++ {
			old := versions[i]
			if old == text {
				continue
			}
			p := Patch(id, old, text)
			e.Patches[Sum(old)] = p
			r.files[p] = encode([]byte(text), []byte(old))
		}
		into[id] = e
	}
	for id, v := range r.lists {
		put(m.Lists, id, v, ".txt")
	}
	for id, v := range r.resources {
		put(m.Resources, id, v, ".json")
	}
	r.manifest, _ = json.Marshal(m)
	r.sig = []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(r.Key, r.manifest)) + "\n")
	r.etag++
}

// Manifest is the published manifest and its signature.
func (r *Release) Manifest() (manifest, sig []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.manifest, r.sig
}

// SetManifest serves manifest and sig in place of the published ones.
func (r *Release) SetManifest(manifest, sig []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manifest, r.sig = manifest, sig
	r.etag++
}

// encode is text as a zstd frame, with dict as its raw dictionary (id 0) for a patch.
func encode(text, dict []byte) []byte {
	opts := []zstd.EOption{zstd.WithEncoderConcurrency(1)}
	if dict != nil {
		opts = append(opts, zstd.WithEncoderDictRaw(0, dict))
	}
	e, err := zstd.NewWriter(nil, opts...)
	if err != nil {
		panic(err)
	}
	defer e.Close()
	return e.EncodeAll(text, nil)
}
