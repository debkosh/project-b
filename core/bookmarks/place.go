package bookmarks

import (
	"math/big"
	"strings"
)

// A place is an entry's place among its folder's entries: a key that sorts as text, so an
// entry goes between two others by taking a key between theirs, and moving one changes that
// one alone (docs/bookmark-folders.md). Keys are base-36 digits with no trailing zero, so
// there is always a key between two.
const digits = "0123456789abcdefghijklmnopqrstuvwxyz"

// between is a key after a and before b, "" meaning no bound on that side: a < between < b.
// At either end it steps a digit, so a folder whose entries keep going first or last grows
// its keys by a digit only every 35 or so.
func between(a, b string) string {
	switch {
	case a == "" && b == "":
		return "i"
	case b == "":
		if d := strings.IndexByte(digits, a[0]); d < len(digits)-1 {
			return digits[d+1 : d+2]
		}
		if len(a) == 1 {
			return "z1"
		}
		return "z" + between(a[1:], "")
	case a == "":
		switch d := strings.IndexByte(digits, b[0]); {
		case d > 1:
			return digits[d-1 : d]
		case d == 1 && len(b) > 1:
			return "1"
		case d == 1:
			return "0z"
		default:
			return "0" + between("", b[1:])
		}
	}
	// What the two share stays.
	n := 0
	for n < len(b) && digitAt(a, n) == b[n] {
		n++
	}
	if n > 0 {
		return b[:n] + between(tail(a, n), b[n:])
	}
	lo, hi := strings.IndexByte(digits, a[0]), strings.IndexByte(digits, b[0])
	if hi-lo > 1 {
		return digits[(lo+hi+1)/2 : (lo+hi+1)/2+1]
	}
	// No digit between them: b's first digit is before b when b goes on, else a's first
	// digit and a key after the rest of a.
	if len(b) > 1 {
		return b[:1]
	}
	return digits[lo:lo+1] + between(tail(a, 1), "")
}

// spread is n keys in order, evenly apart and all the same length but for trailing zeros,
// for a folder's entries placed at once.
func spread(n int) []string {
	width := 1
	space := big.NewInt(36)
	for space.Cmp(big.NewInt(int64(n+1))) <= 0 {
		width++
		space.Mul(space, big.NewInt(36))
	}
	out := make([]string, n)
	for i := range out {
		k := new(big.Int).Mul(space, big.NewInt(int64(i+1)))
		k.Div(k, big.NewInt(int64(n+1)))
		s := k.Text(36)
		out[i] = strings.TrimRight(strings.Repeat("0", width-len(s))+s, "0")
	}
	return out
}

func digitAt(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return '0'
}

func tail(s string, n int) string {
	if n < len(s) {
		return s[n:]
	}
	return ""
}
