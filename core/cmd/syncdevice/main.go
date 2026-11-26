// Another device for checking sync on a real one (tools/drive_android.py): a profile of its
// own, in a folder made for the run, that saves one login and turns sync on with a bucket,
// choosing its passphrase, as a person's first device does. The device under test then
// joins the bucket with that passphrase and is brought the login. It trusts the test
// server's CA (core/cmd/syncserver's ca.pem), prints "synced" once its first run is done,
// and ends.
//
//	syncdevice -address https://127.0.0.1:<port>/sync/<token> -ca <folder>/ca.pem \
//	  -passphrase "<words>" -login "https://example.com|asha|<password>" [-login ...]
//
// See docs/passwords.md.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/debkosh/project-b/browserdata"

	"projectb/keys"
	"projectb/passwords"
	"projectb/store"
	"projectb/sync"
)

func main() {
	address := flag.String("address", "", "the bucket's address, its token as the last part")
	ca := flag.String("ca", "", "the test server's ca.pem")
	passphrase := flag.String("passphrase", "", "the sync passphrase this device chooses")
	var saved []browserdata.Login
	flag.Func("login", "a login to save, as origin|username|password; given once for each", func(s string) error {
		parts := strings.SplitN(s, "|", 3)
		if len(parts) != 3 {
			return errors.New("origin|username|password")
		}
		saved = append(saved, browserdata.Login{Origin: parts[0], Username: parts[1], Password: parts[2]})
		return nil
	})
	flag.Parse()

	pem, err := os.ReadFile(*ca)
	if err != nil {
		log.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		log.Fatalf("%s holds no certificate", *ca)
	}
	sync.HTTPClient = &http.Client{Timeout: time.Minute, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots}}}

	dir, err := os.MkdirTemp("", "syncdevice")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	st, err := store.Open(dir)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()
	// What the app posts to its main thread runs here on one goroutine, in order.
	posted := make(chan func(), 1024)
	go func() {
		for f := range posted {
			f()
		}
	}()
	k := keys.New(dir, "", st, nil)
	logins := passwords.New(st, k, func(context.Context, string) error { return nil })
	m := sync.New(st, k, func(f func()) { posted <- f }, logins.Synced())
	defer m.Stop()
	if added, _, _ := logins.Import(saved); added != len(saved) || added == 0 {
		log.Fatalf("%d of %d logins were kept", added, len(saved))
	}
	connected := make(chan error, 1)
	m.Connect(*address, "", *passphrase, func(err error) { connected <- err })
	if err := <-connected; err != nil {
		log.Fatal(err)
	}
	for deadline := time.Now().Add(time.Minute); ; time.Sleep(100 * time.Millisecond) {
		s := m.Status()
		if s.Error != "" {
			log.Fatal(s.Error)
		}
		if s.LastSynced != 0 && !s.Syncing {
			break
		}
		if time.Now().After(deadline) {
			log.Fatal("the first run didn't end in a minute")
		}
	}
	fmt.Println("synced")
}
