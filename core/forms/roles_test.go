package forms

import (
	"testing"
)

// in is an input a person can use, of a type, with what the page calls it: its name, then
// "key=value" for more. A password field's type is password.
func in(ref, typ, name string, more ...string) Field {
	f := Field{Ref: ref, Form: "f", Tag: "input", Type: typ, Name: name, Visible: true, Focusable: true, Empty: true}
	for _, m := range more {
		switch k, v, _ := cut(m); k {
		case "autocomplete":
			f.Autocomplete = v
		case "label":
			f.Label = v
		case "placeholder":
			f.Placeholder = v
		case "id":
			f.ID = v
		case "form":
			f.Form = v
		case "hidden":
			f.Visible, f.Focusable = false, false
		case "readonly":
			f.ReadOnly = true
		case "was-password":
			f.WasPassword = true
		}
	}
	return f
}

func cut(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// The roles of a frame's fields and the kind of each form, from what the form script
// reports: tags first, then how the fields stand, then the page's own words for them.
func TestRoles(t *testing.T) {
	for _, c := range []struct {
		name   string
		fields []Field
		values map[string]string
		kind   Kind
		// The refs wanted for the username, the current password, the new one and its
		// confirmation, "" for none.
		username, current, fresh, confirm string
	}{
		{name: "an ordinary sign-in, no tags",
			fields: []Field{in("u", "text", "login"), in("p", "password", "pass")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a sign-in by its tags, the fields out of order",
			fields: []Field{in("p", "password", "a", "autocomplete=current-password"), in("x", "text", "b"), in("u", "email", "c", "autocomplete=username")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "only the tag's last word counts",
			fields: []Field{in("u", "text", "a", "autocomplete=section-login username"), in("p", "password", "b", "autocomplete=section-login current-password")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "the username is the nearest text field before the password",
			fields: []Field{in("first", "text", "first_name"), in("u", "email", "email"), in("p", "password", "password"), in("after", "text", "referrer")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a search box before the password is no username",
			fields: []Field{in("u", "text", "user"), in("q", "text", "q"), in("p", "password", "password")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a field a person can't use is passed over for one they can",
			fields: []Field{in("u", "text", "user"), in("h", "text", "csrf_user", "hidden"), in("p", "password", "password")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a hidden field is the username when it is the only one",
			fields: []Field{in("h", "text", "user", "hidden"), in("p", "password", "password")},
			kind:   SignIn, username: "h", current: "p"},
		{name: "a password shown as text is still the password",
			fields: []Field{in("u", "text", "user"), in("p", "text", "password", "was-password")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a sign-up by its tags",
			fields: []Field{in("u", "email", "email", "autocomplete=username"), in("n", "password", "a", "autocomplete=new-password"), in("c", "password", "b", "autocomplete=new-password")},
			kind:   SignUp, username: "u", fresh: "n", confirm: "c"},
		{name: "two password fields, the second called a confirmation, are a sign-up",
			fields: []Field{in("u", "email", "email"), in("n", "password", "password"), in("c", "password", "password_confirmation")},
			kind:   SignUp, username: "u", fresh: "n", confirm: "c"},
		{name: "two password fields with nothing said are a change",
			fields: []Field{in("a", "password", "p1"), in("b", "password", "p2")},
			kind:   Change, current: "a", fresh: "b"},
		{name: "two that were sent holding the same are new and its confirmation",
			fields: []Field{in("u", "text", "user"), in("a", "password", "p1"), in("b", "password", "p2")},
			values: map[string]string{"a": "hunter2!", "b": "hunter2!"},
			kind:   SignUp, username: "u", fresh: "a", confirm: "b"},
		{name: "two that were sent holding different things are current and new",
			fields: []Field{in("a", "password", "password"), in("b", "password", "password_confirmation")},
			values: map[string]string{"a": "old one", "b": "new one"},
			kind:   Change, current: "a", fresh: "b"},
		{name: "two tagged new that were sent holding different things: the tags are wrong",
			fields: []Field{in("a", "password", "a", "autocomplete=new-password"), in("b", "password", "b", "autocomplete=new-password")},
			values: map[string]string{"a": "old one", "b": "new one"},
			kind:   Change, current: "a", fresh: "b"},
		{name: "three password fields are current, new and its confirmation",
			fields: []Field{in("a", "password", "old"), in("b", "password", "new"), in("c", "password", "again")},
			kind:   Change, current: "a", fresh: "b", confirm: "c"},
		{name: "a tagged current password and one more: the other is new",
			fields: []Field{in("a", "password", "x", "autocomplete=current-password"), in("b", "password", "y")},
			kind:   Change, current: "a", fresh: "b"},
		{name: "one password field the page calls new, with no username, is a reset",
			fields: []Field{in("n", "password", "new_password")},
			kind:   Reset, fresh: "n"},
		{name: "a code field among the passwords is left alone",
			fields: []Field{in("u", "text", "user"), in("p", "password", "password"), in("o", "password", "otp_code")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a one-time code by its tag is no password",
			fields: []Field{in("u", "text", "user"), in("p", "password", "password"), in("o", "password", "x", "autocomplete=one-time-code")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a card's security code is no password",
			fields: []Field{in("n", "text", "cardnumber", "autocomplete=cc-number"), in("v", "password", "cvv", "autocomplete=cc-csc")},
			kind:   ""},
		{name: "a password field that can't be typed in is passed over",
			fields: []Field{in("u", "text", "user"), in("r", "password", "token", "readonly"), in("p", "password", "password")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "the first step of a sign-in in two: a username alone",
			fields: []Field{in("u", "email", "identifier")},
			kind:   UsernameOnly, username: "u"},
		{name: "a username alone by its tag, whatever it is called",
			fields: []Field{in("u", "text", "f1", "autocomplete=username")},
			kind:   UsernameOnly, username: "u"},
		{name: "a username first, with a password field kept out of sight beside it",
			fields: []Field{in("u", "email", "identifier", "autocomplete=username"), in("p", "password", "hiddenPassword", "hidden")},
			kind:   UsernameOnly, username: "u"},
		{name: "the same with no tag, the page calling the field an email",
			fields: []Field{in("u", "email", "email"), in("p", "password", "autofill-password", "hidden")},
			kind:   UsernameOnly, username: "u"},
		{name: "a whole sign-in out of sight, as in a dialog not yet opened, is still one",
			fields: []Field{in("u", "text", "user", "hidden"), in("p", "password", "password", "hidden")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a search form is nothing",
			fields: []Field{in("q", "text", "q", "placeholder=Search")},
			kind:   ""},
		{name: "a newsletter's email field beside another field is nothing",
			fields: []Field{in("n", "text", "name"), in("e", "email", "email")},
			kind:   ""},
		{name: "a phone number is a username",
			fields: []Field{in("u", "tel", "phone"), in("p", "password", "password")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a PIN beside a card number is no password",
			fields: []Field{in("n", "text", "account"), in("p", "password", "pin")},
			kind:   ""},
	} {
		got := Roles(Fields{Fields: c.fields}, c.values)
		if c.kind == "" {
			if len(got) != 0 {
				t.Errorf("%s: found %+v, want nothing", c.name, got)
			}
			continue
		}
		if len(got) != 1 {
			t.Errorf("%s: found %d forms: %+v", c.name, len(got), got)
			continue
		}
		g := got[0]
		if g.Kind != c.kind || g.Username != c.username || g.Current != c.current || g.New != c.fresh || g.Confirm != c.confirm {
			t.Errorf("%s: %s with username %q, current %q, new %q, confirm %q; want %s with %q, %q, %q, %q",
				c.name, g.Kind, g.Username, g.Current, g.New, g.Confirm, c.kind, c.username, c.current, c.fresh, c.confirm)
		}
	}
}

// Each form of a frame is worked out apart, and the fields in no form together.
func TestRolesByForm(t *testing.T) {
	page := Fields{Fields: []Field{
		in("q", "text", "q", "form=search"),
		in("u", "text", "user", "form=login"), in("p", "password", "pass", "form=login"),
		in("e", "email", "email", "form="), in("n", "password", "new_password", "form="), in("c", "password", "confirm", "form="),
	}}
	got := Roles(page, nil)
	if len(got) != 2 || got[0].Form != "login" || got[0].Kind != SignIn || got[1].Form != "" || got[1].Kind != SignUp || got[1].Username != "e" {
		t.Fatalf("the forms: %+v", got)
	}
}
