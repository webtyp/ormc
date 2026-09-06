package ormc

import "webtyp.com/model"

import (
	"os"
	"sort"
	"strings"

	"webtyp.com/fmt"
)

// hasExcludedField reports whether the Definition drops any field from the
// generated struct. Only then does the output carry a filtered _schemaX literal:
// otherwise Schema() returns XModel.Fields and no field literal is re-emitted.
func hasExcludedField(fields []FieldInfo) bool {
	for _, f := range fields {
		if f.Exclude {
			return true
		}
	}
	return false
}

// GenerateForFile writes ORM implementations for all infos into one file.
func (o *Generator) GenerateForFile(infos []StructInfo, sourceFile string) error {
	if len(infos) == 0 {
		return nil
	}
	buf := fmt.Convert()

	// File Header
	buf.Write(GeneratedHeader + "\n\n")
	buf.Write(fmt.Sprintf("package %s\n\n", infos[0].PackageName))

	hasORM := false
	// kindImports collects the packages that non-model kind constructors
	// (webtyp/input kinds, project-custom kinds) live in, keyed by import path.
	kindImports := make(map[string]string) // path -> alias used in the scanned source
	for _, info := range infos {
		if !info.NoDB {
			hasORM = true
		} else {
			// Check if it has scalar FKs (SchemaExt)
			for _, f := range info.Fields {
				if f.Ref != "" && f.Type != model.FieldStruct && f.Type != model.FieldStructSlice {
					hasORM = true
					break
				}
			}
		}
		// Kind constructors (input.Email(), custom kinds) are only WRITTEN into the
		// output inside the _schemaX literal, which exists only when a field is
		// excluded — otherwise Schema() returns XModel.Fields and the generated file
		// never names a kind. Importing them unconditionally emits an unused import,
		// and the generated file does not compile.
		if !hasExcludedField(info.Fields) {
			continue
		}
		for _, f := range info.Fields {
			if f.KindImportPath != "" {
				kindImports[f.KindImportPath] = f.KindImportAlias
			}
		}
	}

	buf.Write("import (\n")
	buf.Write("\t\"webtyp.com/model\"\n")
	if hasORM {
		buf.Write("\t\"webtyp.com/orm\"\n")
	}

	if len(kindImports) > 0 {
		paths := make([]string, 0, len(kindImports))
		for path := range kindImports {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			alias := kindImports[path]
			defaultAlias := path
			if idx := strings.LastIndex(path, "/"); idx != -1 {
				defaultAlias = path[idx+1:]
			}
			if alias != "" && alias != defaultAlias {
				buf.Write(fmt.Sprintf("\t%s \"%s\"\n", alias, path))
			} else {
				buf.Write(fmt.Sprintf("\t\"%s\"\n", path))
			}
		}
	}
	buf.Write(")\n\n")

	for _, info := range infos {
		// Struct definition
		buf.Write(fmt.Sprintf("type %s struct {\n", info.Name))
		for _, f := range info.Fields {
			buf.Write(fmt.Sprintf("\t%s %s\n", f.Name, f.GoType))
		}
		buf.Write("}\n\n")

		// Model Interface Methods
		buf.Write(fmt.Sprintf("func (m *%s) ModelName() string { return \"%s\" }\n\n", info.Name, info.ModelName))

		if hasExcludedField(info.Fields) {
			buf.Write(fmt.Sprintf("var _schema%s = []model.Field{\n", info.Name))
			for _, f := range info.Fields {
				if f.Exclude {
					continue
				}

				buf.Write(fmt.Sprintf("\t\t{Name: \"%s\", Type: %s", f.ColumnName, f.KindConstructor))
				if !info.NoDB && (f.PK || f.Unique || f.AutoInc) {
					buf.Write(", DB: &model.FieldDB{")
					var parts []string
					if f.PK {
						parts = append(parts, "PK: true")
					}
					if f.Unique {
						parts = append(parts, "Unique: true")
					}
					if f.AutoInc {
						parts = append(parts, "AutoInc: true")
					}
					buf.Write(fmt.Convert(parts).Join(", ").String())
					buf.Write("}")
				}
				if f.NotNull {
					buf.Write(", NotNull: true")
				}
				if f.OmitEmpty {
					buf.Write(", OmitEmpty: true")
				}
				writePermittedFields(buf, f)
				buf.Write("},\n")
			}
			buf.Write("\t}\n\n")
			buf.Write(fmt.Sprintf("func (m *%s) Schema() []model.Field { return _schema%s }\n\n", info.Name, info.Name))
		} else {
			buf.Write(fmt.Sprintf("func (m *%s) Schema() []model.Field { return %sModel.Fields }\n\n", info.Name, info.Name))
		}

		buf.Write(fmt.Sprintf("func (m *%s) Pointers() []any { return []any{", info.Name))
		first := true
		for _, f := range info.Fields {
			if f.Exclude {
				continue
			}
			if !first {
				buf.Write(", ")
			}
			buf.Write(fmt.Sprintf("&m.%s", f.Name))
			first = false
		}
		buf.Write("} }\n\n")

		buf.Write(fmt.Sprintf("func (m *%s) IsNil() bool { return m == nil }\n\n", info.Name))

		buf.Write(fmt.Sprintf("func (m *%s) EncodeFields(w model.FieldWriter) {\n", info.Name))
		for _, f := range info.Fields {
			if f.Exclude {
				continue
			}
			var line string
			switch f.Type {
			case model.FieldText:
				line = fmt.Sprintf("w.String(\"%s\", m.%s)", f.ColumnName, f.Name)
			case model.FieldRaw:
				line = fmt.Sprintf("w.Raw(\"%s\", m.%s)", f.ColumnName, f.Name)
			case model.FieldInt:
				line = fmt.Sprintf("w.Int(\"%s\", m.%s)", f.ColumnName, f.Name)
			case model.FieldFloat:
				line = fmt.Sprintf("w.Float(\"%s\", m.%s)", f.ColumnName, f.Name)
			case model.FieldBool:
				line = fmt.Sprintf("w.Bool(\"%s\", m.%s)", f.ColumnName, f.Name)
			case model.FieldBlob:
				line = fmt.Sprintf("w.Bytes(\"%s\", m.%s)", f.ColumnName, f.Name)
			case model.FieldStruct:
				if f.IsPointer {
					if f.OmitEmpty {
						line = fmt.Sprintf("w.Object(\"%s\", m.%s)", f.ColumnName, f.Name)
					} else {
						buf.Write(fmt.Sprintf("\tif m.%s != nil { w.Object(\"%s\", m.%s) } else { w.Null(\"%s\") }\n", f.Name, f.ColumnName, f.Name, f.ColumnName))
					}
				} else {
					line = fmt.Sprintf("w.Object(\"%s\", &m.%s)", f.ColumnName, f.Name)
				}
			case model.FieldIntSlice:
				if f.OmitEmpty {
					buf.Write(fmt.Sprintf("\tif len(m.%s) != 0 {\n", f.Name))
				}
				buf.Write(fmt.Sprintf("\t\t{\n\t\t\tarr := w.Array(\"%s\", len(m.%s))\n\t\t\tfor _, x := range m.%s {\n\t\t\t\tarr.Int(int64(x))\n\t\t\t}\n\t\t\tarr.Close()\n\t\t}\n", f.ColumnName, f.Name, f.Name))
				if f.OmitEmpty {
					buf.Write("\t}\n")
				}
			case model.FieldStructSlice:
				isPtr := fmt.HasPrefix(f.GoType, "[]*")
				if f.OmitEmpty {
					buf.Write(fmt.Sprintf("\tif len(m.%s) != 0 {\n", f.Name))
				}
				buf.Write(fmt.Sprintf("\t\t{\n\t\t\tarr := w.Array(\"%s\", len(m.%s))\n\t\t\tfor _, x := range m.%s {\n", f.ColumnName, f.Name, f.Name))
				if isPtr {
					buf.Write("\t\t\t\tif x != nil { arr.Object(x) }\n")
				} else {
					buf.Write("\t\t\t\tarr.Object(&x)\n")
				}
				buf.Write("\t\t\t}\n\t\t\tarr.Close()\n\t\t}\n")
				if f.OmitEmpty {
					buf.Write("\t}\n")
				}
			}

			if f.OmitEmpty && line != "" {
				guard := ""
				switch f.Type {
				case model.FieldText:
					guard = fmt.Sprintf("m.%s != \"\"", f.Name)
				case model.FieldRaw, model.FieldBlob:
					guard = fmt.Sprintf("len(m.%s) != 0", f.Name)
				case model.FieldInt, model.FieldFloat:
					guard = fmt.Sprintf("m.%s != 0", f.Name)
				case model.FieldBool:
					guard = fmt.Sprintf("m.%s", f.Name)
				case model.FieldStruct:
					if f.IsPointer {
						guard = fmt.Sprintf("m.%s != nil", f.Name)
					}
				}
				if guard != "" {
					buf.Write(fmt.Sprintf("\tif %s { %s }\n", guard, line))
					continue
				}
			}
			if line != "" {
				buf.Write("\t" + line + "\n")
			}
		}
		buf.Write("}\n\n")

		buf.Write(fmt.Sprintf("func (m *%s) DecodeFields(r model.FieldReader) {\n", info.Name))
		for _, f := range info.Fields {
			if f.Exclude {
				continue
			}
			switch f.Type {
			case model.FieldText:
				buf.Write(fmt.Sprintf("\tif v, ok := r.String(\"%s\"); ok { m.%s = v }\n", f.ColumnName, f.Name))
			case model.FieldRaw:
				buf.Write(fmt.Sprintf("\tif v, ok := r.Raw(\"%s\"); ok { m.%s = v }\n", f.ColumnName, f.Name))
			case model.FieldInt:
				buf.Write(fmt.Sprintf("\tif v, ok := r.Int(\"%s\"); ok { m.%s = v }\n", f.ColumnName, f.Name))
			case model.FieldFloat:
				buf.Write(fmt.Sprintf("\tif v, ok := r.Float(\"%s\"); ok { m.%s = v }\n", f.ColumnName, f.Name))
			case model.FieldBool:
				buf.Write(fmt.Sprintf("\tif v, ok := r.Bool(\"%s\"); ok { m.%s = v }\n", f.ColumnName, f.Name))
			case model.FieldBlob:
				buf.Write(fmt.Sprintf("\tif v, ok := r.Bytes(\"%s\"); ok { m.%s = v }\n", f.ColumnName, f.Name))
			case model.FieldStruct:
				elemType := f.GoType
				if f.IsPointer {
					buf.Write(fmt.Sprintf("\tif m.%s == nil { m.%s = new(%s) }\n", f.Name, f.Name, elemType))
					buf.Write(fmt.Sprintf("\tif !r.Object(\"%s\", m.%s) { m.%s = nil }\n", f.ColumnName, f.Name, f.Name))
				} else {
					buf.Write(fmt.Sprintf("\tr.Object(\"%s\", &m.%s)\n", f.ColumnName, f.Name))
				}
			case model.FieldIntSlice:
				elemType := fmt.Convert(f.GoType).TrimPrefix("[]").String()
				buf.Write(fmt.Sprintf("\tif arr, ok := r.Array(\"%s\"); ok {\n", f.ColumnName))
				buf.Write(fmt.Sprintf("\t\tn := arr.Len()\n\t\tm.%s = make(%s, n)\n\t\tfor i := 0; i < n; i++ {\n\t\t\tm.%s[i] = %s(arr.Int(i))\n\t\t}\n\t}\n", f.Name, f.GoType, f.Name, elemType))
			case model.FieldStructSlice:
				isPtr := fmt.HasPrefix(f.GoType, "[]*")
				elemType := fmt.Convert(f.GoType).TrimPrefix("[]").TrimPrefix("*").String()
				buf.Write(fmt.Sprintf("\tif arr, ok := r.Array(\"%s\"); ok {\n", f.ColumnName))
				buf.Write(fmt.Sprintf("\t\tn := arr.Len()\n\t\tm.%s = make(%s, n)\n\t\tfor i := 0; i < n; i++ {\n", f.Name, f.GoType))
				if isPtr {
					buf.Write(fmt.Sprintf("\t\t\tm.%s[i] = new(%s)\n", f.Name, elemType))
					buf.Write(fmt.Sprintf("\t\t\tarr.Object(i, m.%s[i])\n", f.Name))
				} else {
					buf.Write(fmt.Sprintf("\t\t\tarr.Object(i, &m.%s[i])\n", f.Name))
				}
				buf.Write("\t\t}\n\t}\n")
			}
		}
		buf.Write("}\n\n")

		// RenameProvider
		hasOldNames := false
		for _, f := range info.Fields {
			if f.OldName != "" {
				hasOldNames = true
				break
			}
		}
		if hasOldNames {
			buf.Write(fmt.Sprintf("func (m *%s) OldNames() map[string]string {\n", info.Name))
			buf.Write("\treturn map[string]string{\n")
			for _, f := range info.Fields {
				if f.OldName != "" {
					buf.Write(fmt.Sprintf("\t\t\"%s\": \"%s\",\n", f.ColumnName, f.OldName))
				}
			}
			buf.Write("\t}\n}\n\n")
		}

		buf.Write(fmt.Sprintf("type %sList []*%s\n\n", info.Name, info.Name))
		buf.Write(fmt.Sprintf("func (s *%sList) Schema() []model.Field { return nil }\n", info.Name))
		buf.Write(fmt.Sprintf("func (s *%sList) Pointers() []any     { return nil }\n", info.Name))
		buf.Write(fmt.Sprintf("func (s *%sList) Len() int             { return len(*s) }\n", info.Name))
		buf.Write(fmt.Sprintf("func (s *%sList) At(i int) model.Fielder { return (*s)[i] }\n", info.Name))
		buf.Write(fmt.Sprintf("func (s *%sList) Append() model.Fielder  { v := &%s{}; *s = append(*s, v); return v }\n", info.Name, info.Name))
		buf.Write(fmt.Sprintf("func (s *%sList) IsNil() bool          { return s == nil }\n", info.Name))
		buf.Write(fmt.Sprintf("func (s *%sList) EncodeFields(_ model.FieldWriter) {}\n", info.Name))
		buf.Write(fmt.Sprintf("func (s *%sList) DecodeFields(_ model.FieldReader) {}\n\n", info.Name))

		buf.Write(fmt.Sprintf("func (m *%s) Validate(action byte) error {\n", info.Name))
		buf.Write("\treturn model.ValidateFields(action, m)\n")
		buf.Write("}\n\n")

		if !info.NoDB {
			// Metadata Descriptors
			buf.Write(fmt.Sprintf("var %s_ = struct {\n", info.Name))
			for _, f := range info.Fields {
				buf.Write(fmt.Sprintf("\t%s string\n", f.Name))
			}
			buf.Write("}{\n")
			for _, f := range info.Fields {
				buf.Write(fmt.Sprintf("\t%s: \"%s\",\n", f.Name, f.ColumnName))
			}
			buf.Write("}\n\n")

			// Typed Read Operations
			buf.Write(fmt.Sprintf("func ReadOne%s(qb *orm.QB, model *%s) (*%s, error) {\n", info.Name, info.Name, info.Name))
			buf.Write("\terr := qb.ReadOne()\n")
			buf.Write("\tif err != nil {\n")
			buf.Write("\t\treturn nil, err\n")
			buf.Write("\t}\n")
			buf.Write("\treturn model, nil\n")
			buf.Write("}\n\n")

			buf.Write(fmt.Sprintf("func ReadAll%s(qb *orm.QB) (%sList, error) {\n", info.Name, info.Name))
			buf.Write(fmt.Sprintf("\tvar results %sList\n", info.Name))
			buf.Write("\terr := qb.ReadAll(\n")
			buf.Write(fmt.Sprintf("\t\tfunc() model.Model { return &%s{} },\n", info.Name))
			buf.Write(fmt.Sprintf("\t\tfunc(m model.Model) { results = append(results, m.(*%s)) },\n", info.Name))
			buf.Write("\t)\n")
			buf.Write("\treturn results, err\n")
			buf.Write("}\n\n")

			// SchemaExt
			hasFK := false
			for _, f := range info.Fields {
				if f.Ref != "" && f.Type != model.FieldStruct && f.Type != model.FieldStructSlice {
					hasFK = true
					break
				}
			}
			if hasFK {
				buf.Write(fmt.Sprintf("func (m *%s) SchemaExt() []model.FieldExt {\n", info.Name))
				buf.Write("\treturn []model.FieldExt{\n")
				schemaVar := fmt.Sprintf("%sModel.Fields", info.Name)
				if hasExcludedField(info.Fields) {
					schemaVar = fmt.Sprintf("_schema%s", info.Name)
				}

				j := 0
				for _, f := range info.Fields {
					if f.Exclude {
						continue
					}
					if f.Ref != "" && f.Type != model.FieldStruct && f.Type != model.FieldStructSlice {
						// Resolve Ref name from Definition variable name
						refName := ToSnakeCase(strings.TrimSuffix(strings.TrimPrefix(f.Ref, "&"), "Model"))
						buf.Write(fmt.Sprintf("\t\t{Field: %s[%d], Ref: \"%s\", RefColumn: \"%s\", OnDelete: \"%s\"},\n", schemaVar, j, refName, f.RefColumn, f.OnDelete))
					}
					j++
				}
				buf.Write("\t}\n}\n\n")
			}
		}
	}

	outName := fmt.Convert(sourceFile).TrimSuffix(".go").String() + "_orm.go"
	return os.WriteFile(outName, buf.Bytes(), 0644)
}

type modelStub struct {
	name   string
	schema []model.Field
	exts   []model.FieldExt
}

func (m *modelStub) ModelName() string                { return m.name }
func (m *modelStub) Schema() []model.Field            { return m.schema }
func (m *modelStub) Pointers() []any                  { return nil }
func (m *modelStub) IsNil() bool                      { return m == nil }
func (m *modelStub) EncodeFields(_ model.FieldWriter) {}
func (m *modelStub) DecodeFields(_ model.FieldReader) {}
func (m *modelStub) SchemaExt() []model.FieldExt       { return m.exts }

func newModelStub(info StructInfo) *modelStub {
	stub := &modelStub{name: info.ModelName}
	for _, f := range info.Fields {
		var kind model.Kind
		switch goTypeToFieldType(f.GoType) {
		case model.FieldInt:
			kind = model.Int()
		case model.FieldFloat:
			kind = model.Float()
		case model.FieldBool:
			kind = model.Bool()
		case model.FieldBlob:
			kind = model.Blob()
		case model.FieldRaw:
			kind = model.Raw()
		case model.FieldStruct:
			kind = model.Struct(nil)
		case model.FieldIntSlice:
			kind = model.IntSlice()
		case model.FieldStructSlice:
			kind = model.StructSlice(nil)
		default:
			kind = model.Text()
		}

		field := model.Field{
			Name:    f.ColumnName,
			Type:    kind,
			NotNull: f.NotNull,
		}
		if f.PK || f.Unique || f.AutoInc {
			field.DB = &model.FieldDB{PK: f.PK, Unique: f.Unique, AutoInc: f.AutoInc}
		}
		if f.Maximum > 0 {
			if field.DB == nil {
				field.DB = &model.FieldDB{}
			}
			field.Permitted.Maximum = f.Maximum
		}
		stub.schema = append(stub.schema, field)
		if f.Ref != "" {
			stub.exts = append(stub.exts, model.FieldExt{
				Field:     field,
				Ref:       f.Ref,
				RefColumn: f.RefColumn,
				OnDelete:  f.OnDelete,
			})
		}
	}
	return stub
}

func goTypeToFieldType(goType string) model.FieldType {
	switch goType {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return model.FieldInt
	case "float32", "float64":
		return model.FieldFloat
	case "bool":
		return model.FieldBool
	case "[]byte":
		return model.FieldBlob
	default:
		return model.FieldText
	}
}

// Exporter is implemented by SQL adapter compilers (sqlt, postgres) that can render a
// full schema export. Defined here — not imported from ddlc — per Go convention: the
// consumer of a single-method interface owns the interface, not the implementer.
type Exporter interface {
	ExportDDL(models []model.Model) (string, error)
}

// ExportSQL scans the directory for models and returns the full DDL.
// Requires an injected Exporter (e.g. from sqlt or postgres).
func (g *Generator) ExportSQL(root string, exporter Exporter) (string, error) {
	g.rootDir = root
	all, _, _, err := g.collectAllStructs()
	if err != nil {
		return "", err
	}
	var models []model.Model
	for _, info := range all {
		if info.NoDB {
			continue
		}
		models = append(models, newModelStub(info))
	}
	if len(models) == 0 {
		return "", ErrNoModelsFound
	}
	return exporter.ExportDDL(models)
}
