// Package fixturekind is a local, real (non-model) Kind implementation used
// only by the ormc probe integration test: it exercises the actual `go run`
// dependency probe against a package that genuinely compiles, instead of
// mocking the toolchain.
package fixturekind

import "webtyp.com/model"

type customKind struct{}

func (customKind) Storage() model.FieldType    { return model.FieldText }
func (customKind) Name() string                { return "fixture_custom" }
func (customKind) Validate(value string) error { return nil }

// Custom is a zero-arg Kind constructor, per the ecosystem convention for
// probe-resolved kinds.
func Custom() model.Kind { return customKind{} }
