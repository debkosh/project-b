package tones

import (
	"math"
	"regexp"
	"slices"
	"strconv"

	"github.com/sudiptadeb/gulal"
)

// The colours of what the user makes and names, a workspace or a tab group
// (docs/design-system.md, "Made up, or picked"). The picker offers gulal's seven most
// distinct hues as swatches, and a + for any colour. A swatch picked is kept as its hue, in
// whole degrees ("181"), and drawn for each theme, so it is the same hue in light and dark
// and as strong as the frame. A colour picked with + is kept as "#rrggbb" and drawn as it is.
//
// The seven start from hue 0, not the frame's hue as gulal's own start would, so they are
// the same seven whatever the frame: a kept hue is always one of the swatches.

// Hue is one of the seven, as Go sends it with the tones: the hue, its name for a screen
// reader, and its colour as a dot or a line on the bar, light and dark.
type Hue struct {
	Hue   int    `json:"hue"`
	Name  string `json:"name"`
	Light string `json:"light"`
	Dark  string `json:"dark"`
}

// names name a hue by where it sits on OKLCH's wheel: each up to its bound.
var names = []struct {
	to   int
	name string
}{{20, "Rose"}, {70, "Orange"}, {115, "Olive"}, {160, "Green"}, {205, "Teal"}, {250, "Blue"}, {290, "Indigo"}, {345, "Purple"}, {360, "Rose"}}

func nameOf(h int) string {
	for _, n := range names {
		if h < n.to {
			return n.name
		}
	}
	return ""
}

// danger is each scheme's --danger, which every colour gulal makes keeps away from, so no
// group or workspace reads as an error.
var danger = map[bool]string{false: "#d93025", true: "#f28b82"}

// Hues are the seven, most distinct first: gulal's distinct(7) from hue 0, away from both
// schemes' --danger.
var Hues = func() []int {
	g := gulal.New(gulal.Options{Avoid: []string{danger[false], danger[true]}})
	var out []int
	for _, c := range g.Distinct(7, gulal.DistinctOptions{From: "zero"}).Colours {
		_, _, h := c.OKLCH()
		out = append(out, int(math.Round(h))%360)
	}
	return out
}()

var picked = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Kept says whether c is a colour a workspace or group can keep: "#rrggbb", or one of Hues.
func Kept(c string) bool {
	return picked.MatchString(c) || slices.Contains(Hues, hueOf(c))
}

// hueOf is the hue a kept colour names, or -1 for one that names none.
func hueOf(c string) int {
	h, err := strconv.Atoi(c)
	if err != nil || h < 0 || h >= 360 || strconv.Itoa(h) != c {
		return -1
	}
	return h
}

// Next is the colour a new workspace or group starts on, among those in use: the swatch
// used least, and of those the one whose hue is farthest from theirs, then the first.
func Next(existing []string) string {
	var hues []float64
	uses := map[int]int{}
	for _, c := range existing {
		if h := hueOf(c); h >= 0 {
			uses[h]++
			hues = append(hues, float64(h))
		} else if a := plain.Adapt(c); a != nil {
			_, _, h := a.OKLCH()
			hues = append(hues, h)
		}
	}
	best, bestUses, bestGap := 0, math.MaxInt, -1.0
	for _, h := range Hues {
		gap := 360.0
		for _, x := range hues {
			d := math.Abs(float64(h) - x)
			gap = math.Min(gap, math.Min(d, 360-d))
		}
		if uses[h] < bestUses || uses[h] == bestUses && gap > bestGap {
			best, bestUses, bestGap = h, uses[h], gap
		}
	}
	return strconv.Itoa(best)
}

// plain is gulal with no accent, which reads a colour's hue as it is.
var plain = gulal.New(gulal.Options{})

// Fill is a kept colour as one solid colour, as a frame is: a picked one as it is, a hue at
// gulal's middle lightness and strength. It is "" for what isn't kept.
func Fill(c string) string {
	switch {
	case picked.MatchString(c):
		return c
	case Kept(c):
		return plain.Hue(float64(hueOf(c))).Hex()
	}
	return ""
}

// hues draws the seven for frame's palettes: each as a mark on the bar, as strong as the
// frame, or as None's grey when no colour is picked.
func hues(light, dark Palette, frame string) []Hue {
	on := func(p Palette, dark bool) *gulal.Env {
		o := gulal.Options{Scheme: "light", Background: p["--bar"], Avoid: []string{danger[dark]}}
		if dark {
			o.Scheme = "dark"
		}
		if frame != None {
			o.Accent = frame
		}
		return gulal.New(o)
	}
	l, d := on(light, false), on(dark, true)
	out := make([]Hue, len(Hues))
	for i, h := range Hues {
		out[i] = Hue{Hue: h, Name: nameOf(h), Light: l.Hue(float64(h)).Mark(), Dark: d.Hue(float64(h)).Mark()}
	}
	return out
}
