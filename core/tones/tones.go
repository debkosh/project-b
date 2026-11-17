// Package tones makes every colour of our pages and shells from the one frame colour Go
// owns, light or dark, so any frame colour looks coherent in both. Light: the bar and
// surfaces are the frame mixed far toward white, icons a little toward black. Dark: the bar
// and surfaces are a near-black with a trace of the frame, icons and the accent the frame
// lifted toward white. The frame itself stays the colour the user picked, as the window
// paints it.
//
// Go sends both palettes with the frame colour (docs/native-shell.md), and each page or
// shell picks light or dark as the theme pref and its own system say, so a web shell and a
// native one paint the same tones from one place. Each palette is the design system's
// tokens by their CSS names (docs/design-system.md).
package tones

import (
	"fmt"
	"math"
	"strconv"
)

// Palette is one theme's tokens, by name: "--accent" and so on.
type Palette map[string]string

// Both is the light and the dark palette of one frame colour.
type Both struct {
	Light Palette `json:"light"`
	Dark  Palette `json:"dark"`
}

// None is no colour picked, as a profile starts (docs/onboarding.md): a light grey frame,
// or a dark grey one in dark, and ink for the accent, the frame being too pale, or too
// dark, to read as one.
const None = ""

// The frames of None, light and dark.
const (
	noneLight = "#dcdee2"
	noneDark  = "#3a3f4a"
)

// Frame is the colour the window paints for frame, light or dark: frame itself, or one of
// None's greys.
func Frame(frame string, dark bool) string {
	switch {
	case frame != None:
		return frame
	case dark:
		return noneDark
	}
	return noneLight
}

// Of is both palettes of frame, "#rrggbb" or None. A frame that is neither gives nil, as
// there is nothing to paint.
func Of(frame string) *Both {
	if _, ok := parse(frame); !ok && frame != None {
		return nil
	}
	return &Both{Light: For(frame, false), Dark: For(frame, true)}
}

// For is one palette of frame, "#rrggbb" or None: dark's, or light's.
func For(frame string, dark bool) Palette {
	if frame == None {
		return none(dark)
	}
	c, _ := parse(frame)
	toward := func(target [3]float64, t float64) string { return hex(mix(c, target, t)) }
	faint := func(a string) string {
		return fmt.Sprintf("rgba(%d, %d, %d, %s)", int(c[0]), int(c[1]), int(c[2]), a)
	}
	base := c
	if dark {
		base = mix(c, all(255), 0.2)
	}
	accent, onAccent := labelled(base)
	if dark {
		return Palette{
			"--frame":       frame,
			"--accent":      hex(accent),
			"--on-accent":   hex(onAccent),
			"--bar":         toward([3]float64{27, 28, 30}, 0.88),
			"--surface":     toward([3]float64{40, 42, 45}, 0.9),
			"--icon":        toward(all(255), 0.55),
			"--accent-soft": faint("0.34"),
			"--accent-tint": faint("0.2"),
			"--text":        "#e8eaed",
			"--muted":       "#9aa0a6",
			"--line":        "rgba(255, 255, 255, 0.12)",
			"--hover":       "rgba(255, 255, 255, 0.08)",
			"--press":       "rgba(255, 255, 255, 0.14)",
			"--danger":      "#f28b82",
			"--on-danger":   "#1f1f1f",
		}
	}
	return Palette{
		"--frame":       frame,
		"--accent":      hex(accent),
		"--on-accent":   hex(onAccent),
		"--bar":         toward(all(255), 0.87),
		"--surface":     "#ffffff",
		"--icon":        toward(all(0), 0.16),
		"--accent-soft": faint("0.18"),
		"--accent-tint": faint("0.1"),
		"--text":        "#1f1f1f",
		"--muted":       "#5c6270",
		"--line":        "rgba(15, 20, 25, 0.1)",
		"--hover":       "rgba(255, 255, 255, 0.6)",
		"--press":       "rgba(0, 0, 0, 0.08)",
		"--danger":      "#d93025",
		"--on-danger":   "#ffffff",
	}
}

// none is None's palette: its grey's, with ink for the accent and its washes in light, and
// in dark the light text's, as a grey accent would not read.
func none(dark bool) Palette {
	p := For(Frame(None, dark), dark)
	p["--frame"] = Frame(None, dark)
	if dark {
		p["--accent"], p["--on-accent"] = "#e7eaf2", "#1f1f1f"
		p["--accent-soft"], p["--accent-tint"] = "rgba(231, 234, 242, 0.16)", "rgba(231, 234, 242, 0.08)"
		return p
	}
	p["--accent"], p["--on-accent"] = "#23272f", "#ffffff"
	p["--icon"] = "#4a4f58"
	p["--accent-soft"], p["--accent-tint"] = "rgba(35, 39, 47, 0.12)", "rgba(35, 39, 47, 0.06)"
	return p
}

func parse(frame string) ([3]float64, bool) {
	var c [3]float64
	if len(frame) != 7 || frame[0] != '#' {
		return c, false
	}
	for i := range c {
		v, err := strconv.ParseUint(frame[1+2*i:3+2*i], 16, 8)
		if err != nil {
			return c, false
		}
		c[i] = float64(v)
	}
	return c, true
}

func all(v float64) [3]float64 { return [3]float64{v, v, v} }

func hex(c [3]float64) string {
	return fmt.Sprintf("#%02x%02x%02x", int(c[0]), int(c[1]), int(c[2]))
}

// mix is a colour moved a fraction t of the way to target, rounded as JavaScript's
// Math.round does, half up. The product is rounded on its own, as JavaScript rounds it: Go
// may otherwise fuse the multiply and the add (an FMA on arm64), and a value that lands on
// a half then rounds the other way.
func mix(from, target [3]float64, t float64) [3]float64 {
	var out [3]float64
	for i := range from {
		step := float64((target[i] - from[i]) * t)
		out[i] = math.Floor(from[i] + step + 0.5)
	}
	return out
}

var (
	white = all(255)
	ink   = [3]float64{31, 31, 31}
)

// labelled is the accent and what goes on it, a primary button's words or a badge's count:
// white or the ink, whichever reads better. It must reach WCAG's 4.5 for text. A few
// colours sit where neither does, a mid blue or a red lifted for dark: those move away from
// the better label, in small steps, until it reads. The rest keep the accent they had.
func labelled(base [3]float64) (accent, onAccent [3]float64) {
	onAccent = ink
	if contrast(base, white) >= contrast(base, ink) {
		onAccent = white
	}
	away := all(255)
	if onAccent == white {
		away = all(0)
	}
	accent = base
	for t := 0.02; contrast(accent, onAccent) < 4.5; t += 0.02 {
		accent = mix(base, away, t)
	}
	return accent, onAccent
}

// contrast is WCAG 2's contrast ratio of two colours, from their relative luminance.
func contrast(a, b [3]float64) float64 {
	hi, lo := luminance(a), luminance(b)
	if lo > hi {
		hi, lo = lo, hi
	}
	return (hi + 0.05) / (lo + 0.05)
}

func luminance(c [3]float64) float64 {
	var l [3]float64
	for i, v := range c {
		s := v / 255
		if s <= 0.03928 {
			l[i] = s / 12.92
		} else {
			l[i] = math.Pow((s+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*l[0] + 0.7152*l[1] + 0.0722*l[2]
}
