module projectb

go 1.26.0

require github.com/debkosh/amarit v0.12.0

require (
	github.com/debkosh/project-b/browserdata v0.0.0
	golang.org/x/net v0.62.0
)

require github.com/debkosh/gulal v0.3.1

require github.com/debkosh/project-b/oskit v0.0.0

require (
	github.com/Eyevinn/dash-mpd v0.19.0
	github.com/Eyevinn/hls-m3u8 v0.6.5
	github.com/Eyevinn/mp4ff v0.64.0
	github.com/bluenviron/mediacommon/v2 v2.10.0
	github.com/debkosh/amarit/flatpak v0.12.0
	github.com/debkosh/synckosh v0.3.1
	github.com/godbus/dbus/v5 v5.2.4
	github.com/mattn/go-sqlite3 v1.14.53
	github.com/ncruces/go-sqlite3 v0.37.0
	golang.org/x/crypto v0.59.0
	rsc.io/qr v0.2.0
)

require (
	github.com/asticode/go-astikit v0.30.0 // indirect
	github.com/asticode/go-astits v1.17.0 // indirect
	github.com/barkimedes/go-deepcopy v0.0.0-20220514131651-17c30cfc62df // indirect
	github.com/ncruces/go-sqlite3-wasm/v6 v6.5.35401 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	go.etcd.io/bbolt v1.5.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
)

replace github.com/debkosh/project-b/browserdata => ../browserdata

replace github.com/debkosh/project-b/oskit => ../oskit
