module projectb

go 1.26.0

require github.com/sudiptadeb/amarit v0.11.1

require (
	github.com/sudiptadeb/project-b/browserdata v0.0.0
	golang.org/x/net v0.62.0
)

require github.com/sudiptadeb/gulal v0.3.0

require github.com/sudiptadeb/oskit v0.0.0

require (
	github.com/godbus/dbus/v5 v5.2.4
	github.com/mattn/go-sqlite3 v1.14.53
	github.com/ncruces/go-sqlite3 v0.37.0
	github.com/sudiptadeb/amarit/flatpak v0.11.2
	github.com/sudiptadeb/synckosh v0.2.0
	golang.org/x/crypto v0.59.0
	rsc.io/qr v0.2.0
)

require (
	github.com/ncruces/go-sqlite3-wasm/v6 v6.5.35401 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	go.etcd.io/bbolt v1.5.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
)

replace github.com/sudiptadeb/project-b/browserdata => ../browserdata

replace github.com/sudiptadeb/oskit => ../oskit
