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
