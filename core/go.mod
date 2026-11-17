module projectb

go 1.26.0

require github.com/sudiptadeb/amarit v0.7.1

require (
	github.com/sudiptadeb/project-b/browserdata v0.0.0
	golang.org/x/net v0.62.0
)

replace github.com/sudiptadeb/project-b/browserdata => ../browserdata
