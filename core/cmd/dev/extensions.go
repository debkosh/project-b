//go:build extensions

package main

import (
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"strconv"

	"projectb/engine"
	"projectb/mockengine"
)

// The mock engine's fake extension runtime, and what a test does with it over HTTP:
//
//	POST /mock/extensions/call?extension=&name=&tab=   the body is the JSON list of
//	                                                   arguments; answers {result, error}
//	POST /mock/extensions/fire                         kind, extension, value: see
//	                                                   mockengine.FireExtension
//	GET  /mock/extensions                              the events fired and the open popup
//	POST /mock/extensions/hold                         on: 1 holds installs from the store,
//	                                                   0 lets them go on
//
// -store names the folder of packages the mock engine installs from by id, as the real one
// fetches them from the store: .tools/extensions, as tools/pack-test-extension.py fills it.
func init() {
	store := flag.String("store", "", "the folder of the store's packages, crx/<id>_<version>.crx, that an install by id takes")
	routes = append(routes, func(mux *http.ServeMux, fake *mockengine.Engine) {
		fake.SetExtensionStore(*store)
		mux.HandleFunc("POST /mock/extensions/call", func(w http.ResponseWriter, r *http.Request) {
			args, _ := io.ReadAll(r.Body)
			q := r.URL.Query()
			tab, _ := strconv.ParseInt(q.Get("tab"), 10, 64)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(fake.Call(q.Get("extension"), q.Get("name"), string(args), engine.TabID(tab)))
		})
		mux.HandleFunc("POST /mock/extensions/fire", func(w http.ResponseWriter, r *http.Request) {
			fake.FireExtension(r.FormValue("kind"), r.FormValue("extension"), r.FormValue("value"))
		})
		// on=1 keeps installs from the store from ending, as a store that doesn't answer;
		// on=0 lets those held go on.
		mux.HandleFunc("POST /mock/extensions/hold", func(w http.ResponseWriter, r *http.Request) {
			on := r.FormValue("on") == "1"
			fake.Post(func() { fake.HoldStoreInstalls(on) })
		})
		mux.HandleFunc("GET /mock/extensions", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(fake.ExtensionState())
		})
	})
}
