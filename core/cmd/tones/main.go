// Prints the light and dark palettes Go makes from a frame colour (core/tones), as JSON, so
// a check outside Go compares a screen with the colours the app paints rather than copies of
// them, which a colour change leaves behind (tools/vm/check.py).
//
//	go run ./cmd/tones              # a new profile's frame, tones.Brand
//	go run ./cmd/tones -frame ''    # no colour picked
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"strconv"

	"projectb/tones"
	"projectb/workspaces"
)

func main() {
	frame := flag.String("frame", tones.Brand, `"#rrggbb", or "" for no colour picked`)
	flag.Parse()
	both := tones.Of(*frame)
	if both == nil {
		log.Fatalf("not a frame colour: %q", *frame)
	}
	// Main's dot as the shell paints it in light: a hue as its mark for this frame, a picked
	// colour as it is.
	main := workspaces.MainColor
	for _, h := range both.Hues {
		if strconv.Itoa(h.Hue) == main {
			main = h.Light
		}
	}
	out := map[string]any{"frame": tones.Frame(*frame, false), "light": both.Light, "dark": both.Dark, "main": main}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		log.Fatal(err)
	}
}
