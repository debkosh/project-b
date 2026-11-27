package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// checkWith is the check that runs program on a list's text in a file: core/blocking/lib's
// check example, which compiles it with the library the app pins and fails on a list it
// can't use. No program, no check.
func checkWith(program string) func(id string, text []byte) error {
	if program == "" {
		return nil
	}
	return func(id string, text []byte) error {
		dir, err := os.MkdirTemp("", "lists-check-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		path := filepath.Join(dir, id+".txt")
		if err := os.WriteFile(path, text, 0o644); err != nil {
			return err
		}
		out, err := exec.Command(program, path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %v: %s", filepath.Base(program), err, strings.TrimSpace(string(out)))
		}
		return nil
	}
}

// checkLists runs the check on each list whose text changed; one it fails keeps its previous
// version, as a failed download does.
func checkLists(c config, items []item, prev previous) {
	if c.check == nil {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, c.workers)
	for i, it := range items {
		if it.text == nil {
			continue
		}
		sum := sha256.Sum256(it.text)
		if old := prev.Lists[it.id]; old != nil && old.SHA256 == hex.EncodeToString(sum[:]) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := c.check(it.id, it.text); err != nil {
				log.Printf("%s: keeping the previous text: the library doesn't take it: %v", it.id, err)
				items[i] = item{id: it.id, ext: it.ext}
			}
		}()
	}
	wg.Wait()
}
