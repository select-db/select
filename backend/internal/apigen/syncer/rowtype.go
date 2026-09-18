package syncer

import (
	"backend/internal/apigen/codegen"
	"backend/internal/apigen/schema"
	"fmt"
	"go/format"
	"strings"
)

// EmitSyncRowType renders the sync wire struct types.<Sing>Row. It lives in the
// shared types package and carries every column — including the tenant and
// soft-delete, which the client needs locally (unlike the read API surface).
func EmitSyncRowType(e schema.Entity) (codegen.GenFile, error) {
	sing := codegen.Pascal(e.Table)
	d := struct {
		Sing, Table string
		NeedTime    bool
		Fields      []rowField
	}{Sing: sing, Table: e.Table, NeedTime: entityHasTime(e)}
	for _, f := range e.Fields {
		tag := f.Column
		if f.Column == schema.SoftDeleteColumn {
			tag += ",omitempty"
		}
		d.Fields = append(d.Fields, rowField{Name: codegen.GoField(f.Column), Type: rowGoType(e, f), Tag: fmt.Sprintf("json:%q", tag)})
	}

	var b strings.Builder
	if err := rowTypeTmpl.Execute(&b, d); err != nil {
		return codegen.GenFile{}, err
	}
	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return codegen.GenFile{}, err
	}
	return codegen.GenFile{Name: e.Table + "_row_gen.go", Content: string(formatted)}, nil
}

var rowTypeTmpl = mustParse("rowtype.go.tmpl")

// rowGoType is the plain Go type for a sync-row field. The soft-delete column
// is a nullable pointer; every other nullable column uses its zero value.
func rowGoType(e schema.Entity, f schema.Field) string {
	switch f.Kind {
	case schema.KindInt:
		return "int64"
	case schema.KindBool:
		return "bool"
	case schema.KindTime:
		if f.Column == schema.SoftDeleteColumn {
			return "*time.Time"
		}
		return "time.Time"
	default: // uuid, inet, text
		return "string"
	}
}

func entityHasTime(e schema.Entity) bool {
	for _, f := range e.Fields {
		if f.Kind == schema.KindTime {
			return true
		}
	}
	return false
}
