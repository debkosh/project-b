package tones

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Every palette, light and dark, for the presets, the workspace colours and a spread of
// custom colours: testdata/tones.json, first written from the JavaScript Go took this over
// from (testdata/gen.mjs), its labels since picked by APCA.
func TestTheSameTonesAsTheJavaScript(t *testing.T) {
	b, err := os.ReadFile("testdata/tones.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]Both
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	if len(want) < 600 {
		t.Fatalf("only %d frames", len(want))
	}
	for frame, w := range want {
		got := Of(frame)
		if !reflect.DeepEqual(got.Light, w.Light) {
			t.Errorf("%s light:\n got %v\nwant %v", frame, got.Light, w.Light)
		}
		if !reflect.DeepEqual(got.Dark, w.Dark) {
			t.Errorf("%s dark:\n got %v\nwant %v", frame, got.Dark, w.Dark)
		}
	}
}

// A colour that isn't #rrggbb has nothing to paint.
func TestNoTonesForWhatIsntAColour(t *testing.T) {
	for _, frame := range []string{"#fff", "4f9e9e00", "#4f9e9g"} {
		if Of(frame) != nil {
			t.Errorf("%q has tones", frame)
		}
	}
}

// No colour picked is a grey frame for each theme, with an accent that reads on the page:
// ink in light, the light text in dark, and what goes on it the other way round.
func TestNoColourIsGreyWithAnAccentThatReads(t *testing.T) {
	b := Of(None)
	if b == nil || b.Light["--frame"] != Frame(None, false) || b.Dark["--frame"] != Frame(None, true) || Frame(None, false) == Frame(None, true) {
		t.Fatalf("None's frames: %v", b)
	}
	for _, p := range []Palette{b.Light, b.Dark} {
		a, _ := parse(p["--accent"])
		on, _ := parse(p["--on-accent"])
		bar, _ := parse(p["--bar"])
		if contrast(a, on) < 4.5 || contrast(a, bar) < 4.5 {
			t.Errorf("%s on %s, over %s, doesn't read", p["--on-accent"], p["--accent"], p["--bar"])
		}
	}
	if Frame("#5b7fd6", true) != "#5b7fd6" {
		t.Error("a colour picked isn't its own frame in dark")
	}
}

// Every preset's primary button reads, light and dark: its label is white or the ink,
// whichever APCA says contrasts more with the frame, and reaches Lc 60 on the accent.
func TestEveryPresetsLabelReadsByAPCA(t *testing.T) {
	for _, frame := range []string{None, "#0faaaa", "#5b7fd6", "#faa804", "#d82b1f", "#384562"} {
		for _, dark := range []bool{false, true} {
			p := For(frame, dark)
			a, _ := parse(p["--accent"])
			on, _ := parse(p["--on-accent"])
			if lc := lightness(on, a); lc < readable {
				t.Errorf("%q dark=%v: %s on %s is Lc %.0f", frame, dark, p["--on-accent"], p["--accent"], lc)
			}
		}
	}
	// The mid tones WCAG 2's ratio gave the ink: white reads better on them.
	for _, frame := range []string{"#0faaaa", "#5b7fd6"} {
		if on := For(frame, false)["--on-accent"]; on != "#ffffff" {
			t.Errorf("%s light: label %s, want white", frame, on)
		}
	}
}
