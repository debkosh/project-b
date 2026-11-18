package browserdata

import (
	"encoding/csv"
	"errors"
	"html"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// What a person exported from a browser to a file: bookmarks as the HTML page every browser
// exports (Netscape's bookmark file), and passwords as CSV.

// ErrNotAFile is what a reader says of a file that isn't what it reads.
var ErrNotAFile = errors.New("browserdata: not a file of this kind")

// ReadPasswordsCSV reads passwords exported as CSV: Chrome's, Edge's and Brave's (name,
// url, username, password, note), Firefox's (url, username, password, httpRealm, …,
// timeCreated, timeLastUsed) and the Passwords app's, Safari's (Title, URL, Username,
// Password, Notes, OTPAuth). Columns are found by their names, in any order and case. An
// HTTP auth login (Firefox's httpRealm) and a row with no password are left out.
func ReadPasswordsCSV(r io.Reader) ([]Login, error) {
	c := csv.NewReader(r)
	c.FieldsPerRecord = -1
	header, err := c.Read()
	if err != nil {
		return nil, ErrNotAFile
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))] = i
	}
	find := func(names ...string) int {
		for _, n := range names {
			if i, ok := col[n]; ok {
				return i
			}
		}
		return -1
	}
	url, user, pass := find("url", "login_uri", "website"), find("username", "login_username"), find("password", "login_password")
	realm, created, used := find("httprealm"), find("timecreated"), find("timelastused")
	if url < 0 || pass < 0 {
		return nil, ErrNotAFile
	}
	field := func(rec []string, i int) string {
		if i < 0 || i >= len(rec) {
			return ""
		}
		return rec[i]
	}
	var out []Login
	for {
		rec, err := c.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		if field(rec, realm) != "" || field(rec, pass) == "" || field(rec, url) == "" {
			continue
		}
		l := Login{Origin: field(rec, url), Username: field(rec, user), Password: field(rec, pass)}
		if ms, err := strconv.ParseInt(field(rec, created), 10, 64); err == nil {
			l.Created = millis(ms)
		}
		if ms, err := strconv.ParseInt(field(rec, used), 10, 64); err == nil {
			l.LastUsed = millis(ms)
		}
		out = append(out, l)
	}
}

var netscapeTag = regexp.MustCompile(`(?is)<(/?)(dl|h3|a)\b([^>]*)>`)
var netscapeAttr = regexp.MustCompile(`(?is)\b(href|add_date)\s*=\s*"([^"]*)"`)

// ReadBookmarksHTML reads bookmarks exported as Netscape's bookmark file, the HTML every
// browser exports: its folders (H3, then a DL of what they hold) and bookmarks (A, with
// HREF and ADD_DATE in seconds), in their order.
func ReadBookmarksHTML(r io.Reader) ([]Bookmark, error) {
	b, err := io.ReadAll(io.LimitReader(r, 64<<20))
	if err != nil {
		return nil, err
	}
	text := string(b)
	if !strings.Contains(strings.ToUpper(text[:min(len(text), 4096)]), "NETSCAPE-BOOKMARK-FILE") {
		return nil, ErrNotAFile
	}
	root := &Bookmark{}
	stack := []*Bookmark{root}
	var pending *Bookmark // a folder named, whose DL comes next
	tags := netscapeTag.FindAllStringSubmatchIndex(text, -1)
	// textAfter is what is written after the tag that ends at i, up to the next tag.
	textAfter := func(i int) string {
		end := strings.IndexByte(text[i:], '<')
		if end < 0 {
			end = len(text) - i
		}
		return strings.TrimSpace(html.UnescapeString(text[i : i+end]))
	}
	for _, t := range tags {
		closing, name, attrs := text[t[2]:t[3]] == "/", strings.ToLower(text[t[4]:t[5]]), text[t[6]:t[7]]
		top := stack[len(stack)-1]
		switch {
		case name == "dl" && !closing:
			// A folder's list; one with no folder named before it, as the file's first, is
			// the one it is in. A folder on the stack is in its parent's Children, which
			// grows only once it is closed, so the pointer holds.
			if pending != nil {
				stack = append(stack, pending)
				pending = nil
			} else {
				stack = append(stack, top)
			}
		case name == "dl" && closing:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case name == "h3" && !closing:
			top.Children = append(top.Children, Bookmark{Title: textAfter(t[1])})
			pending = &top.Children[len(top.Children)-1]
		case name == "a" && !closing:
			bm := Bookmark{Title: textAfter(t[1])}
			for _, a := range netscapeAttr.FindAllStringSubmatch(attrs, -1) {
				switch strings.ToLower(a[1]) {
				case "href":
					bm.URL = html.UnescapeString(a[2])
				case "add_date":
					if s, err := strconv.ParseInt(a[2], 10, 64); err == nil && s > 0 {
						bm.Added = time.Unix(s, 0).UTC()
					}
				}
			}
			if bm.URL != "" && !strings.HasPrefix(strings.ToLower(bm.URL), "javascript:") {
				top.Children = append(top.Children, bm)
			}
		}
	}
	return root.Children, nil
}
