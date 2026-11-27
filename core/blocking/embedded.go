package blocking

import _ "embed"

// catalogJSON is the catalogue of lists, kept by hand in the repo (docs/blocking.md, The
// catalogue): only a commit changes it.
//
//go:embed catalog.json
var catalogJSON []byte

// Embedded is the catalogue the app carries.
func Embedded() (*Catalogue, error) { return ParseCatalogue(catalogJSON) }
