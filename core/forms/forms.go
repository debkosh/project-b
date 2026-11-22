// Package forms is the page protocol (docs/password-autofill.md): what a page side tells Go
// of a frame's form fields, and what Go tells it back, with Go's working out of each field's
// role from those facts. Nothing in it is about passwords alone: it carries fields, the roles
// to watch and the values to fill, and core/passwords is the first thing to use it.
//
// The page side is a form script in each frame, which reports facts and fills on command and
// decides nothing. A carrier gets its messages to Go and adds what a page can't fake, the
// Stamp: in Project B the engine, in the test bench an extension's service worker. Go trusts
// only the stamp for where a message came from.
package forms

// Stamp is what the carrier says of the frame a message came from. No field of a message
// from the page side says any of this: a page could lie.
type Stamp struct {
	// Tab is the tab, and Frame the frame in it, by the carrier's own ids.
	Tab   string `json:"tab"`
	Frame string `json:"frame"`
	// Document is the frame's document, new at each navigation: a fill goes only to the
	// document that sent the fields it fills.
	Document string `json:"document"`
	// Origin is the frame's committed origin, scheme://host[:port].
	Origin string `json:"origin"`
	// Top is whether it is the tab's top frame, and CrossOrigin whether any frame above it
	// is of another origin.
	Top         bool `json:"top"`
	CrossOrigin bool `json:"crossOrigin"`
	// Opaque is a sandboxed frame, or one with no origin of its own: about:blank, srcdoc,
	// data:. Nothing is filled there.
	Opaque bool `json:"opaque"`
	// Secure is https with a certificate that checks.
	Secure bool `json:"secure"`
}

// Field is one field of a frame, as the form script saw it. It carries no value.
type Field struct {
	// Ref is the script's own name for the field, in its frame, and Form its form's, "" for
	// a field in no form.
	Ref  string `json:"ref"`
	Form string `json:"form,omitempty"`
	// Tag is input, textarea or select, and Type an input's type as the page wrote it.
	Tag  string `json:"tag"`
	Type string `json:"type,omitempty"`
	// What the page calls it, each as written.
	Name         string `json:"name,omitempty"`
	ID           string `json:"id,omitempty"`
	Autocomplete string `json:"autocomplete,omitempty"`
	Placeholder  string `json:"placeholder,omitempty"`
	Label        string `json:"label,omitempty"`
	AriaLabel    string `json:"ariaLabel,omitempty"`
	// Nearby is the text just before it, for a field with no label.
	Nearby    string `json:"nearby,omitempty"`
	MaxLength int    `json:"maxLength,omitempty"`
	// WasPassword is a field that was type=password once and is text now, as a field with
	// its password shown is.
	WasPassword bool `json:"wasPassword,omitempty"`
	Visible     bool `json:"visible"`
	Focusable   bool `json:"focusable"`
	ReadOnly    bool `json:"readOnly,omitempty"`
	Empty       bool `json:"empty"`
}

// Form is a form element of the frame.
type Form struct {
	Ref    string `json:"ref"`
	Action string `json:"action,omitempty"`
	Method string `json:"method,omitempty"`
	// Buttons is the words on its buttons.
	Buttons []string `json:"buttons,omitempty"`
}

// Fields is the page side's "fields": every field of the frame, in the page's order, sent
// when it has some and again when they change.
type Fields struct {
	Fields []Field `json:"fields"`
	Forms  []Form  `json:"forms,omitempty"`
}

// Box is a rectangle. From the page side it is in the frame; the carrier makes it the
// window's.
type Box struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Pressed is the page side's "pressed": the person pressed or focused a field.
type Pressed struct {
	Ref string `json:"ref"`
	Box Box    `json:"box"`
}

// Sent is the page side's "submitted", a form sent, and its "gone", a form removed or
// hidden after a request: each with the values of the fields Go asked to watch, by ref.
type Sent struct {
	Form   string            `json:"form,omitempty"`
	Values map[string]string `json:"values"`
}

// Watch is Go's "watch": the fields whose values go with submitted and gone. Only fields Go
// took for a role something uses are ever watched; nothing else on a page leaves it.
type Watch struct {
	Refs []string `json:"refs"`
}

// Fill is Go's "fill": values for fields, by ref. The carrier delivers it only to the
// document that sent the fields, and the script fills only fields still visible and
// focusable, then answers with Filled.
type Fill struct {
	Values map[string]string `json:"values"`
}

// Filled is the page side's answer to a fill: the fields it filled.
type Filled struct {
	Refs []string `json:"refs"`
}
