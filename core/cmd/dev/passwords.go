package main

import (
	"strings"

	"github.com/sudiptadeb/project-b/browserdata"
)

// keychainOf is -keychain: a login keychain that gives secret for every browser, or nil,
// the Mac's own.
func keychainOf(secret string) func(string, string) ([]byte, error) {
	if secret == "" {
		return nil
	}
	return func(string, string) ([]byte, error) { return []byte(secret), nil }
}

// keychainItemsOf is -keychain-items: with -keychain, the keychain's list of Safe Storage
// items, one for each name, none when no name is given; without it, nil, the Mac's own.
func keychainItemsOf(secret, names string) func() ([]browserdata.KeychainItem, error) {
	if secret == "" {
		return nil
	}
	var items []browserdata.KeychainItem
	for name := range strings.SplitSeq(names, ",") {
		if name = strings.TrimSpace(name); name != "" {
			items = append(items, browserdata.KeychainItem{Service: name + " Safe Storage", Account: name})
		}
	}
	return func() ([]browserdata.KeychainItem, error) { return items, nil }
}
