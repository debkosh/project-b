package forms

import _ "embed"

// Script is the form script, the page side of the protocol (page/forms.js): a carrier runs
// it in each frame, in a world of its own, with a transport for its messages.
//
//go:embed page/forms.js
var Script string
