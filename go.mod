module github.com/npillmayer/docrender

go 1.27.4

require (
	github.com/carlos7ags/folio v0.11.0
	github.com/npillmayer/cords v0.1.2-alpha.3
	github.com/npillmayer/khipu v0.0.0
	github.com/npillmayer/typography v0.0.0
	github.com/npillmayer/uax v0.3.0-alpha1
	golang.org/x/image v0.47.0
	golang.org/x/text v0.43.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jolestar/go-commons-pool v2.0.0+incompatible // indirect
	github.com/npillmayer/opentype v0.0.0 // indirect
	github.com/npillmayer/schuko v0.2.0-alpha.3.0.20211209143531-2d524c4964ff // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	golang.org/x/net v0.61.0 // indirect
)

replace github.com/npillmayer/typography => ../typography

replace github.com/npillmayer/khipu => ../khipu

replace github.com/npillmayer/opentype => ../opentype

replace github.com/npillmayer/cords => ../cords

replace github.com/npillmayer/uax => ../uax

replace github.com/npillmayer/schuko => ../schuko
