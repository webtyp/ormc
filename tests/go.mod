// Separate test module: isolates codegen fixture deps (orm runtime for
// generated models) so the root webtyp.com/ormc module stays
// fmt + model + modfind only.
module webtyp.com/ormc/tests

go 1.25.2

require (
	webtyp.com/model v0.1.9
	webtyp.com/orm v0.12.3
	webtyp.com/ormc v0.1.15
)

require (
	webtyp.com/fmt v1.0.0 // indirect
	webtyp.com/modfind v0.0.9 // indirect
	webtyp.com/storage v0.0.9 // indirect
)

replace webtyp.com/ormc => ..
