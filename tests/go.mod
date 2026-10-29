// Separate test module: isolates codegen fixture deps (orm runtime for
// generated models) so the root webtyp.com/ormc module stays
// fmt + model + modfind only.
module webtyp.com/ormc/tests

go 1.26.8

require (
	webtyp.com/json v0.5.37
	webtyp.com/model v0.2.10
	webtyp.com/orm v0.12.20
	webtyp.com/ormc v0.1.29
	webtyp.com/time v0.6.2
)

require (
	webtyp.com/calendar v0.1.0 // indirect
	webtyp.com/escape v0.1.0 // indirect
	webtyp.com/fmt v1.0.0 // indirect
	webtyp.com/modfind v0.0.11 // indirect
	webtyp.com/storage v0.1.8 // indirect
)

replace webtyp.com/ormc => ..
