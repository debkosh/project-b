package history

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"projectb/store"
)

// The search, in tiers (docs/store.md): the typed addresses whose bare address starts with
// the text, most typed first; then history, scored as it always was: a bare address that
// starts with the text 1000, an address that holds it 100, a title that holds it 50, each
// plus twice its visits up to 50, then the latest visit, then the address. Each tier is one
// query with a LIMIT, through an index, so none sorts every match.

// scored is a row of a history tier with its score.
type scored struct {
	Entry
	score int
}

// order is the score order, which history_rank keeps within a tier.
const order = ` ORDER BY h.rank DESC, h.last_visit DESC, h.url LIMIT `

// columns are what a tier reads of a row: with its rank, how often it was typed.
const columns = `SELECT h.url, h.title, h.visits, h.last_visit, h.rank, coalesce((SELECT t.count FROM typed t WHERE t.url = h.url), 0) FROM `

// The tiers' conditions on a history row h. ?1 is the text, ?2 the text and a byte after any
// UTF-8, so a bare address that starts with the text is one in [?1, ?2); ?3 is the least
// rank a title match needs, and every tier's query has it, so each has the same arguments.
const (
	starts = `h.bare >= ?1 AND h.bare < ?2`
	holds  = `?3 >= 0 AND instr(h.low_url, ?1) > 0 AND NOT (` + starts + `)`
	titled = `h.rank >= ?3 AND instr(h.low_title, ?1) > 0 AND instr(h.low_url, ?1) = 0`
	// worded is a title with a word that starts with the text, for text under three letters,
	// which matches only at a start: an address that starts with it is the starts tier's.
	worded = `h.rank >= ?3 AND (h.low_title >= ?1 AND h.low_title < ?2 OR instr(h.low_title, ' ' || ?1) > 0) AND NOT (` + starts + `)`
)

// short is how many letters text must have to match anywhere: shorter, it matches only
// where an address starts or a word of a title does (docs/daily-driver.md).
const short = 3

// head is how many rows of the score order a tier looks at first, when the trigram index
// matches too many rows to read them all.
const head = 256

func search(tx *store.Tx, q string, limit int) ([]Entry, error) {
	s := &searching{tx: tx, q: q, hi: q + "\xff", spaced: strings.ContainsAny(q, " \t\r\n")}
	// An address has its spaces escaped, so text with a space is in no address.
	var out []Entry
	shown := map[string]bool{}
	if !s.spaced {
		typed, err := s.rows(0, `SELECT h.url, h.title, h.visits, h.last_visit, 0, t.count FROM typed t CROSS JOIN history h ON h.url = t.url
			WHERE h.bare >= ?1 AND h.bare < ?2 ORDER BY t.count DESC, t.last DESC, t.url LIMIT ?3`, q, s.hi, limit)
		if err != nil {
			return nil, err
		}
		for _, e := range typed {
			out = append(out, e.Entry)
			shown[e.URL] = true
		}
	}
	n := limit - len(out)
	if n <= 0 {
		return out, nil
	}
	var top int64
	if err := tx.QueryRow(`SELECT coalesce(max(page), 0) FROM history`).Scan(&top); err != nil {
		return nil, err
	}
	// Few enough matches to read and sort, against enough that a walk down the score order
	// meets the ones wanted soon: the two cost the same at about this many.
	s.few = max(200, int(math.Sqrt(float64(8*top))))
	if !s.spaced {
		// The typed addresses are among them: a few more, as they are left out.
		first, err := s.starting(n + len(out))
		if err != nil {
			return nil, err
		}
		for _, e := range first {
			if len(out) < limit && !shown[e.URL] {
				out = append(out, e.Entry)
			}
		}
		if n = limit - len(out); n <= 0 {
			return out, nil
		}
	}
	// An address match scores 102 to 200 and a title match 52 to 150: both are fetched and
	// merged. Once n address matches are in, a title match needs a rank that reaches the last.
	var rest []scored
	if utf8.RuneCountInString(q) < short {
		// Under three letters a title matches only by a word's start, and an address only
		// where it starts, which the starts tier has found.
		titles, err := s.tier(worded, "low_title", 50, n, 0)
		for _, e := range titles {
			out = append(out, e.Entry)
		}
		return out, err
	}
	if !s.spaced {
		var err error
		if rest, err = s.tier(holds, "low_url", 100, n, 0); err != nil {
			return nil, err
		}
	}
	least := 0
	if len(rest) == n {
		least = (rest[n-1].score - 50 + 1) / 2
	}
	if least <= 50 {
		titles, err := s.tier(titled, "low_title", 50, n, least)
		if err != nil {
			return nil, err
		}
		rest = append(rest, titles...)
	}
	slices.SortFunc(rest, func(a, b scored) int {
		switch {
		case a.score != b.score:
			return b.score - a.score
		case a.LastVisit != b.LastVisit:
			if a.LastVisit > b.LastVisit {
				return -1
			}
			return 1
		}
		return strings.Compare(a.URL, b.URL)
	})
	for _, e := range rest[:min(n, len(rest))] {
		out = append(out, e.Entry)
	}
	return out, nil
}

// searching is one search's text and what it found out on the way.
type searching struct {
	tx     *store.Tx
	q, hi  string
	spaced bool
	few    int
	// matched is the rows the trigram index matches in an address or a title, as a JSON
	// array, once looked for: "" before, "many" when there are more than few.
	matched string
}

// starting is the best n rows whose bare address starts with the text: few, read from the
// index of bare and sorted; many, a walk down the score order.
func (s *searching) starting(n int) ([]scored, error) {
	count := 0
	if err := s.tx.QueryRow(`SELECT count(*) FROM (SELECT 1 FROM history h INDEXED BY history_bare WHERE `+starts+` LIMIT ?3)`, s.q, s.hi, s.few+1).Scan(&count); err != nil {
		return nil, err
	}
	index := "history_rank"
	if count <= s.few {
		index = "history_bare"
	}
	return s.rows(1000, columns+`history h INDEXED BY `+index+` WHERE `+starts+order+strconv.Itoa(n), s.q, s.hi)
}

// tier is the best n rows that meet cond, whose column holds the text. Three letters or more
// go through the trigram index when it matches few rows; otherwise a look at the head of the
// score order, then the index's matches in the column, then a walk down the score order.
func (s *searching) tier(cond, column string, base, n, least int) ([]scored, error) {
	walk := func() ([]scored, error) {
		return s.rows(base, columns+`history h INDEXED BY history_rank WHERE `+cond+order+strconv.Itoa(n), s.q, s.hi, least)
	}
	if utf8.RuneCountInString(s.q) < short {
		return walk()
	}
	if s.matched == "" {
		if err := s.match(); err != nil {
			return nil, err
		}
	}
	if s.matched != "many" {
		return s.rows(base, columns+`json_each(?4) j JOIN history h ON h.page = j.value WHERE `+cond+order+strconv.Itoa(n), s.q, s.hi, least, s.matched)
	}
	if s.spaced {
		// The matches are the titles' own, and many: a walk meets them soon.
		return walk()
	}
	got, err := s.rows(base, columns+`(SELECT * FROM history h INDEXED BY history_rank`+order+strconv.Itoa(head)+`) h WHERE `+cond+order+strconv.Itoa(n), s.q, s.hi, least)
	if err != nil || len(got) == n {
		return got, err
	}
	in := column + " : " + phrase(s.q)
	count := 0
	if err := s.tx.QueryRow(`SELECT count(*) FROM (SELECT 1 FROM history_tri WHERE history_tri MATCH ?1 LIMIT ?2)`, in, s.few+1).Scan(&count); err != nil {
		return nil, err
	}
	if count > s.few {
		return walk()
	}
	return s.rows(base, columns+`history_tri JOIN history h ON h.page = history_tri.rowid WHERE history_tri MATCH ?4 AND `+cond+order+strconv.Itoa(n), s.q, s.hi, least, in)
}

// match reads the rows the trigram index matches, in an address or a title, or in a title
// alone for text with a space, up to few of them.
func (s *searching) match() error {
	in := phrase(s.q)
	if s.spaced {
		in = "low_title : " + in
	}
	r, err := s.tx.Query(`SELECT rowid FROM history_tri WHERE history_tri MATCH ?1 LIMIT ?2`, in, s.few+1)
	if err != nil {
		return err
	}
	defer r.Close()
	b := []byte{'['}
	k := 0
	for r.Next() {
		var id int64
		if err := r.Scan(&id); err != nil {
			return err
		}
		if k > 0 {
			b = append(b, ',')
		}
		b = strconv.AppendInt(b, id, 10)
		k++
	}
	if err := r.Err(); err != nil {
		return err
	}
	if k > s.few {
		s.matched = "many"
	} else {
		s.matched = string(append(b, ']'))
	}
	return nil
}

// rows reads a tier's rows, each scored base and twice its rank.
func (s *searching) rows(base int, q string, args ...any) ([]scored, error) {
	r, err := s.tx.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var out []scored
	for r.Next() {
		var e scored
		var rank int
		if err := r.Scan(&e.URL, &e.Title, &e.Visits, &e.LastVisit, &rank, &e.Typed); err != nil {
			return nil, err
		}
		e.score = base + 2*rank
		out = append(out, e)
	}
	return out, r.Err()
}

// phrase is text as an FTS5 string, which the trigram index matches anywhere.
func phrase(text string) string { return `"` + strings.ReplaceAll(text, `"`, `""`) + `"` }
