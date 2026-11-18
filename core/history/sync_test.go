package history

import (
	"encoding/json"
	"testing"
)

// Two versions of a page join to the larger count, the later visit and the title of the
// version visited later, whichever is the row.
func TestJoiningPages(t *testing.T) {
	join := Synced()[0].Join
	raw := func(p page) json.RawMessage { b, _ := json.Marshal(p); return b }
	for _, c := range []struct{ a, b, want page }{
		{page{"u", "Old", 7, 100}, page{"u", "New", 3, 200}, page{"u", "New", 7, 200}},
		{page{"u", "", 1, 300}, page{"u", "Titled", 1, 200}, page{"u", "Titled", 1, 300}},
		{page{"u", "Alpha", 2, 100}, page{"u", "Beta", 2, 100}, page{"u", "Beta", 2, 100}},
	} {
		for _, pair := range [][2]page{{c.a, c.b}, {c.b, c.a}} {
			var got page
			json.Unmarshal(join(raw(pair[0]), raw(pair[1])), &got)
			if got != c.want {
				t.Errorf("%+v with %+v: %+v, want %+v", pair[0], pair[1], got, c.want)
			}
		}
	}
}
