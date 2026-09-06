// Package fixtureuse feeds the ormc probe integration test
// (../probe_integration_test.go): a Definition using a real, local, non-model
// Kind so the generation-time dependency probe actually shells out to
// `go run` instead of a canned test double. Not named model.go/models.go on
// purpose — it must not be picked up by the tests module's own
// ormc.Run() scan.
package fixtureuse

import (
	"webtyp.com/model"
	"webtyp.com/ormc/tests/fixturekind"
)

var FixtureModel = model.Definition{
	Name: "fixture",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		{Name: "note", Type: fixturekind.Custom()},
	},
}
