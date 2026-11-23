package forms

import _ "embed"

// matching is DuckDuckGo's field matching with our glue, built by tools/form-matching.sh
// (page/matching.gen.js; its source and licence in page/matching), and script the form
// script, which asks it what each field is (page/forms.js).
var (
	//go:embed page/matching.gen.js
	matching string
	//go:embed page/forms.js
	script string
)

// Script is the page side of the protocol: a carrier runs it in each frame, in a world of
// its own, with a transport for its messages. The matching comes first, as the form script
// calls it.
var Script = matching + "\n" + script
