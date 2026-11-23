// formroles gives core/forms' roles for pages the form script read, a JSON line each in
// and out: {"file", "told": <the fields message>} to {"file", "roles": {ref: role},
// "kinds": {ref: the kind of its form}}. tools/form-matching.sh scores the field matching
// with it on DuckDuckGo's labelled forms (docs/password-autofill.md, "Detection").
package main

import (
	"bufio"
	"encoding/json"
	"os"

	"projectb/forms"
)

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 64<<20)
	out := json.NewEncoder(os.Stdout)
	for in.Scan() {
		var line struct {
			File string       `json:"file"`
			Told forms.Fields `json:"told"`
		}
		if json.Unmarshal(in.Bytes(), &line) != nil {
			continue
		}
		roles, kinds := map[string]string{}, map[string]string{}
		for _, found := range forms.Roles(line.Told, nil) {
			for ref, role := range found.Roles {
				roles[ref], kinds[ref] = string(role), string(found.Kind)
			}
		}
		out.Encode(map[string]any{"file": line.File, "roles": roles, "kinds": kinds})
	}
}
