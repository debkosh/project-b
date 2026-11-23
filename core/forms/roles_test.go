package forms

import (
	"testing"
)

// in is an input a person can use, of a type, with what the matching took it for, "" for
// nothing, then "key" or "key=value" for more.
func in(ref, typ, guess string, more ...string) Field {
	f := Field{Ref: ref, Form: "f", Tag: "input", Type: typ, Guess: guess, Visible: true, Focusable: true, Empty: true}
	for _, m := range more {
		switch k, v, _ := cut(m); k {
		case "form":
			f.Form = v
		case "group":
			f.Group = v
		case "hidden":
			f.Visible, f.Focusable = false, false
		case "readonly":
			f.ReadOnly = true
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

// The roles of a form's fields and what the form is for, from what the matching took each
// field for and how the fields stand together.
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
		{name: "an ordinary sign-in",
			fields: []Field{in("u", "text", "username"), in("p", "password", "password")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a sign-in whose username comes after its password",
			fields: []Field{in("p", "password", "password"), in("x", "text", ""), in("u", "email", "username")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a password beside a field the matching names nothing for: no username",
			fields: []Field{in("x", "text", ""), in("p", "password", "password")},
			kind:   SignIn, current: "p"},
		{name: "an email address beside a password is the username",
			fields: []Field{in("e", "email", "email"), in("p", "password", "password")},
			kind:   SignIn, username: "e", current: "p"},
		{name: "a field taken for a username goes before one taken for an email address",
			fields: []Field{in("u", "text", "username"), in("e", "email", "email"), in("p", "password", "password")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "of two usernames, the nearest before the password",
			fields: []Field{in("a", "text", "username"), in("b", "text", "username"), in("p", "password", "password")},
			kind:   SignIn, username: "b", current: "p"},
		{name: "of two usernames, one a person can use before one they can't",
			fields: []Field{in("a", "text", "username"), in("b", "text", "username", "hidden"), in("p", "password", "password")},
			kind:   SignIn, username: "a", current: "p"},
		{name: "a sign-up: an email address, a new password and its confirmation",
			fields: []Field{in("e", "email", "email"), in("n", "password", "password.new"), in("c", "password", "password.new")},
			kind:   SignUp, username: "e", fresh: "n", confirm: "c"},
		{name: "two called new that were sent holding different things are a current one and a new one",
			fields: []Field{in("u", "text", "username"), in("a", "password", "password.new"), in("b", "password", "password.new")},
			values: map[string]string{"a": "one", "b": "two"},
			kind:   Change, username: "u", current: "a", fresh: "b"},
		{name: "a change of password: the current one, the new one and its confirmation",
			fields: []Field{in("o", "password", "password.current"), in("n", "password", "password.new"), in("c", "password", "password.new")},
			kind:   Change, current: "o", fresh: "n", confirm: "c"},
		{name: "a current one named, and one beside it that isn't: the new one",
			fields: []Field{in("o", "password", "password.current"), in("n", "password", "password")},
			kind:   Change, current: "o", fresh: "n"},
		{name: "a new one named, and one beside it that isn't: its confirmation",
			fields: []Field{in("e", "email", "email"), in("n", "password", "password.new"), in("c", "password", "password")},
			kind:   SignUp, username: "e", fresh: "n", confirm: "c"},
		{name: "a reset: a new password and its confirmation, and no username",
			fields: []Field{in("n", "password", "password.new"), in("c", "password", "password.new")},
			kind:   Reset, fresh: "n", confirm: "c"},
		{name: "two passwords nothing is said of, sent holding the same: a new one and its confirmation",
			fields: []Field{in("u", "text", "username"), in("a", "password", "password"), in("b", "password", "password")},
			values: map[string]string{"a": "same", "b": "same"},
			kind:   SignUp, username: "u", fresh: "a", confirm: "b"},
		{name: "two passwords nothing is said of, on a page only read: the current one and a new one",
			fields: []Field{in("a", "password", "password"), in("b", "password", "password")},
			kind:   Change, current: "a", fresh: "b"},
		{name: "three passwords nothing is said of: the current one, the new one and its confirmation",
			fields: []Field{in("a", "password", "password"), in("b", "password", "password"), in("c", "password", "password")},
			kind:   Change, current: "a", fresh: "b", confirm: "c"},
		{name: "a code's field beside a sign-in is known and left alone",
			fields: []Field{in("u", "text", "username"), in("p", "password", "password"), in("o", "text", "totp")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a code's field alone is no form of ours",
			fields: []Field{in("o", "text", "totp")}},
		{name: "a username asked for first, its password kept out of sight: the first step of two",
			fields: []Field{in("u", "email", "username"), in("p", "password", "password", "hidden")},
			kind:   UsernameOnly, username: "u"},
		{name: "a whole sign-in out of sight, as a page that shows its form later has",
			fields: []Field{in("u", "text", "username", "hidden"), in("p", "password", "password", "hidden")},
			kind:   SignIn, username: "u", current: "p"},
		{name: "a password field that can't be typed in has no role, and with it in sight its username is no first step either",
			fields: []Field{in("u", "text", "username"), in("p", "password", "password", "readonly")}},
		{name: "a username alone: the first step of a sign-in in two",
			fields: []Field{in("u", "text", "username")},
			kind:   UsernameOnly, username: "u"},
		{name: "two usernames and no password say nothing",
			fields: []Field{in("a", "text", "username"), in("b", "text", "username")}},
		{name: "an email address alone, as a newsletter asks for, is no form of ours",
			fields: []Field{in("e", "email", "email")}},
		{name: "a field of type password that the matching names nothing for, as a card's code, has no role",
			fields: []Field{in("n", "text", ""), in("c", "password", "")}},
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

// A code's field in a sign-in has its role, though nothing is done with it.
func TestACodesFieldIsKnown(t *testing.T) {
	got := Roles(Fields{Fields: []Field{in("u", "text", "username"), in("p", "password", "password"), in("o", "text", "totp")}}, nil)
	if len(got) != 1 || got[0].Roles["o"] != OneTimeCode {
		t.Fatalf("the code's field: %+v", got)
	}
}

// The fields are taken together as the matching grouped them, which may be closer than
// their form element, as on a page with one form around everything; with no group told,
// by their form, and the fields in none together. A form's ref is the form element's.
func TestRolesByGroup(t *testing.T) {
	page := Fields{Fields: []Field{
		in("q", "text", "", "form=page", "group=G1"),
		in("u", "text", "username", "form=page", "group=G2"), in("p", "password", "password", "form=page", "group=G2"),
		in("e", "email", "email", "form=page", "group=G3"),
		in("s", "email", "email", "form=", "group=G4"), in("n", "password", "password.new", "form=", "group=G4"), in("c", "password", "password.new", "form=", "group=G4"),
	}}
	got := Roles(page, nil)
	if len(got) != 2 || got[0].Form != "page" || got[0].Kind != SignIn || got[0].Username != "u" || got[1].Form != "" || got[1].Kind != SignUp || got[1].Username != "s" {
		t.Fatalf("the groups: %+v", got)
	}
	plain := Fields{Fields: []Field{
		in("u", "text", "username", "form=login"), in("p", "password", "password", "form=login"),
		in("e", "email", "email", "form="), in("n", "password", "password.new", "form="),
	}}
	got = Roles(plain, nil)
	if len(got) != 2 || got[0].Form != "login" || got[0].Kind != SignIn || got[1].Form != "" || got[1].Kind != SignUp || got[1].Username != "e" {
		t.Fatalf("the forms, with no group told: %+v", got)
	}
}
