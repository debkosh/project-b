package sparks

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// The limits on a spark.
const (
	MaxSource      = 140 * 1024
	MaxName        = 50
	MaxDescription = 250
)

// When a spark runs: as the page starts, before its own scripts, or once its document is
// read, the default.
const (
	DocumentStart = "document-start"
	DocumentEnd   = "document-end"
)

// Meta is what a spark's ==UserScript== header says.
type Meta struct {
	Name        string
	Description string
	// Match and ExcludeMatch are match patterns: the spark runs on an address any Match
	// takes and no ExcludeMatch does.
	Match        []string
	ExcludeMatch []string
	RunAt        string
	// NoFrames keeps it to a tab's page, out of the frames inside it.
	NoFrames bool
}

// Parse reads the header of a spark's source. Its errors are sentences to show the person.
func Parse(source string) (Meta, error) {
	if len(source) > MaxSource {
		return Meta{}, errors.New("The script is larger than 140 KB.")
	}
	lines, ok := header(source)
	if !ok {
		return Meta{}, errors.New("The script has no ==UserScript== header.")
	}
	m := Meta{RunAt: DocumentEnd}
	named := false
	for _, line := range lines {
		key, value := field(line)
		switch key {
		case "@name":
			if !named {
				m.Name, named = value, true
			}
		case "@description":
			m.Description = value
		case "@match", "@exclude-match":
			if _, err := compile(value); err != nil {
				return Meta{}, fmt.Errorf("The %s pattern %q is not a valid match pattern.", key, value)
			}
			if key == "@match" {
				m.Match = append(m.Match, value)
			} else {
				m.ExcludeMatch = append(m.ExcludeMatch, value)
			}
		case "@run-at":
			if value != DocumentStart && value != DocumentEnd {
				return Meta{}, fmt.Errorf("The @run-at value %q is not document-start or document-end.", value)
			}
			m.RunAt = value
		case "@noframes":
			m.NoFrames = true
		}
	}
	switch {
	case m.Name == "":
		return Meta{}, errors.New("The script's header has no @name.")
	case utf8.RuneCountInString(m.Name) > MaxName:
		return Meta{}, fmt.Errorf("The name is longer than %d characters.", MaxName)
	case utf8.RuneCountInString(m.Description) > MaxDescription:
		return Meta{}, fmt.Errorf("The description is longer than %d characters.", MaxDescription)
	case len(m.Match) == 0:
		return Meta{}, errors.New("The script's header has no @match.")
	}
	return m, nil
}

// header is the lines between the source's first // ==UserScript== and the
// // ==/UserScript== after it, and whether it has both.
func header(source string) ([]string, bool) {
	var lines []string
	in := false
	for _, line := range strings.Split(source, "\n") {
		text, ok := comment(line)
		switch {
		case !in && ok && text == "==UserScript==":
			in = true
		case in && ok && text == "==/UserScript==":
			return lines, true
		case in:
			lines = append(lines, line)
		}
	}
	return nil, false
}

// comment is the text of a // line, trimmed, and whether the line is one.
func comment(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "//") {
		return "", false
	}
	return strings.TrimSpace(line[2:]), true
}

// field is a header line's @key and its value, or "" for a line that has none.
func field(line string) (key, value string) {
	text, ok := comment(line)
	if !ok || !strings.HasPrefix(text, "@") {
		return "", ""
	}
	i := strings.IndexAny(text, " \t")
	if i < 0 {
		return text, ""
	}
	return text[:i], strings.TrimSpace(text[i+1:])
}
