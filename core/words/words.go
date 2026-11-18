// Package words makes passphrases of words, as Settings' Suggest one offers for sync
// (docs/sync.md): common words from a list of its own, each picked at random.
package words

import (
	"crypto/rand"
	_ "embed"
	"math/big"
	"strings"
)

//go:embed words.txt
var list string

// All is the list, a word to a line: short, common, lower case.
var All = strings.Fields(list)

// Passphrase is n words picked at random from All, with a space between each.
func Passphrase(n int) string {
	out := make([]string, n)
	for i := range out {
		at, err := rand.Int(rand.Reader, big.NewInt(int64(len(All))))
		if err != nil {
			panic(err)
		}
		out[i] = All[at.Int64()]
	}
	return strings.Join(out, " ")
}
