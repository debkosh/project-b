module projectb

go 1.26.0

require github.com/sudiptadeb/amarit v0.10.3

require (
	github.com/sudiptadeb/project-b/browserdata v0.0.0
	golang.org/x/net v0.62.0
)

require github.com/sudiptadeb/gulal v0.3.0

require (
	github.com/mattn/go-sqlite3 v1.14.53
	github.com/sudiptadeb/synckosh v0.2.0
	golang.org/x/crypto v0.59.0
	rsc.io/qr v0.2.0
)

require (
	go.etcd.io/bbolt v1.5.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
)

replace github.com/sudiptadeb/project-b/browserdata => ../browserdata
