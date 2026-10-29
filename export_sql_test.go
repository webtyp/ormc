package ormc

import (
	"errors"
	"os"
	"path/filepath"
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

// A model read from source keeps what a diagram draws: the table a select's Options
// comes from, and an instant's unit — not just its storage type.
func TestModels_KeepsOptionsAndInstantKinds(t *testing.T) {
	dir := t.TempDir()
	src := `package devices

import "webtyp.com/model"

var ZoneModel = model.Definition{
	Name: "zone",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
	},
}

var DeviceModel = model.Definition{
	Name: "device",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "zone_id", Type: model.Text(), Options: &ZoneModel},
		{Name: "expires_at", Type: model.UnixTime()},
		{Name: "updated_at", Type: model.NanoTime()},
	},
}
`
	if err := os.WriteFile(filepath.Join(dir, "models.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	models, err := New().Models(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models {
		if m.ModelName() != "device" {
			continue
		}
		want := map[string]string{"expires_at": model.UnixTime().Name(), "updated_at": model.NanoTime().Name()}
		for _, f := range m.Schema() {
			if f.Name == "zone_id" && (f.Options == nil || f.Options.Name != "zone") {
				t.Errorf("zone_id Options = %+v, want the zone table", f.Options)
			}
			if k, ok := want[f.Name]; ok && f.Type.Name() != k {
				t.Errorf("%s kind = %q, want %q", f.Name, f.Type.Name(), k)
			}
		}
		return
	}
	t.Fatal("model device not found")
}
