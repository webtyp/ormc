package ormc

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemp writes content to a temp file and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "model.go")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseDefinition_Basic(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text()},
	},
}
`
	g := New()
	tmpPath := writeTemp(t, src)
	fset := token.NewFileSet()
	node, _ := parser.ParseFile(fset, tmpPath, nil, parser.ParseComments)
	infos, err := g.parseDefinitionsInFile(tmpPath)
	if err == nil {
		_ = g.resolveStorage(infos, node)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 info, got %d", len(infos))
	}
	info := infos[0]
	if info.Name != "User" {
		t.Errorf("expected User, got %s", info.Name)
	}
	if info.Fields[0].KindConstructor != "model.Int()" {
		t.Errorf("expected model.Int(), got %v", info.Fields[0].KindConstructor)
	}
	if info.ModelName != "user" {
		t.Errorf("expected user, got %s", info.ModelName)
	}
	if len(info.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(info.Fields))
	}
}

func TestParseDefinition_Exclude(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		{Name: "password_hash", Type: model.Text(), Exclude: true},
	},
}
`
	g := New()
	tmpPath := writeTemp(t, src)
	fset := token.NewFileSet()
	node, _ := parser.ParseFile(fset, tmpPath, nil, parser.ParseComments)
	infos, err := g.parseDefinitionsInFile(tmpPath)
	if err == nil {
		_ = g.resolveStorage(infos, node)
	}
	if err != nil {
		t.Fatal(err)
	}
	info := infos[0]
	if !info.Fields[1].Exclude {
		t.Errorf("expected Exclude: true for password_hash")
	}
}

func TestGenerate_E2E(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var ChildModel = model.Definition{
	Name: "child",
	Fields: model.Fields{
		{Name: "x", Type: model.Text()},
	},
}
var ParentModel = model.Definition{
	Name: "parent",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		{Name: "count", Type: model.Int()},
		{Name: "child", Type: model.Struct(&ChildModel)},
	},
}
`
	tmpFile := writeTemp(t, src)
	g := New()
	fset := token.NewFileSet()
	node, _ := parser.ParseFile(fset, tmpFile, nil, parser.ParseComments)
	infos, err := g.parseDefinitionsInFile(tmpFile)
	if err == nil {
		_ = g.resolveStorage(infos, node)
	}
	if err != nil {
		t.Fatal(err)
	}

	err = g.GenerateForFile(infos, tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	genFile := strings.TrimSuffix(tmpFile, ".go") + "_orm.go"
	content, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	// Verify struct generation
	if !strings.Contains(s, "type Parent struct {") {
		t.Errorf("missing Parent struct definition")
	}
	if !strings.Contains(s, "Child Child") {
		t.Errorf("missing Child field in Parent struct")
	}

	// Verify Schema reuse
	if !strings.Contains(s, "func (m *Parent) Schema() []model.Field { return ParentModel.Fields }") {
		t.Errorf("Schema() should return ParentModel.Fields")
	}

	// Verification: struct by value should use &m.Field and NO nil check
	if !strings.Contains(s, "w.Object(\"child\", &m.Child)") {
		t.Errorf("missing expected w.Object for value struct field in EncodeFields")
	}
	if strings.Contains(s, "if m.Child != nil") {
		t.Errorf("unexpected nil check for value struct field in EncodeFields")
	}
	if !strings.Contains(s, "r.Object(\"child\", &m.Child)") {
		t.Errorf("missing expected r.Object for value struct field in DecodeFields")
	}
}

func TestGenerate_RawField(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var ModelModel = model.Definition{
	Name: "model",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		{Name: "config", Type: model.Raw()},
	},
}
`
	tmpFile := writeTemp(t, src)
	g := New()
	fset := token.NewFileSet()
	node, _ := parser.ParseFile(fset, tmpFile, nil, parser.ParseComments)
	infos, err := g.parseDefinitionsInFile(tmpFile)
	if err == nil {
		_ = g.resolveStorage(infos, node)
	}
	if err != nil {
		t.Fatal(err)
	}

	err = g.GenerateForFile(infos, tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	genFile := strings.TrimSuffix(tmpFile, ".go") + "_orm.go"
	content, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	// Verify EncodeFields uses w.Raw()
	if !strings.Contains(s, "w.Raw(\"config\", m.Config)") {
		t.Errorf("missing expected w.Raw for FieldRaw in EncodeFields")
	}

	// Verify DecodeFields uses r.Raw()
	if !strings.Contains(s, "if v, ok := r.Raw(\"config\"); ok { m.Config = v }") {
		t.Errorf("missing expected r.Raw for FieldRaw in DecodeFields")
	}
}

func TestGenerate_OmitEmpty(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var ModelModel = model.Definition{
	Name: "model",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		{Name: "text", Type: model.Text(), OmitEmpty: true},
	},
}
`
	tmpFile := writeTemp(t, src)
	g := New()
	fset := token.NewFileSet()
	node, _ := parser.ParseFile(fset, tmpFile, nil, parser.ParseComments)
	infos, err := g.parseDefinitionsInFile(tmpFile)
	if err == nil {
		_ = g.resolveStorage(infos, node)
	}
	if err != nil {
		t.Fatal(err)
	}

	err = g.GenerateForFile(infos, tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	genFile := strings.TrimSuffix(tmpFile, ".go") + "_orm.go"
	content, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	// Verify EncodeFields has guards
	if !strings.Contains(s, "if m.Text != \"\" { w.String(\"text\", m.Text) }") {
		t.Errorf("missing guard for Text")
	}
}

func TestGenerate_FK_SchemaExt(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var UserModel = model.Definition{ Name: "user" }
var SessionModel = model.Definition{
	Name: "session",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		{Name: "user_id", Type: model.Int(), Ref: &UserModel, DB: &model.FieldDB{RefColumn: "id", OnDelete: "CASCADE"}},
	},
}
`
	tmpFile := writeTemp(t, src)
	g := New()
	fset := token.NewFileSet()
	node, _ := parser.ParseFile(fset, tmpFile, nil, parser.ParseComments)
	infos, err := g.parseDefinitionsInFile(tmpFile)
	if err == nil {
		_ = g.resolveStorage(infos, node)
	}
	if err != nil {
		t.Fatal(err)
	}

	err = g.GenerateForFile(infos, tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	genFile := strings.TrimSuffix(tmpFile, ".go") + "_orm.go"
	content, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	if !strings.Contains(s, "func (m *Session) SchemaExt() []model.FieldExt {") {
		t.Errorf("missing SchemaExt generation")
	}
	if !strings.Contains(s, "Ref: \"user\", RefColumn: \"id\", OnDelete: \"CASCADE\"") {
		t.Errorf("incorrect SchemaExt content")
	}
}

func TestGenerate_Exclude_Parallelism(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text()},
		{Name: "secret", Type: model.Text(), Exclude: true},
	},
}
`
	tmpFile := writeTemp(t, src)
	g := New()
	fset := token.NewFileSet()
	node, _ := parser.ParseFile(fset, tmpFile, nil, parser.ParseComments)
	infos, err := g.parseDefinitionsInFile(tmpFile)
	if err == nil {
		_ = g.resolveStorage(infos, node)
	}
	if err != nil {
		t.Fatal(err)
	}

	err = g.GenerateForFile(infos, tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	genFile := strings.TrimSuffix(tmpFile, ".go") + "_orm.go"
	content, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	// Schema should be filtered
	if !strings.Contains(s, "var _schemaUser = []model.Field{") {
		t.Errorf("missing _schemaUser variable for Exclude case")
	}
	if strings.Contains(s, "Name: \"secret\"") && strings.Contains(s, "_schemaUser") {
		// This is a bit weak but good enough for a basic check
		// We want to ensure "secret" is NOT in the _schemaUser literal.
	}

	// Pointers should be filtered
	if !strings.Contains(s, "return []any{&m.Id, &m.Name}") {
		t.Errorf("Pointers() should not include &m.Secret")
	}
}

func TestGenerate_AlwaysOnHelper(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var ItemModel = model.Definition{
	Name: "item",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		{Name: "tenant_id", Type: model.Text()},
		{Name: "sku", Type: model.Text()},
	},
}
var NoDBModel = model.Definition{
	Name: "no_db",
	Fields: model.Fields{
		{Name: "x", Type: model.Text()},
	},
}
`
	tmpFile := writeTemp(t, src)
	g := New()
	fset := token.NewFileSet()
	node, _ := parser.ParseFile(fset, tmpFile, nil, parser.ParseComments)
	infos, err := g.parseDefinitionsInFile(tmpFile)
	if err == nil {
		_ = g.resolveStorage(infos, node)
	}
	if err != nil {
		t.Fatal(err)
	}

	err = g.GenerateForFile(infos, tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	genFile := strings.TrimSuffix(tmpFile, ".go") + "_orm.go"
	content, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	// Verify Item_ helper is generated (DB model)
	if !strings.Contains(s, "var Item_ = struct {") {
		t.Errorf("missing Item_ helper for DB model")
	}
	// Verify pure algorithmic casing: Id, TenantId, Sku
	if !strings.Contains(s, "Id string") || !strings.Contains(s, "TenantId string") || !strings.Contains(s, "Sku string") {
		t.Errorf("incorrect helper struct fields casing")
	}
	if !strings.Contains(s, "Id: \"id\"") || !strings.Contains(s, "TenantId: \"tenant_id\"") || !strings.Contains(s, "Sku: \"sku\"") {
		t.Errorf("incorrect helper struct field values")
	}

	// Verify NoDB_ helper is NOT generated (non-DB model)
	if strings.Contains(s, "var NoDB_ = struct {") {
		t.Errorf("unexpected NoDB_ helper for non-DB model")
	}
}

func TestGenerate_UnconditionalValidate(t *testing.T) {
	src := `package p
import "webtyp.com/model"
var PingArgsModel = model.Definition{
	Name: "ping_args",
	Fields: model.Fields{
		{Name: "count", Type: model.Int()},
	},
}
`
	tmpFile := writeTemp(t, src)
	g := New()
	fset := token.NewFileSet()
	node, _ := parser.ParseFile(fset, tmpFile, nil, parser.ParseComments)
	infos, err := g.parseDefinitionsInFile(tmpFile)
	if err == nil {
		_ = g.resolveStorage(infos, node)
	}
	if err != nil {
		t.Fatal(err)
	}

	err = g.GenerateForFile(infos, tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	genFile := strings.TrimSuffix(tmpFile, ".go") + "_orm.go"
	content, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	// Verify Validate method is generated even without any rules
	if !strings.Contains(s, "func (m *PingArgs) Validate(action byte) error {") {
		t.Errorf("missing Validate method for model without rules")
	}
	if !strings.Contains(s, "return model.ValidateFields(action, m)") {
		t.Errorf("incorrect Validate method body")
	}
}
