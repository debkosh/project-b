package blocking

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// PublicKey is the public half of lists-signing.key, which signs the lists release's
// manifest (docs/blocking.md, Building and publishing them). Only a manifest it signed is
// taken.
var PublicKey = ed25519.PublicKey(mustHex("4ca1b52b0ca5f7e7c5f2389c041017dc3546d8b160813a53e6e29628b3911ea3"))

// Releases is where the lists release's files are: the manifest, its signature, and each
// list and patch by its file name.
const Releases = "https://github.com/debkosh/project-b/releases/download/lists/"

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// Manifest is manifest.json, as the workflow writes it: each list's text as it is now, by
// its SHA-256, and the patches there are to it.
type Manifest struct {
	Format    int              `json:"format"`
	Published time.Time        `json:"published"`
	Lists     map[string]Entry `json:"lists"`
	Resources map[string]Entry `json:"resources"`
}

// Entry is one file's: the SHA-256 and size of its text, uncompressed; the release's file
// of it, zstd; when it last changed; and the patches to it, by the SHA-256 of the text each
// starts from.
type Entry struct {
	SHA256  string            `json:"sha256"`
	Size    int64             `json:"size"`
	File    string            `json:"file"`
	Changed time.Time         `json:"changed"`
	Patches map[string]string `json:"patches"`
}

var (
	sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)
	// A release file's name: no folders, nothing hidden.
	fileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,200}$`)
	// A list's id, which names its files on disk.
	fileID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,80}$`)
)

// maxText is the largest text a list may be: the largest lists are a few megabytes.
const maxText = 64 << 20

// Verify checks that sig, manifest.json.sig as the release has it (its Ed25519 signature
// in standard base64), signs manifest's exact bytes with key, and only then reads it.
func Verify(key ed25519.PublicKey, manifest, sig []byte) (*Manifest, error) {
	raw, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(sig)))
	if err != nil || len(raw) != ed25519.SignatureSize {
		return nil, errors.New("the manifest's signature isn't one")
	}
	if !ed25519.Verify(key, manifest, raw) {
		return nil, errors.New("the manifest isn't signed with the lists' key")
	}
	var m Manifest
	if err := json.Unmarshal(manifest, &m); err != nil {
		return nil, fmt.Errorf("the manifest: %v", err)
	}
	if m.Format != 1 {
		return nil, fmt.Errorf("the manifest is format %d, not 1", m.Format)
	}
	for _, set := range []map[string]Entry{m.Lists, m.Resources} {
		for id, e := range set {
			if !fileID.MatchString(id) || !sha256Hex.MatchString(e.SHA256) || !fileName.MatchString(e.File) || e.Size < 0 || e.Size > maxText {
				return nil, fmt.Errorf("the manifest's %q isn't one", id)
			}
			for from, p := range e.Patches {
				if !sha256Hex.MatchString(from) || !fileName.MatchString(p) {
					return nil, fmt.Errorf("the manifest's patch of %q isn't one", id)
				}
			}
		}
	}
	return &m, nil
}
