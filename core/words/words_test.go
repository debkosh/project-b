package words

import (
	"regexp"
	"strings"
	"testing"
)

// The list is long enough for five words to be a strong passphrase, each word once, short
// and plain to type.
func TestTheList(t *testing.T) {
	if len(All) < 512 {
		t.Fatalf("%d words", len(All))
	}
	seen := map[string]bool{}
	plain := regexp.MustCompile(`^[a-z]{3,9}$`)
	for _, w := range All {
		if seen[w] || !plain.MatchString(w) {
			t.Errorf("%q twice or not plain", w)
		}
		seen[w] = true
	}
}

// A passphrase is n words of the list, a different one each time.
func TestPassphrase(t *testing.T) {
	a, b := Passphrase(5), Passphrase(5)
	if a == b {
		t.Fatalf("the same twice: %q", a)
	}
	for _, w := range strings.Split(a, " ") {
		if !strings.Contains("\n"+list, "\n"+w+"\n") {
			t.Fatalf("%q isn't in the list: %q", w, a)
		}
	}
	if n := len(strings.Split(a, " ")); n != 5 {
		t.Fatalf("%d words: %q", n, a)
	}
}
