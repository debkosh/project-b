package mockengine

import (
	"fmt"

	"projectb/engine"
)

// FormSent is a message the app sent to a frame's form script.
type FormSent struct {
	To            engine.FormFrame
	Kind, Payload string
}

// SetFormScript keeps the form script the app gave. The mock runs no page, so nothing
// runs it: a test plays the page side with FormMessage.
func (e *Engine) SetFormScript(source string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.formScript = source
}

// FormScript is the form script the app gave.
func (e *Engine) FormScript() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.formScript
}

// FormMessage plays a frame's form script telling the app something, with the frame as an
// engine would stamp it. The frame's document is the one a later SendForm reaches.
func (e *Engine) FormMessage(from engine.FormFrame, kind, payload string) {
	e.mu.Lock()
	if e.formDocuments == nil {
		e.formDocuments = map[string]string{}
	}
	e.formDocuments[formKey(from)] = from.Document
	e.mu.Unlock()
	e.listener.FormMessage(from, kind, payload)
}

// FormLeft plays a frame going on to another page: its document is no more.
func (e *Engine) FormLeft(from engine.FormFrame) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.formDocuments, formKey(from))
}

// SendForm records a message for a frame's form script, when its document is still the
// one that last spoke from that frame.
func (e *Engine) SendForm(to engine.FormFrame, kind, payload string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if to.Document == "" || e.formDocuments[formKey(to)] != to.Document {
		return false
	}
	e.formSent = append(e.formSent, FormSent{To: to, Kind: kind, Payload: payload})
	return true
}

// FormsSent are the messages the app sent to form scripts, in order.
func (e *Engine) FormsSent() []FormSent {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]FormSent(nil), e.formSent...)
}

func formKey(f engine.FormFrame) string {
	return fmt.Sprintf("%d\x00%s", f.Tab, f.Frame)
}
