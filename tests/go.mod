// Separate test module: isolates codegen fixture deps (orm runtime for
// generated models) so the root webtyp.com/ormc module stays
// fmt + model + modfind only.
module webtyp.com/ormc/tests

go 1.25.2

require (
	webtyp.com/model v0.1.7
	webtyp.com/orm v0.12.0
	webtyp.com/ormc v0.1.13
)

require (
	webtyp.com/fmt v0.25.7 // indirect
	webtyp.com/modfind v0.0.4 // indirect
	webtyp.com/storage v0.0.6 // indirect
)

replace webtyp.com/ormc => ..
