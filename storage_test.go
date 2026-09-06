package ormc

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"webtyp.com/model"
)

func parseAndResolve(t *testing.T, g *Generator, src string) ([]StructInfo, error) {
	t.Helper()
	tmpPath := writeTemp(t, src)
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, tmpPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parser.ParseFile: %v", err)
	}
	infos, err := g.parseDefinitionsInFile(tmpPath)
	if err != nil {
		return nil, err
	}
	if err := g.resolveStorage(infos, node); err != nil {
		return nil, err
	}
	return infos, nil
}

func TestField_WidgetRemoved_HardError(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "id", Widget: "input.Text()"},
	},
}
`
	g := New()
	_, err := parseAndResolve(t, g, src)
	if err == nil {
		t.Fatal("expected error for Widget field")
	}
	if !strings.Contains(err.Error(), "Field.Widget was removed") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestField_MissingType_HardError(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "id"},
	},
}
`
	g := New()
	_, err := parseAndResolve(t, g, src)
	if err == nil {
		t.Fatal("expected error for missing Type")
	}
	if !strings.Contains(err.Error(), "kind required") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestComposition_NilArgument_HardError(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var ParentModel = model.Definition{
	Name: "parent",
	Fields: model.Fields{
		{Name: "child", Type: model.Struct(nil)},
	},
}
`
	g := New()
	_, err := parseAndResolve(t, g, src)
	if err == nil {
		t.Fatal("expected error for model.Struct(nil)")
	}
	if !strings.Contains(err.Error(), "non-nil Definition argument") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestComposition_MissingArgument_HardError(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var ParentModel = model.Definition{
	Name: "parent",
	Fields: model.Fields{
		{Name: "child", Type: model.Struct()},
	},
}
`
	g := New()
	_, err := parseAndResolve(t, g, src)
	if err == nil {
		t.Fatal("expected error for model.Struct() with no argument")
	}
	if !strings.Contains(err.Error(), "non-nil Definition argument") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestComposition_RefContradiction_HardError(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var ChildModel = model.Definition{Name: "child"}
var ParentModel = model.Definition{
	Name: "parent",
	Fields: model.Fields{
		{Name: "child", Type: model.Struct(&ChildModel), Ref: &ChildModel},
	},
}
`
	g := New()
	_, err := parseAndResolve(t, g, src)
	if err == nil {
		t.Fatal("expected contradiction error for Ref: alongside composition kind")
	}
	if !strings.Contains(err.Error(), "contradiction") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestKind_NonSelfContainedArgument_HardError(t *testing.T) {
	src := `package p
import (
	"webtyp.com/model"
	"webtyp.com/input"
)
var localOptions = 5
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "role", Type: input.Select(localOptions)},
	},
}
`
	g := New()
	_, err := parseAndResolve(t, g, src)
	if err == nil {
		t.Fatal("expected error for constructor argument referencing a scanned-package identifier")
	}
	if !strings.Contains(err.Error(), "localOptions") {
		t.Errorf("expected error to name the offending identifier, got: %q", err.Error())
	}
}

func TestKind_UnknownPackageAlias_HardError(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "role", Type: input.Select()},
	},
}
`
	g := New()
	_, err := parseAndResolve(t, g, src)
	if err == nil {
		t.Fatal("expected error for unresolvable package alias")
	}
	if !strings.Contains(err.Error(), "unknown package alias") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestKind_LocalKind_HardError(t *testing.T) {
	src := `package p
import "webtyp.com/model"
func LocalKind() model.Kind { return nil }
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "role", Type: LocalKind()},
	},
}
`
	g := New()
	_, err := parseAndResolve(t, g, src)
	if err == nil {
		t.Fatal("expected error for a kind declared in the same package as its Definitions")
	}
	if !strings.Contains(err.Error(), "must live in their own package") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestProbe_Failure_SurfacesOutputVerbatim(t *testing.T) {
	src := `package p
import (
	"webtyp.com/model"
	"webtyp.com/input"
)
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "email", Type: input.Email()},
	},
}
`
	g := New()
	g.probeRunner = func(mainContent, workDir string) (string, error) {
		return "kind.go:1: undefined: input.Email", errors.New("exit status 1")
	}
	_, err := parseAndResolve(t, g, src)
	if err == nil {
		t.Fatal("expected probe failure error")
	}
	if !strings.Contains(err.Error(), "undefined: input.Email") {
		t.Errorf("expected probe compiler output surfaced verbatim, got: %q", err.Error())
	}
}

func TestProbe_CompositionKindStorage_HardError(t *testing.T) {
	src := `package p
import (
	"webtyp.com/model"
	"webtyp.com/input"
)
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "profile", Type: input.Profile()},
	},
}
`
	g := New()
	g.probeRunner = func(mainContent, workDir string) (string, error) {
		return fmt.Sprintf("0=%d\n", int(model.FieldStruct)), nil
	}
	_, err := parseAndResolve(t, g, src)
	if err == nil {
		t.Fatal("expected error: custom composition kinds are unsupported")
	}
	if !strings.Contains(err.Error(), "custom composition kinds are unsupported") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestProbe_GeneratedSource_And_Resolution(t *testing.T) {
	src := `package p
import (
	"webtyp.com/model"
	"webtyp.com/input"
)
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "email", Type: input.Email()},
	},
}
`
	g := New()
	var captured string
	g.probeRunner = func(mainContent, workDir string) (string, error) {
		captured = mainContent
		return fmt.Sprintf("0=%d\n", int(model.FieldText)), nil
	}
	infos, err := parseAndResolve(t, g, src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(captured, `k0 "webtyp.com/input"`) {
		t.Errorf("expected probe source to import the kind's package under a k0 alias, got:\n%s", captured)
	}
	if !strings.Contains(captured, "k0.Email()") {
		t.Errorf("expected probe source to call the constructor via the alias, got:\n%s", captured)
	}
	if infos[0].Fields[0].Type != model.FieldText {
		t.Errorf("expected resolved storage FieldText, got %v", infos[0].Fields[0].Type)
	}
	if infos[0].Fields[0].GoType != "string" {
		t.Errorf("expected GoType string, got %v", infos[0].Fields[0].GoType)
	}
}

func TestGenerate_ProbedKind_EmitsPackageImport(t *testing.T) {
	src := `package p
import (
	"webtyp.com/model"
	"webtyp.com/input"
)
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		{Name: "email", Type: input.Email(), NotNull: true},
		{Name: "secret", Type: model.Text(), Exclude: true},
	},
}
`
	tmpPath := writeTemp(t, src)
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, tmpPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	g := New()
	g.probeRunner = func(mainContent, workDir string) (string, error) {
		return fmt.Sprintf("0=%d\n", int(model.FieldText)), nil
	}
	infos, err := g.parseDefinitionsInFile(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.resolveStorage(infos, node); err != nil {
		t.Fatal(err)
	}
	if err := g.GenerateForFile(infos, tmpPath); err != nil {
		t.Fatal(err)
	}

	genFile := strings.TrimSuffix(tmpPath, ".go") + "_orm.go"
	content, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !strings.Contains(s, `"webtyp.com/input"`) {
		t.Errorf("expected generated file to import the probed kind's package, got:\n%s", s)
	}
	if !strings.Contains(s, "Type: input.Email()") {
		t.Errorf("expected Schema() to re-emit the constructor verbatim, got:\n%s", s)
	}
}

func TestProbe_Cache_HitAndInvalidate(t *testing.T) {
	dir := t.TempDir()
	writeGoMod := func(content string) {
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeGoMod("module cache_test\n\ngo 1.21\n")

	src := `package p
import (
	"webtyp.com/model"
	"webtyp.com/input"
)
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "email", Type: input.Email()},
	},
}
`
	tmpPath := writeTemp(t, src)
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, tmpPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	g := New()
	g.SetRootDir(dir)
	callCount := 0
	g.probeRunner = func(mainContent, workDir string) (string, error) {
		callCount++
		return fmt.Sprintf("0=%d\n", int(model.FieldText)), nil
	}

	resolveOnce := func() {
		infos, err := g.parseDefinitionsInFile(tmpPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := g.resolveStorage(infos, node); err != nil {
			t.Fatal(err)
		}
	}

	resolveOnce()
	if callCount != 1 {
		t.Fatalf("expected 1 probe run, got %d", callCount)
	}

	resolveOnce()
	if callCount != 1 {
		t.Fatalf("expected cache hit (still 1 probe run), got %d", callCount)
	}

	writeGoMod("module cache_test\n\ngo 1.22\n")
	resolveOnce()
	if callCount != 2 {
		t.Fatalf("expected go.mod change to invalidate cache (2 probe runs), got %d", callCount)
	}
}
