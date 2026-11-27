package downloads

import (
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Naming a download (docs/daily-driver.md, "Naming downloads"). A file keeps the name its
// server gave it unless that name says nothing, an id or a word like "video": then it is
// named after better text, the title a page's video downloader passed for it, the page's
// title, or the page's host and the words of its path. Only the name ever comes from a
// page: the ending is the file's own, and what a file name may not hold is taken out.

// How downloads are named: Settings' Name downloads, and each site's own.
const (
	// NamesAuto keeps a file's name unless it looks random.
	NamesAuto = "auto"
	// NamesTitle names every file "Title – original.ext".
	NamesTitle = "title"
	// NamesFile never renames.
	NamesFile = "file"
)

// Naming is what a download is named by, beside the name its server gave it: the mode, the
// title a page's link carried for it (#pb-download=<title>), the title of the page it came
// from, and that page's address. Any may be empty.
type Naming struct {
	Mode   string
	Linked string
	Title  string
	Page   string
}

// nameLimit is about how long a name's own part may be, in characters.
const nameLimit = 120

// generic are the words a streaming server names its files by, which say nothing.
var generic = map[string]bool{
	"video": true, "index": true, "master": true, "playlist": true, "download": true, "file": true,
	"media": true, "stream": true, "output": true, "chunk": true, "seg": true, "segment": true,
}

var uuid = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// LooksRandom says a file's name, without its ending, says nothing to a person: every part
// of it, split at - _ . and spaces, is digits, or hex of 8 characters or more, or a UUID, or
// a jumble of 16 or more letters and digits, or one of the generic words.
func LooksRandom(stem string) bool {
	stem = uuid.ReplaceAllString(stem, " ")
	// Brackets too, as round a copy's number: "index (1)".
	for _, part := range strings.FieldsFunc(stem, func(r rune) bool { return strings.ContainsRune("-_.()[]", r) || unicode.IsSpace(r) }) {
		if !randomPart(part) {
			return false
		}
	}
	return true
}

func randomPart(part string) bool {
	if generic[strings.ToLower(part)] {
		return true
	}
	digits, hex, alnum := true, true, true
	letters, numbers := 0, 0
	for _, r := range part {
		switch {
		case r >= '0' && r <= '9':
			numbers++
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
			letters++
			digits = false
			if l := unicode.ToLower(r); l > 'f' {
				hex = false
			}
		default:
			digits, hex, alnum = false, false, false
		}
	}
	switch {
	case digits:
		return true
	case hex && len(part) >= 8:
		return true
	}
	// A jumble: long, letters and digits both, about 4 bits to a character, and no run of
	// it that reads as a word.
	return alnum && len(part) >= 16 && letters > 0 && numbers > 0 && entropy(part) >= 3.5 && !wordLike(part)
}

// entropy is the Shannon entropy of a string's characters, in bits to a character.
func entropy(s string) float64 {
	count := map[rune]float64{}
	for _, r := range s {
		count[r]++
	}
	var bits float64
	n := float64(utf8.RuneCountInString(s))
	for _, c := range count {
		bits -= c / n * math.Log2(c/n)
	}
	return bits
}

// wordLike says a string holds a run of five letters or more that reads as a word: in one
// case, or a capital and then small letters, with two vowels or more and never four
// consonants in a row. A random id's letters change case and have few vowels.
func wordLike(s string) bool {
	runs := strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) })
	for _, run := range runs {
		// A change from small to capital starts a new run, as camelCase does.
		start := 0
		for i := 1; i <= len(run); i++ {
			if i == len(run) || unicode.IsUpper(rune(run[i])) && unicode.IsLower(rune(run[i-1])) {
				if reads(run[start:i]) {
					return true
				}
				start = i
			}
		}
	}
	return false
}

func reads(run string) bool {
	if len(run) < 5 {
		return false
	}
	rest := run[1:]
	if rest != strings.ToLower(rest) && run != strings.ToUpper(run) {
		return false
	}
	vowels, consonants := 0, 0
	for _, r := range strings.ToLower(run) {
		if strings.ContainsRune("aeiouy", r) {
			vowels++
			consonants = 0
		} else if consonants++; consonants >= 4 {
			return false
		}
	}
	return vowels >= 2
}

// LinkedTitle is the title a download's address carries for it after #pb-download=, as a
// page's video downloader writes it, or "".
func LinkedTitle(address string) string {
	title, _ := linked(address)
	return title
}

// LinkedQuality is the height a download's address asks its stream to be saved at, the
// &q=<height> that ends its #pb-download=<title>, or 0 for none: the best.
func LinkedQuality(address string) int {
	_, height := linked(address)
	return height
}

// linked reads what an address carries after #pb-download=: a title, percent-encoded, and
// then, or alone, &q= and a height in digits, the last thing in it. A title's own & is
// percent-encoded, so one that isn't starts the quality only where digits alone follow.
func linked(address string) (title string, height int) {
	u, err := url.Parse(address)
	if err != nil {
		return "", 0
	}
	// The fragment as written: its title is percent-encoded.
	raw, ok := strings.CutPrefix(u.EscapedFragment(), "pb-download=")
	if !ok {
		return "", 0
	}
	if at := strings.LastIndex(raw, "&q="); at >= 0 {
		// No more digits than a height has, and never 0.
		if q := raw[at+len("&q="):]; len(q) <= 5 {
			if n, err := strconv.ParseUint(q, 10, 32); err == nil && n > 0 {
				raw, height = raw[:at], int(n)
			}
		}
	}
	title, err = url.PathUnescape(raw)
	if err != nil {
		return "", height
	}
	return title, height
}

// Name is the name a download whose server calls it original goes by, and whether that is
// another name. The ending is always original's.
func (n Naming) Name(original string) (string, bool) {
	ext := filepath.Ext(original)
	stem := strings.TrimSuffix(original, ext)
	var name string
	switch n.Mode {
	case NamesFile:
		return original, false
	case NamesTitle:
		title := first(cleanTitle(n.Linked, nameLimit), cleanTitle(n.Title, nameLimit))
		if title == "" || title == stem {
			return original, false
		}
		name = title + " – " + stem
	default:
		if !LooksRandom(stem) {
			return original, false
		}
		name = first(cleanTitle(n.Linked, nameLimit), cleanTitle(n.Title, nameLimit), cleanTitle(pageWords(n.Page), nameLimit))
	}
	if name == "" || name == stem {
		return original, false
	}
	return name + ext, true
}

func first(of ...string) string {
	for _, s := range of {
		if s != "" {
			return s
		}
	}
	return ""
}

// pageWords is a page's host and the words of its path, for a page with no title: the
// parts of the path that say something.
func pageWords(page string) string {
	u, err := url.Parse(page)
	if err != nil || u.Hostname() == "" || u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	words := []string{strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")}
	for _, part := range strings.FieldsFunc(u.Path, func(r rune) bool { return r == '/' || r == '-' || r == '_' || r == '.' || unicode.IsSpace(r) }) {
		if !randomPart(part) && len(part) > 1 {
			words = append(words, part)
		}
	}
	return strings.Join(words, " ")
}

// cleanTitle is text made fit to be a file's name, at most limit characters: no slashes,
// no control characters, none of the characters a file system keeps for itself, spaces
// run together, and no dot or space at either end.
func cleanTitle(text string, limit int) string {
	var b strings.Builder
	space := false
	for _, r := range text {
		switch {
		case r == utf8.RuneError || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || strings.ContainsRune(`/\:*?"<>|`, r) || unicode.IsSpace(r):
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	out := strings.Trim(b.String(), " .")
	if utf8.RuneCountInString(out) > limit {
		out = strings.Trim(string([]rune(out)[:limit]), " .")
	}
	return out
}
