package forms

import (
	"regexp"
	"strings"
)

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

// Found is one form of a frame, or the fields of a frame that are in no form taken
// together, with the roles Go gave its fields. Username, Current, New and Confirm are
// refs, "" for none.
type Found struct {
	Form string
	Kind Kind
	// Roles is every field of the group Go gave a role, by ref.
	Roles                           map[string]Role
	Username, Current, New, Confirm string
}

// The words that mark a field, in its name, id, label or placeholder, when no tag says. From
// Chromium's own patterns for a field that is no password, and the managers' keyword lists.
var (
	notPassword  = regexp.MustCompile(`(?i)\botp\b|one.?time|sms.?code|verif\w*.?code|security.?code|auth\w*.?code|\b2fa\b|\bmfa\b|totp|\bpin\b|\bcvc\b|\bcvv\b|\bcsc\b|card.?verif|\bssn\b|social.?sec`)
	notUsername  = regexp.MustCompile(`(?i)search|\bquery\b|\bq\b|captcha|coupon|promo|zip|postal`)
	confirmWords = regexp.MustCompile(`(?i)confirm|repeat|again|retype|re.?enter|re.?type|verify|second`)
	newWords     = regexp.MustCompile(`(?i)\bnew\b|new.?pass|create|choose`)
	currentWords = regexp.MustCompile(`(?i)current|\bold\b|old.?pass|existing|previous`)
	usernameWord = regexp.MustCompile(`(?i)user|e.?mail|login|log.?in|account|identifier|\bid\b|phone|mobile|member`)
)

// token is an autocomplete attribute's last word, which is the one that says what a field
// is: "section-a shipping new-password" is new-password.
func token(autocomplete string) string {
	words := strings.Fields(strings.ToLower(autocomplete))
	if len(words) == 0 {
		return ""
	}
	return words[len(words)-1]
}

// called is everything the page calls a field, for the word lists: its words apart, so
// "otp_code" and "new-password" are read as the words they are made of.
func called(f Field) string {
	return apart.Replace(strings.Join([]string{f.Name, f.ID, f.Label, f.AriaLabel, f.Placeholder}, " "))
}

var apart = strings.NewReplacer("_", " ", "-", " ", ".", " ", "[", " ", "]", " ")

func isPassword(f Field) bool {
	return f.Tag == "input" && (strings.EqualFold(f.Type, "password") || f.WasPassword)
}

// isText is a field a username could be typed in.
func isText(f Field) bool {
	if f.Tag != "input" || isPassword(f) {
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

// Roles works out the roles of a frame's fields, a form at a time, and the fields in no
// form together. values is what watched fields held, by ref, when a form was sent, and nil
// while a page is only being read: two password fields that held the same are a new
// password and its confirmation. A group with no username and no password field isn't
// returned.
//
// Tags first (autocomplete), then how the fields stand: the rules Chromium's own password
// manager uses, written down in docs/password-autofill/chromium.md, with the managers' word
// lists where two password fields hold nothing to compare.
func Roles(page Fields, values map[string]string) []Found {
	var order []string
	groups := map[string][]Field{}
	for _, f := range page.Fields {
		if _, seen := groups[f.Form]; !seen {
			order = append(order, f.Form)
		}
		groups[f.Form] = append(groups[f.Form], f)
	}
	var out []Found
	for _, form := range order {
		if found, ok := rolesOf(form, groups[form], values); ok {
			out = append(out, found)
		}
	}
	return out
}

func rolesOf(form string, fields []Field, values map[string]string) (Found, bool) {
	found := Found{Form: form, Roles: map[string]Role{}}
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

	// What the tags say. A tag on the wrong kind of field says nothing, and so does one used
	// more often than it can be: one username, one current password, a new one and its
	// confirmation.
	tagged := map[string][]Field{}
	for _, f := range fields {
		switch t := token(f.Autocomplete); {
		case t == "one-time-code":
			found.Roles[f.Ref] = OneTimeCode
		case t == "username" && !isPassword(f):
			tagged[t] = append(tagged[t], f)
		case (t == "current-password" || t == "new-password") && isPassword(f):
			tagged[t] = append(tagged[t], f)
		}
	}
	if len(tagged["username"]) == 1 {
		set(tagged["username"][0].Ref, Username)
	}
	if len(tagged["current-password"]) == 1 {
		set(tagged["current-password"][0].Ref, CurrentPassword)
	}
	if fresh := tagged["new-password"]; len(fresh) == 1 || len(fresh) == 2 {
		// Two that were sent holding different things aren't a password and its confirmation,
		// whatever the tags say.
		if len(fresh) == 2 && values != nil && values[fresh[0].Ref] != values[fresh[1].Ref] {
			fresh = nil
		}
		for i, f := range fresh {
			set(f.Ref, []Role{NewPassword, ConfirmPassword}[i])
		}
	}

	// The password fields no tag spoke for: those a person could use, when any is, and none
	// that is for a code, a card's number or the like, or that can't be typed in.
	var passwords []Field
	for _, f := range fields {
		if !isPassword(f) || found.Roles[f.Ref] != "" || f.ReadOnly || isCardField(f) || notPassword.MatchString(called(f)) {
			continue
		}
		passwords = append(passwords, f)
	}
	passwords = best(passwords)
	// A page that asks for the username first often keeps a password field out of sight
	// beside it. With a text field a person can use and no password field they can, the
	// hidden ones are no part of this step: nothing is ever filled where it can't be seen.
	if len(passwords) > 0 && !reachable(passwords[0]) {
		for _, f := range fields {
			if isText(f) && reachable(f) {
				passwords = nil
				break
			}
		}
	}
	if len(passwords) > 3 {
		passwords = passwords[:3]
	}
	held := func(i int) string { return values[passwords[i].Ref] }
	same := func(i, j int) bool { return values != nil && held(i) != "" && held(i) == held(j) }
	says := func(i int, words *regexp.Regexp) bool { return words.MatchString(called(passwords[i])) }
	switch n := len(passwords); {
	case n == 1 && found.Current == "" && found.New == "":
		// One alone is the current password, unless the page calls it a new one.
		if says(0, newWords) && !says(0, currentWords) {
			set(passwords[0].Ref, NewPassword)
		} else {
			set(passwords[0].Ref, CurrentPassword)
		}
	case n == 1 && found.Current != "" && found.New == "":
		set(passwords[0].Ref, NewPassword)
	case n == 1 && found.New != "" && found.Confirm == "":
		set(passwords[0].Ref, ConfirmPassword)
	case n == 2 && found.Current == "" && found.New == "":
		// Two that held the same are a new password and its confirmation. With nothing to
		// compare, the page's words say: a second one called a confirmation makes them that;
		// otherwise the first is the current one and the second new, as on a form that
		// changes a password.
		if same(0, 1) || values == nil && says(1, confirmWords) && !says(0, currentWords) {
			set(passwords[0].Ref, NewPassword)
			set(passwords[1].Ref, ConfirmPassword)
		} else {
			set(passwords[0].Ref, CurrentPassword)
			set(passwords[1].Ref, NewPassword)
		}
	case n == 2 && found.Current != "" && found.New == "":
		set(passwords[0].Ref, NewPassword)
		set(passwords[1].Ref, ConfirmPassword)
	case n == 3 && found.Current == "" && found.New == "":
		switch {
		case same(0, 1) && same(1, 2):
			set(passwords[0].Ref, CurrentPassword)
		case same(0, 1):
			set(passwords[0].Ref, NewPassword)
			set(passwords[1].Ref, ConfirmPassword)
		default:
			// The current one, then the new one and its confirmation.
			set(passwords[0].Ref, CurrentPassword)
			set(passwords[1].Ref, NewPassword)
			set(passwords[2].Ref, ConfirmPassword)
		}
	}

	// The username, when no tag named it: the nearest text field before the first password
	// field, one a person could use before one they couldn't.
	first := -1
	for i, f := range fields {
		if r := found.Roles[f.Ref]; r == CurrentPassword || r == NewPassword {
			first = i
			break
		}
	}
	if found.Username == "" && first >= 0 {
		var before []Field
		for i := first - 1; i >= 0; i-- {
			f := fields[i]
			if isText(f) && found.Roles[f.Ref] == "" && !isCardField(f) && !notPassword.MatchString(called(f)) && !notUsername.MatchString(called(f)) && !strings.EqualFold(f.Type, "number") {
				before = append(before, f)
			}
		}
		for _, f := range before {
			if reachable(f) {
				set(f.Ref, Username)
				break
			}
		}
		if found.Username == "" && len(before) > 0 {
			set(before[0].Ref, Username)
		}
	}
	// A form with no password field: the one text field a person can use, when the page
	// calls it a username, is the first step of a sign-in in two.
	if found.Username == "" && first < 0 && found.Current == "" && found.New == "" {
		var texts []Field
		secret := false
		for _, f := range fields {
			if isText(f) && reachable(f) && found.Roles[f.Ref] == "" {
				texts = append(texts, f)
			}
			// A field for a code or a PIN beside it, one a person can use, makes this no first
			// step of a sign-in.
			secret = secret || isPassword(f) && reachable(f)
		}
		if !secret && len(texts) == 1 && usernameWord.MatchString(called(texts[0])) && !notUsername.MatchString(called(texts[0])) && !notPassword.MatchString(called(texts[0])) {
			set(texts[0].Ref, Username)
		}
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

// isCardField is a field a tag gives to a payment card.
func isCardField(f Field) bool { return strings.HasPrefix(token(f.Autocomplete), "cc-") }

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
