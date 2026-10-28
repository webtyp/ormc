package ormc

import (
	"errors"
	"testing"

	"webtyp.com/model"
)

// exportDDLCalled is a test-only Exporter that records whether ExportDDL was
// invoked, so the reproduction test can assert it's never called when there
// are no models to export.
type exportDDLCalled struct {
	called bool
}

func (e *exportDDLCalled) ExportDDL(models []model.Model) (string, error) {
	e.called = true
	return "", nil
}

// TestExportSQL_NoModels_ReturnsErrNoModelsFound reproduces the reported bug:
// webtyp -tui against a project with no model.go/models.go files (e.g. a
// view-only project) still writes an empty config/schema.sql, because ExportSQL
// silently returned ("", nil) — a "successful" empty result — for zero
// models, instead of a distinguishable error. The button's Handler
// (ddlc/tui) only skips writing the output file when err != nil, so a nil
// error here means the file gets written even with nothing to export.
func TestExportSQL_NoModels_ReturnsErrNoModelsFound(t *testing.T) {
	g := New()
	tmpDir := t.TempDir() // empty: no model.go/models.go anywhere

	exporter := &exportDDLCalled{}
	sql, err := g.ExportSQL(tmpDir, exporter)

	if !errors.Is(err, ErrNoModelsFound) {
		t.Fatalf("ExportSQL with no models: err = %v, want ErrNoModelsFound", err)
	}
	if sql != "" {
		t.Errorf("expected empty sql, got %q", sql)
	}
	if exporter.called {
		t.Error("ExportDDL should not be called when there are no models")
	}
}

func TestModels_SameOrderEveryTimeAndKeepsForeignKeys(t *testing.T) {
	first, err := New().Models("tests")
	if err != nil {
		t.Fatal(err)
	}
	second, _ := New().Models("tests")
	if len(first) == 0 || len(first) != len(second) {
		t.Fatalf("len = %d and %d, want equal and > 0", len(first), len(second))
	}
	for i := range first {
		if first[i].ModelName() != second[i].ModelName() {
			t.Fatalf("order differs at %d: %s vs %s", i, first[i].ModelName(), second[i].ModelName())
		}
	}
	for _, m := range first {
		if m.ModelName() != "order" {
			continue
		}
		ext := m.(interface{ SchemaExt() []model.FieldExt }).SchemaExt()
		if len(ext) != 1 || ext[0].Ref != "user" {
			t.Fatalf("order SchemaExt = %+v, want one FK to user", ext)
		}
		return
	}
	t.Fatal("model order not found in tests/")
}

func TestModels_NoModels_ReturnsErrNoModelsFound(t *testing.T) {
	if _, err := New().Models(t.TempDir()); !errors.Is(err, ErrNoModelsFound) {
		t.Fatalf("err = %v, want ErrNoModelsFound", err)
	}
}
