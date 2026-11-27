// Command lists publishes Project B's blocking lists to a GitHub release, as
// .github/workflows/lists.yml runs it every hour (docs/blocking.md, "Building and publishing
// them"): it reads the catalogue, downloads every source and the resources, and writes into
// -out the files that changed, a patch to each from its last eight versions, and the signed
// manifest; -keep gets the name of every file the release keeps, and the workflow deletes the
// rest. When nothing changed, -out stays empty.
//
//	LISTS_SIGNING_KEY=<base64 seed> go run . -catalog ../../core/blocking/catalog.json \
//	    -out out -keep keep.txt -prev gh:lists -repo-root ../..
//	go run . -keygen    # a new key pair: the seed for the secret, the public key (hex) for core
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	catalog := flag.String("catalog", "core/blocking/catalog.json", "the catalogue")
	out := flag.String("out", "out", "where the new files go, for the upload")
	keep := flag.String("keep", "", "where the names of the files the release keeps go")
	prev := flag.String("prev", "", "the release published so far: gh:<tag> (with GITHUB_REPOSITORY), an https base address, or dir:<folder>")
	repoRoot := flag.String("repo-root", ".", "the checkout, which our own lists' sources are read from")
	repoRaw := flag.String("repo-raw", "https://raw.githubusercontent.com/debkosh/project-b/main/", "our own lists' address, read from -repo-root instead")
	resources := flag.String("resources", "https://raw.githubusercontent.com/brave/adblock-resources/master/dist/resources.json", "adblock-rust's resources JSON")
	own := flag.String("own-resources", "tools/lists/resources", "a folder of our own scriptlets, *.js")
	keygen := flag.Bool("keygen", false, "print a new signing key's seed and public key, and stop")
	flag.Parse()
	log.SetFlags(0)

	if *keygen {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("LISTS_SIGNING_KEY=%s\npublic=%s\n", base64.StdEncoding.EncodeToString(priv.Seed()), hex.EncodeToString(pub))
		return
	}
	key, err := signingKey(os.Getenv("LISTS_SIGNING_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	// In hex, as core/blocking/manifest.go has it.
	log.Printf("signing with the key whose public half is %s", hex.EncodeToString(key.Public().(ed25519.PublicKey)))
	cat, err := loadCatalog(*catalog)
	if err != nil {
		log.Fatal(err)
	}
	client := &http.Client{Timeout: 90 * time.Second}
	var st store
	switch {
	case strings.HasPrefix(*prev, "gh:"):
		repo := os.Getenv("GITHUB_REPOSITORY")
		if repo == "" {
			log.Fatal("-prev gh:<tag> needs GITHUB_REPOSITORY")
		}
		st = ghStore{repo: repo, tag: strings.TrimPrefix(*prev, "gh:")}
	case strings.HasPrefix(*prev, "dir:"):
		st = dirStore(strings.TrimPrefix(*prev, "dir:"))
	case strings.HasPrefix(*prev, "https://"):
		st = httpStore{base: strings.TrimSuffix(*prev, "/") + "/", client: client, max: maxTextSize}
	default:
		log.Fatal("-prev: gh:<tag>, dir:<folder> or an https address")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	changed, err := run(ctx, config{
		catalog: cat, out: *out, keep: *keep, prev: st, workers: 8,
		fetch:        &fetcher{client: client, maxSize: 64 << 20, repoRaw: *repoRaw, repoRoot: *repoRoot, retryWait: 10 * time.Second},
		resourcesURL: *resources, ownResources: *own, key: key, now: time.Now,
	})
	if err != nil {
		log.Fatal(err)
	}
	if changed {
		fmt.Println("RESULT=changed")
	} else {
		fmt.Println("RESULT=unchanged")
	}
}

// signingKey is the private key from its 32-byte seed in base64 (the kosh's lists-signing.key).
func signingKey(s string) (ed25519.PrivateKey, error) {
	if s == "" {
		return nil, errors.New("LISTS_SIGNING_KEY is not set")
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("LISTS_SIGNING_KEY is not a 32-byte seed in base64")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}
