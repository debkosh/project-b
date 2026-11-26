package main

import (
	"strings"

	"github.com/debkosh/project-b/browserdata"
)

// keychainOf is -keychain: a login keychain that gives secret for every browser, or nil,
// the Mac's own. An item -keychain-items names as <name>=<its secret> gives that secret
// instead: another browser's key, which opens nothing of the one being brought.
func keychainOf(secret, names string) func(string, string) ([]byte, error) {
	if secret == "" {
		return nil
	}
	own := map[string]string{}
	for name := range strings.SplitSeq(names, ",") {
		if n, s, ok := strings.Cut(strings.TrimSpace(name), "="); ok {
			own[n+" Safe Storage"] = s
		}
	}
	return func(service, _ string) ([]byte, error) {
		if s, ok := own[service]; ok {
			return []byte(s), nil
		}
		return []byte(secret), nil
	}
}

// keychainItemsOf is -keychain-items: with -keychain, the keychain's list of Safe Storage
// items, one for each name, none when no name is given; without it, nil, the Mac's own.
func keychainItemsOf(secret, names string) func() ([]browserdata.KeychainItem, error) {
	if secret == "" {
		return nil
	}
	var items []browserdata.KeychainItem
	for name := range strings.SplitSeq(names, ",") {
		if name, _, _ = strings.Cut(strings.TrimSpace(name), "="); name != "" {
			items = append(items, browserdata.KeychainItem{Service: name + " Safe Storage", Account: name})
		}
	}
	return func() ([]browserdata.KeychainItem, error) { return items, nil }
}
