package forms

import "strings"

// Role is what a field is for.
type Role string

const (
	Username        Role = "username"
	CurrentPassword Role = "current-password"
	NewPassword     Role = "new-password"
	ConfirmPassword Role = "confirm-password"
	// OneTimeCode is a field for a code sent or made elsewhere: known, and left alone.
	OneTimeCode Role = "one-time-code"
)

// Kind is what a form is for, which follows from the roles found in it.
type Kind string

const (
	SignIn Kind = "sign-in"
	SignUp Kind = "sign-up"
	// Change has a current password and a new one; Reset a new one alone.
	Change Kind = "change"
	Reset  Kind = "reset"
	// UsernameOnly is the first step of a sign-in in two steps.
	UsernameOnly Kind = "username-only"
)

// Found is one form of a frame, as the matching grouped its fields, with the roles Go gave
// them. Form is the form element's ref, "" for fields in none. Username, Current, New and
// Confirm are refs, "" for none.
type Found struct {
	Form string
	Kind Kind
	// Roles is every field of the group Go gave a role, by ref.
	Roles                           map[string]Role
	Username, Current, New, Confirm string
}

// The guesses the form script reports (Field.Guess): what DuckDuckGo's matching took a
// field for.
const (
	guessUsername = "username"
	guessPassword = "password"
	guessCurrent  = "password.current"
	guessNew      = "password.new"
	guessCode     = "totp"
	guessEmail    = "email"
)

func isPassword(f Field) bool { return strings.HasPrefix(f.Guess, guessPassword) }

// isText is a field a username could be typed in.
func isText(f Field) bool {
	if f.Tag != "input" || isPassword(f) || strings.EqualFold(f.Type, "password") {
		return false
	}
	switch strings.ToLower(f.Type) {
	case "", "text", "email", "tel", "number", "url":
		return true
	}
	return false
}

// reachable is a field a person could use: seen, or able to take the focus.
func reachable(f Field) bool { return f.Visible || f.Focusable }

// Roles works out the roles of a frame's fields, a form at a time, as the matching grouped
// them. values is what watched fields held, by ref, when a form was sent, and nil while a
// page is only being read: two password fields that held the same are a new password and
// its confirmation. A group with no username and no password field isn't returned.
//
// What a field is comes from DuckDuckGo's matching, which reads the page itself
// (docs/password-autofill.md, "Detection"). What Go adds is what follows from how the
// fields stand together: which of two or three password fields is the current one, the new
// one and its confirmation, which username goes with them, what the form is for, and that
// nothing a person can't see is ever given a role beside something they can.
func Roles(page Fields, values map[string]string) []Found {
	var order []string
	groups := map[string][]Field{}
	for _, f := range page.Fields {
		key := f.Group
		if key == "" {
			key = "form " + f.Form
		}
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], f)
	}
	var out []Found
	for _, key := range order {
		if found, ok := rolesOf(groups[key], values); ok {
			out = append(out, found)
		}
	}
	return out
}

func rolesOf(fields []Field, values map[string]string) (Found, bool) {
	found := Found{Form: fields[0].Form, Roles: map[string]Role{}}
	set := func(ref string, role Role) {
		found.Roles[ref] = role
		switch role {
		case Username:
			found.Username = ref
		case CurrentPassword:
			found.Current = ref
		case NewPassword:
			found.New = ref
		case ConfirmPassword:
			found.Confirm = ref
		}
	}

	// The password fields, those a person could use when any is. A page that asks for the
	// username first often keeps a password field out of sight beside it: with a text field
	// a person can use and no password field they can, the hidden ones are no part of this
	// step, as nothing is ever filled where it can't be seen.
	var passwords []Field
	for _, f := range fields {
		if f.Guess == guessCode {
			found.Roles[f.Ref] = OneTimeCode
		}
		if isPassword(f) && !f.ReadOnly {
			passwords = append(passwords, f)
		}
	}
	passwords = best(passwords)
	if len(passwords) > 0 && !reachable(passwords[0]) {
		for _, f := range fields {
			if isText(f) && reachable(f) {
				passwords = nil
				break
			}
		}
	}

	// What the matching says of each: one current password, and a new one with or without
	// its confirmation. Said more often than that, it says nothing. Two new ones that were
	// sent holding different things aren't a password and its confirmation either.
	said := map[string][]Field{}
	for _, f := range passwords {
		said[f.Guess] = append(said[f.Guess], f)
	}
	if len(said[guessCurrent]) == 1 {
		set(said[guessCurrent][0].Ref, CurrentPassword)
	}
	if fresh := said[guessNew]; len(fresh) == 1 || len(fresh) == 2 {
		if len(fresh) == 2 && values != nil && values[fresh[0].Ref] != values[fresh[1].Ref] {
			fresh = nil
		}
		for i, f := range fresh {
			set(f.Ref, []Role{NewPassword, ConfirmPassword}[i])
		}
	}

	// The rest, by how they stand, up to three: Chromium's rules
	// (docs/password-autofill/chromium.md).
	var rest []Field
	for _, f := range passwords {
		if found.Roles[f.Ref] == "" {
			rest = append(rest, f)
		}
	}
	if len(rest) > 3 {
		rest = rest[:3]
	}
	same := func(i, j int) bool {
		return values != nil && values[rest[i].Ref] != "" && values[rest[i].Ref] == values[rest[j].Ref]
	}
	switch n := len(rest); {
	case n == 1 && found.Current == "" && found.New == "":
		set(rest[0].Ref, CurrentPassword)
	case n == 1 && found.Current != "" && found.New == "":
		set(rest[0].Ref, NewPassword)
	case n == 1 && found.New != "" && found.Confirm == "":
		set(rest[0].Ref, ConfirmPassword)
	case n == 2 && found.Current == "" && found.New == "":
		// Two that held the same are a new password and its confirmation; otherwise the
		// first is the current one and the second new, as on a form that changes a password.
		if same(0, 1) {
			set(rest[0].Ref, NewPassword)
			set(rest[1].Ref, ConfirmPassword)
		} else {
			set(rest[0].Ref, CurrentPassword)
			set(rest[1].Ref, NewPassword)
		}
	case n == 2 && found.Current != "" && found.New == "":
		set(rest[0].Ref, NewPassword)
		set(rest[1].Ref, ConfirmPassword)
	case n == 3 && found.Current == "" && found.New == "":
		switch {
		case same(0, 1) && same(1, 2):
			set(rest[0].Ref, CurrentPassword)
		case same(0, 1):
			set(rest[0].Ref, NewPassword)
			set(rest[1].Ref, ConfirmPassword)
		default:
			// The current one, then the new one and its confirmation.
			set(rest[0].Ref, CurrentPassword)
			set(rest[1].Ref, NewPassword)
			set(rest[2].Ref, ConfirmPassword)
		}
	}

	// The username: a field the matching took for one, or failing that for an email
	// address, which beside a password is what the account is called. The nearest before
	// the first password field, one a person could use before one they couldn't; with no
	// password field, the one a person can use is the first step of a sign-in in two.
	first := -1
	for i, f := range fields {
		if r := found.Roles[f.Ref]; r == CurrentPassword || r == NewPassword {
			first = i
			break
		}
	}
	named := func(guess string) []int {
		var at []int
		for i, f := range fields {
			if f.Guess == guess && found.Roles[f.Ref] == "" && !f.ReadOnly {
				at = append(at, i)
			}
		}
		return at
	}
	candidates := named(guessUsername)
	if len(candidates) == 0 && first >= 0 {
		candidates = named(guessEmail)
	}
	pick := -1
	if first >= 0 {
		// Before the password, nearest first, then after it.
		var order []int
		for i := len(candidates) - 1; i >= 0; i-- {
			if candidates[i] < first {
				order = append(order, candidates[i])
			}
		}
		for _, i := range candidates {
			if i > first {
				order = append(order, i)
			}
		}
		for _, i := range order {
			if reachable(fields[i]) {
				pick = i
				break
			}
		}
		if pick < 0 && len(order) > 0 {
			pick = order[0]
		}
	} else if found.Current == "" && found.New == "" {
		// No password field at all, seen or not, that a person can use beside it.
		secret := false
		for _, f := range fields {
			secret = secret || (isPassword(f) || strings.EqualFold(f.Type, "password")) && reachable(f)
		}
		var can []int
		for _, i := range candidates {
			if reachable(fields[i]) {
				can = append(can, i)
			}
		}
		if !secret && len(can) == 1 {
			pick = can[0]
		}
	}
	if pick >= 0 {
		set(fields[pick].Ref, Username)
	}

	switch {
	case found.Current != "" && found.New != "":
		found.Kind = Change
	case found.Current != "":
		found.Kind = SignIn
	case found.New != "" && found.Username != "":
		found.Kind = SignUp
	case found.New != "":
		found.Kind = Reset
	case found.Username != "":
		found.Kind = UsernameOnly
	default:
		return Found{}, false
	}
	return found, true
}

// best is the fields a person could use, when any is; all of them when none is, as a page
// may show its form only later.
func best(fields []Field) []Field {
	var can []Field
	for _, f := range fields {
		if reachable(f) {
			can = append(can, f)
		}
	}
	if len(can) > 0 {
		return can
	}
	return fields
}
