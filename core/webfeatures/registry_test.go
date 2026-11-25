package webfeatures

import (
	"encoding/json"
	"regexp"
	"testing"
)

// Every row of registry.json holds together, and a row that doesn't is refused.
func TestTheRegistry(t *testing.T) {
	if len(All) == 0 {
		t.Fatal("no rows")
	}
	const row = `"name":"A","api":"a()","why":"w"`
	const both = `"expect":{"mac":"resolved","linux":"resolved","android":"resolved"}`
	for _, bad := range []string{
		`[{"id":"a",` + row + `,"status":{"mac":"works"}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"fine"}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works","windows":"works"}}]`,
		`[{"id":"a","name":"A","api":"a()","status":{"mac":"works","linux":"works","android":"works"}}]`,
		`[{"id":"A",` + row + `,"status":{"mac":"works","linux":"works","android":"works"}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"owner":"x","ownr":"y"}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"}},{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"}}]`,
		// Off says what turns it off, and what is turned off is off everywhere.
		`[{"id":"a",` + row + `,"status":{"mac":"off","linux":"off","android":"off"}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"off","linux":"off","android":"works"},"disable_blink_features":["A"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"quiet","linux":"quiet","android":"quiet"},"disable_features":["A"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"off","linux":"off","android":"off"},"disable_features":["A,B"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"interface":{"name":"x.mojom.X","scope":["page"]}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"delegate":"RunFileChooser"}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"needs":["luck"]}]`,
		// A test names a probe of its own row, a basics check by number, or a check.
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"test":["probe:b"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"test":["basics:x"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"test":["drive"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"probe":[{"name":"p","call":"p()",` + both + `}]},` +
			`{"id":"b",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"probe":[{"name":"p","call":"q()",` + both + `}]}]`,
		// A probe says what its call gives on each platform, as its row's status allows, so a
		// call that starts or stops working fails its probe until the row is changed.
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"probe":[{"name":"p","call":"p()"}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"probe":[{"name":"p","call":"p()","expect":{"mac":"resolved"}}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"quiet","linux":"quiet","android":"works"},"probe":[{"name":"p","call":"p()","expect":{"mac":"absent:TypeError","linux":"absent:TypeError","android":"resolved"}}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"probe":[{"name":"p","call":"p()","expect":{"mac":"fine","linux":"fine","android":"resolved"}}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"probe":[{"name":"p","call":"p()","expect":{"mac":"resolved:","linux":"resolved:","android":"resolved"}}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"probe":[{"name":"p","call":"p()",` + both + `,"expect":{"mac":"resolved","linux":"resolved","android":"resolved","windows_tap":"resolved"}}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"quiet"},"probe":[{"name":"p","call":"p()","expect":{"mac":"resolved","linux":"resolved","android":"pending","android_tap":"absent"}}]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","linux":"works","android":"works"},"probe":[{"name":"p","call":"p()","sees_nothing":true,` + both + `}]}]`,
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	good := `[{"id":"a",` + row + `,"status":{"mac":"off","linux":"off","android":"off"},"disable_blink_features":["A"],` +
		`"probe":[{"name":"p","call":"p()","expect":{"mac":"absent:ReferenceError","linux":"absent:ReferenceError","android":"absent"}},{"name":"q","call":"q()","sees_nothing":true}],` +
		`"test":["probe:p","basics:3","check:a page keeps its renderer"]}]`
	quiet := `[{"id":"a",` + row + `,"status":{"mac":"quiet","linux":"quiet","android":"unknown"},` +
		`"probe":[{"name":"p","call":"p()","expect":{"mac":"rejected:NotAllowedError","linux":"rejected:NotAllowedError","android":"pending","android_tap":"absent"}}]}]`
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

// Every key in registry.json is one GN can name: engine/web_features.gni reads the file
// with read_file(..., "json"), which makes each key an identifier, and a key such as
// "android-tap" failed gn gen on every engine build while Go passed.
func TestTheRegistrysKeysAreGNIdentifiers(t *testing.T) {
	var file any
	if err := json.Unmarshal(registry, &file); err != nil {
		t.Fatal(err)
	}
	ident := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	var walk func(v any, at string)
	walk = func(v any, at string) {
		switch v := v.(type) {
		case map[string]any:
			for k, e := range v {
				if !ident.MatchString(k) {
					t.Errorf("%s has key %q, which GN can't read", at, k)
				}
				walk(e, at+"."+k)
			}
		case []any:
			for _, e := range v {
				walk(e, at)
			}
		}
	}
	walk(file, "registry.json")
}
