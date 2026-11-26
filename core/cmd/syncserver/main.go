// A sync server for checking the real app on a device (tools/drive_android.py): synckosh's
// server over https, as the memd server serves a bucket, at /sync/<token> or at /sync with
// the token as a bearer, its store a bolt file. Its certificate is made at each start by a
// CA of its own, written to <dir>/ca.pem for the device under test to trust: sync takes only
// https, and this is how a test server is https. It prints the address it listens on, then a
// line for each request.
//
//	syncserver -addr 127.0.0.1:0 -dir <folder>
//
// See docs/sync.md.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/debkosh/synckosh/bolt"
	"github.com/debkosh/synckosh/server"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "listen address; port 0 takes a free one, which it prints")
	dir := flag.String("dir", "", "the folder for its store and ca.pem")
	flag.Parse()
	if *dir == "" {
		log.Fatal("-dir is needed")
	}
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		log.Fatal(err)
	}
	cert, err := certificate(filepath.Join(*dir, "ca.pem"))
	if err != nil {
		log.Fatal(err)
	}
	data, err := bolt.OpenStore(filepath.Join(*dir, "server.db"))
	if err != nil {
		log.Fatal(err)
	}
	srv := server.New(data, server.Bearer(func(token string) (string, error) { return "bucket-" + token, nil }))
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("sync server on https://%s\n", l.Addr())
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("request %s %s\n", r.Method, r.URL.Path)
		rest, ok := strings.CutPrefix(r.URL.Path, "/sync/")
		if !ok {
			http.NotFound(w, r)
			return
		}
		r = r.Clone(r.Context())
		r.URL.Path, r.URL.RawPath = "/"+rest, ""
		if token, path, ok := strings.Cut(rest, "/"); ok && token != "v1" {
			r.URL.Path = "/" + path
			r.Header.Set("Authorization", "Bearer "+token)
		}
		srv.ServeHTTP(w, r)
	})
	s := &http.Server{Handler: h, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}}
	log.Fatal(s.ServeTLS(l, "", ""))
}

// certificate makes a CA, writes it to caPath, and returns a certificate it signed for this
// machine: localhost and 127.0.0.1.
func certificate(caPath string) (tls.Certificate, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Project B sync test CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(7 * 24 * time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o644); err != nil {
		return tls.Certificate{}, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	leafTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(7 * 24 * time.Hour), DNSNames: []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, ca, &key.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: key}, nil
}
