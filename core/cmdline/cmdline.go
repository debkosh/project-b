// Package cmdline reads a command line as Chromium's base::CommandLine does on the Mac and
// Android, so the Go core sees the same switches content will: "--name=value" or "--name",
// "-" as good as "--", the last of a name winning, and everything after a lone "--" an
// argument. Arguments are what isn't a switch: the addresses a copy is started with.
package cmdline

import "strings"

// Line is a command line: the program, then its words as given.
type Line struct {
	words    []string
	switches map[string]string
	args     []string
}

// Parse reads argv, the program first.
func Parse(argv []string) *Line {
	l := &Line{words: argv, switches: map[string]string{}}
	if len(argv) == 0 {
		return l
	}
	rest := argv[1:]
	for i, w := range rest {
		if w == "--" {
			l.args = append(l.args, rest[i+1:]...)
			break
		}
		name, value, ok := switchOf(w)
		if !ok {
			l.args = append(l.args, w)
			continue
		}
		l.switches[name] = value
	}
	return l
}

// switchOf splits a word that is a switch into its name and value.
func switchOf(w string) (name, value string, ok bool) {
	var body string
	switch {
	case strings.HasPrefix(w, "--"):
		body = w[2:]
	case strings.HasPrefix(w, "-"):
		body = w[1:]
	default:
		return "", "", false
	}
	if body == "" {
		return "", "", false
	}
	name, value, _ = strings.Cut(body, "=")
	return name, value, true
}

// Has says whether the switch was given.
func (l *Line) Has(name string) bool {
	_, ok := l.switches[name]
	return ok
}

// Value is the switch's value, "" when it has none or wasn't given.
func (l *Line) Value(name string) string { return l.switches[name] }

// Args are the words that aren't switches, in order.
func (l *Line) Args() []string { return l.args }

// With is the command line with each of switches, "--name" or "--name=value", that names
// one not given on it, put just after the program: one given on the command line wins, and a
// switch after a lone "--" would be an argument.
func (l *Line) With(switches ...string) []string {
	out := []string{}
	if len(l.words) > 0 {
		out = append(out, l.words[0])
	}
	seen := map[string]bool{}
	for _, s := range switches {
		name, _, ok := switchOf(s)
		if !ok || l.Has(name) || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, s)
	}
	if len(l.words) > 1 {
		out = append(out, l.words[1:]...)
	}
	return out
}
