package tones

import (
	"slices"
	"testing"
)

// The seven are gulal's most distinct, from hue 0, none of them near either --danger.
func TestTheSevenHues(t *testing.T) {
	if want := []int{0, 181, 90, 270, 135, 225, 315}; !slices.Equal(Hues, want) {
		t.Fatalf("hues %v, want %v", Hues, want)
	}
}

// A kept colour is #rrggbb or one of the seven, written as gulal's whole degrees.
func TestWhatAColourCanKeep(t *testing.T) {
	for _, c := range []string{"#3470d0", "0", "181", "315"} {
		if !Kept(c) {
			t.Errorf("%q not kept", c)
		}
	}
	for _, c := range []string{"", "#FFF", "#3470D0", "180", "-0", "0181", "360", "red", " 90"} {
		if Kept(c) {
			t.Errorf("%q kept", c)
		}
	}
}

// A new one takes the swatch used least, the farthest from those in use, then the first.
func TestNextIsTheSwatchFarthestFromThoseInUse(t *testing.T) {
	for _, c := range []struct {
		in   []string
		want string
	}{
		{nil, "0"},
		{[]string{"0"}, "181"},
		{[]string{"0", "181"}, "90"},
		{[]string{"181"}, "0"},
		{[]string{"90", "270"}, "0"},
		// a picked blue keeps the blues away
		{[]string{"0", "181", "90", "#3470d0"}, "135"},
		{[]string{"0", "181", "90", "270", "135", "225", "315"}, "0"},
		{[]string{"0", "181", "90", "270", "135", "225", "315", "0"}, "181"},
	} {
		if got := Next(c.in); got != c.want {
			t.Errorf("Next(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}

// A hue is filled at gulal's middle, a picked colour as it is.
func TestFill(t *testing.T) {
	if Fill("#3470d0") != "#3470d0" || Fill("nope") != "" || Fill("180") != "" {
		t.Error("fill of a picked or a bad colour")
	}
	if f := Fill("181"); !picked.MatchString(f) {
		t.Errorf("Fill(181) = %q", f)
	}
}

// Every palette draws the seven, in the same order, a different colour light and dark, and
// stronger for a vivid frame than for None.
func TestEveryPaletteDrawsTheSeven(t *testing.T) {
	none, teal := Of(None), Of("#0faaaa")
	for _, b := range []*Both{none, teal} {
		if len(b.Hues) != 7 {
			t.Fatalf("%d hues", len(b.Hues))
		}
		for i, h := range b.Hues {
			if h.Hue != Hues[i] || h.Name == "" || !picked.MatchString(h.Light) || !picked.MatchString(h.Dark) || h.Light == h.Dark {
				t.Errorf("hue %d: %+v", i, h)
			}
		}
	}
	if none.Hues[0].Light == teal.Hues[0].Light {
		t.Errorf("None and teal draw hue 0 the same: %s", none.Hues[0].Light)
	}
}

// Each of the seven has a name of its own, for a screen reader.
func TestTheSevenHaveNames(t *testing.T) {
	seen := map[string]bool{}
	for _, h := range Of(None).Hues {
		if seen[h.Name] {
			t.Errorf("two hues called %s", h.Name)
		}
		seen[h.Name] = true
	}
}
