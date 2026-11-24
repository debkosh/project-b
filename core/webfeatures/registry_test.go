package webfeatures

import "testing"

// Every row of registry.json holds together, and a row that doesn't is refused.
func TestTheRegistry(t *testing.T) {
	if len(All) == 0 {
		t.Fatal("no rows")
	}
	const row = `"name":"A","api":"a()","why":"w"`
	const both = `"expect":{"mac":"resolved","android":"resolved"}`
	for _, bad := range []string{
		`[{"id":"a",` + row + `,"status":{"mac":"works"}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"fine"}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works","linux":"works"}}]`,
		`[{"id":"a","name":"A","api":"a()","status":{"mac":"works","android":"works"}}]`,
		`[{"id":"A",` + row + `,"status":{"mac":"works","android":"works"}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"owner":"x","ownr":"y"}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"}},{"id":"a",` + row + `,"status":{"mac":"works","android":"works"}}]`,
		// Off says what turns it off, and what is turned off is off everywhere.
		`[{"id":"a",` + row + `,"status":{"mac":"off","android":"off"}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"off","android":"works"},"disable_blink_features":["A"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"quiet","android":"quiet"},"disable_features":["A"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"off","android":"off"},"disable_features":["A,B"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"interface":{"name":"x.mojom.X","scope":["page"]}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"delegate":"RunFileChooser"}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"needs":["luck"]}]`,
		// A test names a probe of its own row, a basics check by number, or a check.
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"test":["probe:b"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"test":["basics:x"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"test":["drive"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"probe":[{"name":"p","call":"p()",` + both + `}]},` +
			`{"id":"b",` + row + `,"status":{"mac":"works","android":"works"},"probe":[{"name":"p","call":"q()",` + both + `}]}]`,
		// A probe says what its call gives on each platform, as its row's status allows, so a
		// call that starts or stops working fails its probe until the row is changed.
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"probe":[{"name":"p","call":"p()"}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"probe":[{"name":"p","call":"p()","expect":{"mac":"resolved"}}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"quiet","android":"works"},"probe":[{"name":"p","call":"p()","expect":{"mac":"absent:TypeError","android":"resolved"}}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"probe":[{"name":"p","call":"p()","expect":{"mac":"fine","android":"resolved"}}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"probe":[{"name":"p","call":"p()","expect":{"mac":"resolved:","android":"resolved"}}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"probe":[{"name":"p","call":"p()",` + both + `,"expect":{"mac":"resolved","android":"resolved","linux-tap":"resolved"}}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"quiet"},"probe":[{"name":"p","call":"p()","expect":{"mac":"resolved","android":"pending","android-tap":"absent"}}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"probe":[{"name":"p","call":"p()","sees_nothing":true,` + both + `}]}]`,
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	good := `[{"id":"a",` + row + `,"status":{"mac":"off","android":"off"},"disable_blink_features":["A"],` +
		`"probe":[{"name":"p","call":"p()","expect":{"mac":"absent:ReferenceError","android":"absent"}},{"name":"q","call":"q()","sees_nothing":true}],` +
		`"test":["probe:p","basics:3","check:a page keeps its renderer"]}]`
	quiet := `[{"id":"a",` + row + `,"status":{"mac":"quiet","android":"unknown"},` +
		`"probe":[{"name":"p","call":"p()","expect":{"mac":"rejected:NotAllowedError","android":"pending","android-tap":"absent"}}]}]`
	for _, ok := range []string{good, quiet} {
		if _, err := Parse([]byte(ok)); err != nil {
			t.Errorf("refused %s: %v", ok, err)
		}
	}
}

// The settings page gets every row, with what a page gets of it on this platform.
func TestTheView(t *testing.T) {
	v := View()
	if len(v) != len(All) {
		t.Fatalf("%d rows for %d features", len(v), len(All))
	}
	for i, r := range v {
		f := All[i]
		if r.ID != f.ID || r.Name != f.Name || r.API != f.API || r.Why != f.Why || r.Status != f.Status[Platform()] {
			t.Errorf("%s: %+v", f.ID, r)
		}
	}
}
