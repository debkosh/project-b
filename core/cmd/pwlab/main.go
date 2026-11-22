// pwlab is a test bench for the form rules (docs/password-autofill.md, Carriers): a dummy
// password manager on this computer alone, for trying core/forms on real sites in any
// Chrome, with no Chromium build. It writes out a test extension, whose content script is
// the real form script and whose service worker is the carrier, and answers it with Go's
// real rules over a list of test logins. Its popup shows what Go made of each frame's
// fields and fills a test login where a sign-in form was found.
//
// It keeps no real password: its logins are the ones it is started with, and what a page's
// form held when it was sent is told by its length alone.
//
//	pwlab -extension <folder> [-addr 127.0.0.1:7391] [-logins logins.json] [-anywhere user:password]
//
// Then, in Chrome: Extensions, Developer mode, Load unpacked, that folder.
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"

	"projectb/forms"
)

//go:embed extension
var extension embed.FS

// login is a test login.
type login struct {
	Origin   string `json:"origin"`
	Username string `json:"username"`
	Password string `json:"password"`
	// anywhere is one offered on every site, for trying a fill where no login is saved.
	anywhere bool
}

// frame is what Go holds of one document.
type frame struct {
	stamp  forms.Stamp
	fields forms.Fields
	found  []forms.Found
}

type lab struct {
	mu     sync.Mutex
	logins []login
	frames map[string]*frame // by tab and frame
	logs   map[string][]string
}

func main() {
	addr := flag.String("addr", "127.0.0.1:7391", "where the extension reaches it; this computer only")
	dir := flag.String("extension", "", "the folder to write the test extension to")
	file := flag.String("logins", "", "test logins, a JSON list of {origin, username, password}")
	anywhere := flag.String("anywhere", "", "a made-up login offered on every site, as user:password")
	flag.Parse()

	host, port, err := net.SplitHostPort(*addr)
	if err != nil || (host != "127.0.0.1" && host != "localhost") {
		log.Fatalf("-addr %q: it listens on this computer alone, 127.0.0.1:<port>", *addr)
	}
	l := &lab{frames: map[string]*frame{}, logs: map[string][]string{}}
	if *file != "" {
		b, err := os.ReadFile(*file)
		if err == nil {
			err = json.Unmarshal(b, &l.logins)
		}
		if err != nil {
			log.Fatalf("-logins: %v", err)
		}
	}
	if user, pass, ok := strings.Cut(*anywhere, ":"); ok {
		l.logins = append(l.logins, login{Username: user, Password: pass, anywhere: true})
	}
	if *dir != "" {
		if err := writeExtension(*dir, port); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("the test extension is in %s: in Chrome, Extensions, Developer mode, Load unpacked\n", *dir)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/page", l.page)
	mux.HandleFunc("/tab", l.tab)
	mux.HandleFunc("/fill", l.fill)
	fmt.Printf("pwlab on http://%s, with %d test logins\n", *addr, len(l.logins))
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// writeExtension writes the test extension to dir: its own files, and the real form script.
func writeExtension(dir, port string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files, err := fs.ReadDir(extension, "extension")
	if err != nil {
		return err
	}
	for _, f := range files {
		b, err := extension.ReadFile("extension/" + f.Name())
		if err != nil {
			return err
		}
		b = []byte(strings.ReplaceAll(string(b), "__PORT__", port))
		if err := os.WriteFile(filepath.Join(dir, f.Name()), b, 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, "forms.js"), []byte(forms.Script), 0o644)
}

func (l *lab) say(tab, format string, args ...any) {
	line := time.Now().Format("15:04:05 ") + fmt.Sprintf(format, args...)
	fmt.Println(line)
	l.logs[tab] = append(l.logs[tab], line)
	if len(l.logs[tab]) > 40 {
		l.logs[tab] = l.logs[tab][len(l.logs[tab])-40:]
	}
}

// page takes a message of the page side's, stamped by the carrier, and answers with Go's.
func (l *lab) page(w http.ResponseWriter, r *http.Request) {
	var m struct {
		Stamp   forms.Stamp     `json:"stamp"`
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if json.NewDecoder(r.Body).Decode(&m) != nil {
		http.Error(w, "a message: {stamp, type, payload}", http.StatusBadRequest)
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	key := m.Stamp.Tab + ":" + m.Stamp.Frame
	if m.Type == "left" {
		// The document has gone, and what Go held of it with it, unless another is there now.
		if f := l.frames[key]; f != nil && f.stamp.Document == m.Stamp.Document {
			delete(l.frames, key)
		}
		json.NewEncoder(w).Encode(map[string]any{})
		return
	}
	f := l.frames[key]
	if f == nil || f.stamp.Document != m.Stamp.Document {
		f = &frame{}
		l.frames[key] = f
	}
	f.stamp = m.Stamp
	type out struct {
		Type    string `json:"type"`
		Payload any    `json:"payload"`
	}
	var answer []out
	where := m.Stamp.Origin
	if !m.Stamp.Top {
		where += " (a frame)"
	}
	switch m.Type {
	case "fields":
		json.Unmarshal(m.Payload, &f.fields)
		f.found = forms.Roles(f.fields, nil)
		// Go watches only the fields it took for a username or a password.
		watch := forms.Watch{Refs: []string{}}
		kinds := []string{}
		for _, found := range f.found {
			kinds = append(kinds, string(found.Kind))
			for ref, role := range found.Roles {
				if role != forms.OneTimeCode {
					watch.Refs = append(watch.Refs, ref)
				}
			}
		}
		answer = append(answer, out{"watch", watch})
		l.say(m.Stamp.Tab, "%s: %d fields; %s", where, len(f.fields.Fields), orNone(strings.Join(kinds, ", ")))
	case "pressed":
		var p forms.Pressed
		json.Unmarshal(m.Payload, &p)
		role := "no role"
		for _, found := range f.found {
			if r := found.Roles[p.Ref]; r != "" {
				role = string(r) + " of a " + string(found.Kind)
			}
		}
		l.say(m.Stamp.Tab, "%s: pressed %s, %s", where, describe(f.fields, p.Ref), role)
	case "submitted", "gone":
		var s forms.Sent
		json.Unmarshal(m.Payload, &s)
		// With what the fields held, as a save would be decided. A password is told by its
		// length alone.
		for _, found := range forms.Roles(f.fields, s.Values) {
			if found.Form != s.Form {
				continue
			}
			secret := found.New
			if secret == "" {
				secret = found.Current
			}
			l.say(m.Stamp.Tab, "%s: a %s was %s, username %q, a password of %d letters: this is where saving would be offered, once the sign-in is seen to go through",
				where, found.Kind, map[string]string{"submitted": "sent", "gone": "sent and its form went"}[m.Type], s.Values[found.Username], len([]rune(s.Values[secret])))
		}
	case "filled":
		var done forms.Filled
		json.Unmarshal(m.Payload, &done)
		l.say(m.Stamp.Tab, "%s: filled %d fields", where, len(done.Refs))
	}
	json.NewEncoder(w).Encode(map[string]any{"messages": answer})
}

func orNone(s string) string {
	if s == "" {
		return "no form with a role"
	}
	return s
}

// describe is a field as a person would know it from the page's source.
func describe(page forms.Fields, ref string) string {
	for _, f := range page.Fields {
		if f.Ref != ref {
			continue
		}
		parts := []string{"<input"}
		for _, kv := range [][2]string{{"type", f.Type}, {"name", f.Name}, {"id", f.ID}, {"autocomplete", f.Autocomplete}} {
			if kv[1] != "" {
				parts = append(parts, kv[0]+"="+kv[1])
			}
		}
		s := strings.Join(parts, " ") + ">"
		if label := firstOf(f.Label, f.AriaLabel, f.Placeholder, f.Nearby); label != "" {
			s += " “" + label + "”"
		}
		if !f.Visible {
			s += " (out of sight)"
		}
		return s
	}
	return ref
}

func firstOf(all ...string) string {
	for _, s := range all {
		if s != "" {
			return s
		}
	}
	return ""
}

// site is an origin's registrable domain, "" for one with none.
func site(origin string) string {
	u, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	s, _ := publicsuffix.EffectiveTLDPlusOne(u.Hostname())
	return s
}

// refused is why nothing is filled in a frame, "" when something may be: the fill rules
// that are the carrier's facts alone (docs/password-autofill.md, recommendation 4).
func refused(s forms.Stamp) string {
	switch {
	case s.Opaque:
		return "the frame has no origin of its own"
	case !s.Secure && !strings.HasPrefix(s.Origin, "http://localhost") && !strings.HasPrefix(s.Origin, "http://127.0.0.1"):
		return "the page isn't https"
	}
	return ""
}

// matching is the test logins offered in a frame: its own origin's first, then other hosts
// of the same site, each said to be saved for where it is, then the one offered anywhere.
func (l *lab) matching(s forms.Stamp) []map[string]any {
	out := []map[string]any{}
	for pass := 0; pass < 3; pass++ {
		for i, lg := range l.logins {
			var note string
			switch {
			case pass == 0 && !lg.anywhere && lg.Origin == s.Origin:
			case pass == 1 && !lg.anywhere && lg.Origin != s.Origin && site(lg.Origin) != "" && site(lg.Origin) == site(s.Origin) && strings.HasPrefix(lg.Origin, "https://"):
				note = "saved for " + strings.TrimPrefix(lg.Origin, "https://")
			case pass == 2 && lg.anywhere:
				note = "the made-up login"
			default:
				continue
			}
			out = append(out, map[string]any{"id": i, "username": lg.Username, "note": note})
		}
	}
	return out
}

// tab is what Go made of a tab, for the popup.
func (l *lab) tab(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Tab string `json:"tab"`
	}
	json.NewDecoder(r.Body).Decode(&q)
	l.mu.Lock()
	defer l.mu.Unlock()
	frames := []map[string]any{}
	for key, f := range l.frames {
		if !strings.HasPrefix(key, q.Tab+":") {
			continue
		}
		found := []map[string]any{}
		for _, fd := range f.found {
			roles := []map[string]string{}
			for _, field := range f.fields.Fields {
				if role := fd.Roles[field.Ref]; role != "" {
					roles = append(roles, map[string]string{"role": string(role), "field": describe(f.fields, field.Ref)})
				}
			}
			one := map[string]any{"form": fd.Form, "kind": fd.Kind, "roles": roles}
			if fd.Kind == forms.SignIn || fd.Kind == forms.UsernameOnly || fd.Kind == forms.Change {
				if why := refused(f.stamp); why != "" {
					one["refused"] = why
				} else {
					one["logins"] = l.matching(f.stamp)
				}
			}
			found = append(found, one)
		}
		frames = append(frames, map[string]any{"stamp": f.stamp, "fields": len(f.fields.Fields), "found": found})
	}
	json.NewEncoder(w).Encode(map[string]any{"frames": frames, "log": l.logs[q.Tab]})
}

// fill answers the popup's pick with Go's fill for that form: the username and the current
// password, where the form has them.
func (l *lab) fill(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Tab, Frame, Document, Form string
		Login                      int
	}
	json.NewDecoder(r.Body).Decode(&q)
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.frames[q.Tab+":"+q.Frame]
	if f == nil || f.stamp.Document != q.Document || q.Login < 0 || q.Login >= len(l.logins) || refused(f.stamp) != "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "that page has gone, or nothing is filled there"})
		return
	}
	lg := l.logins[q.Login]
	for _, fd := range f.found {
		if fd.Form != q.Form {
			continue
		}
		fill := forms.Fill{Values: map[string]string{}}
		if fd.Username != "" {
			fill.Values[fd.Username] = lg.Username
		}
		if fd.Current != "" {
			fill.Values[fd.Current] = lg.Password
		}
		json.NewEncoder(w).Encode(map[string]any{"message": map[string]any{"type": "fill", "payload": fill}})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"error": "that form has gone"})
}
