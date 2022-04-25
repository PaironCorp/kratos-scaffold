package generator

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/YoogoC/kratos-scaffold/pkg/field"
	"github.com/YoogoC/kratos-scaffold/pkg/util"

	"github.com/iancoleman/strcase"
	"golang.org/x/tools/imports"
)

type DataEnt struct {
	Name        string
	Namespace   string
	AppDirName  string // TODO
	Fields      []field.Field
	StrToPreMap map[string]field.PredicateType
}

func NewDataEnt(name string, ns string, fields []field.Field) DataEnt {
	adn := ""
	if ns != "" {
		adn = "app/" + ns // TODO
	}
	return DataEnt{
		Name:        plural.Singular(strings.ToUpper(name[0:1]) + name[1:]),
		Namespace:   ns,
		AppDirName:  adn,
		Fields:      fields,
		StrToPreMap: field.StrToPreMap,
	}
}

func (b DataEnt) ParamFields() []field.Predicate {
	fs := make([]field.Predicate, 0, len(b.Fields))
	for _, f := range b.Fields {
		for _, predicate := range f.Predicates {
			fs = append(fs, field.Predicate{
				Name:      f.Name + predicate.Type.String(),
				FieldType: f.FieldType,
				EntName:   entName(f.Name) + predicate.Type.EntString(),
				Type:      predicate.Type,
			})
		}
	}
	return fs
}

func (b DataEnt) FieldsExceptPrimary() []field.Field {
	return util.FilterSlice(b.Fields, func(f field.Field) bool {
		return f.Name != "id"
	})
}

//go:embed tmpl/data_ent_data.tmpl
var dataEntDataTmpl string

//go:embed tmpl/data_ent_schema.tmpl
var dataEntSchemaTmpl string

//go:embed tmpl/data_ent_transfer.tmpl
var dataEntTransferTmpl string

func (b DataEnt) Generate() error {
	// 1. gen ent schema and entity
	err := b.genEnt()
	if err != nil {
		return err
	}
	// 2. gen data transfer
	err = b.genTransfer()
	if err != nil {
		return err
	}
	// 3. gen data
	err = b.genData()
	if err != nil {
		return err
	}
	return nil
}

func (b DataEnt) EntPath() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	p := path.Join(wd, b.AppDirName, "internal/data/ent") // TODO
	if _, err := os.Stat(p); os.IsNotExist(err) {
		if err := os.MkdirAll(p, 0o700); err != nil {
			panic(err)
		}
	}
	return p
}

func (b DataEnt) OutPath() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return path.Join(wd, b.AppDirName, "internal/data") // TODO
}

func (b DataEnt) CurrentPkgPath() string {
	return path.Join(ModName(), b.AppDirName, "internal")
}

func (b DataEnt) genEnt() error {
	fmt.Println("generating ent schema...")
	schemaBuf := new(bytes.Buffer)
	funcMap := template.FuncMap{
		"ToLower":  strings.ToLower,
		"ToPlural": plural.Plural,
		"ToCamel":  strcase.ToCamel,
		"ToSnake":  strcase.ToSnake,
	}
	entSchemaTmpl, err := template.New("dataEntDataTmpl").Funcs(funcMap).Parse(dataEntSchemaTmpl)
	if err != nil {
		return err
	}
	err = entSchemaTmpl.Execute(schemaBuf, b)
	if err != nil {
		return err
	}
	p := path.Join(b.EntPath(), "schema", strings.ToLower(b.Name)+".go")
	if _, err := os.Stat(path.Join(b.EntPath(), "schema")); os.IsNotExist(err) {
		if err := os.MkdirAll(path.Join(b.EntPath(), "schema"), 0o700); err != nil {
			return err
		}
	}
	entSchemaContent, err := imports.Process(p, schemaBuf.Bytes(), nil)
	if err != nil {
		return err
	}
	err = os.WriteFile(p, entSchemaContent, 0o644)
	if err != nil {
		return err
	}
	entGengoPath := path.Join(b.EntPath(), "generate.go")
	if _, err := os.Stat(entGengoPath); os.IsNotExist(err) {
		err := GenEntBase(b.EntPath())
		if err != nil {
			return err
		}
	}

	if err := util.Go("mod", "tidy"); err != nil {
		return err
	}
	fmt.Println("exec ent generate...")
	return util.Go("generate", b.EntPath())
}

func GenEntBase(entPath string) error {
	if err := os.MkdirAll(path.Join(entPath, "external"), 0o700); err != nil {
		return err
	}
	fmt.Println("generating ent external...")
	externalContent := `{{ define "external" }}
package ent

import (
	"database/sql"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func (c *Client) DB() *sql.DB {
	switch d := c.driver.(type) {
	case *entsql.Driver:
		return d.DB()
	case *dialect.DebugDriver:
		return d.Driver.(*entsql.Driver).DB()
	default:
		panic("unknown driver")
	}
}

{{ end }}
`
	err := os.WriteFile(path.Join(entPath, "external", "sql.tmpl"), []byte(externalContent), 0o644)
	if err != nil {
		return err
	}
	fmt.Println("generating ent generate...")
	content := `package ent

//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate --template ./external --feature privacy,sql/modifier,sql/lock ./schema
`
	err = os.WriteFile(path.Join(entPath, "generate.go"), []byte(content), 0o644)
	if err != nil {
		return err
	}
	return nil
}

func (b DataEnt) genTransfer() error {
	fmt.Println("generating data transfer...")
	buf := new(bytes.Buffer)
	funcMap := template.FuncMap{
		"ToLower":      strings.ToLower,
		"ToPlural":     plural.Plural,
		"ToCamel":      strcase.ToCamel,
		"ToLowerCamel": strcase.ToLowerCamel,
		"ToEntName":    entName,
	}
	tmpl, err := template.New("dataEntTransferTmpl").Funcs(funcMap).Parse(dataEntTransferTmpl)
	if err != nil {
		return err
	}
	err = tmpl.Execute(buf, b)
	if err != nil {
		return err
	}
	p := path.Join(b.OutPath(), strcase.ToSnake(b.Name)+"_transfer.go")
	content, err := imports.Process(p, buf.Bytes(), nil)
	if err != nil {
		return err
	}
	return os.WriteFile(p, content, 0o644)
}

func entName(s string) string {
	s = strcase.ToCamel(s)
	if len(s) < 2 || strings.ToLower(s[len(s)-2:]) != "id" {
		return s
	}
	return s[:len(s)-2] + "ID"
}

func (b DataEnt) genData() error {
	fmt.Println("generating data...")
	buf := new(bytes.Buffer)
	funcMap := template.FuncMap{
		"ToLower":      strings.ToLower,
		"ToPlural":     plural.Plural,
		"ToCamel":      strcase.ToCamel,
		"ToLowerCamel": strcase.ToLowerCamel,
		"ToEntName":    entName,
		"last": func(x int, a []field.Field) bool {
			return x == len(a)-1
		},
	}
	tmpl, err := template.New("dataEntDataTmpl").Funcs(funcMap).Parse(dataEntDataTmpl)
	if err != nil {
		return err
	}
	err = tmpl.Execute(buf, b)
	if err != nil {
		return err
	}
	p := path.Join(b.OutPath(), strcase.ToSnake(b.Name)+".go")
	content, err := imports.Process(p, buf.Bytes(), nil)
	if err != nil {
		return err
	}
	return os.WriteFile(p, content, 0o644)
}
