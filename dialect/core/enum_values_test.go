package core

import (
	"reflect"
	"testing"
)

func TestParseInlineEnumValues(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"mysql enum", "enum('active','inactive','pending')", []string{"active", "inactive", "pending"}},
		{"mysql set", "set('r','w','x')", []string{"r", "w", "x"}},
		{"escaped quote", "enum('a''b','c')", []string{"a'b", "c"}},
		{"case insensitive keyword", "ENUM('Yes','No')", []string{"Yes", "No"}},
		{"not an enum", "varchar(255)", nil},
		{"int", "int4", nil},
		{"empty", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParseInlineEnumValues(c.in); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("ParseInlineEnumValues(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestEnrichEnumValues(t *testing.T) {
	statusDefault := "active"
	meta := &Metadata{
		Schemas: []Schema{
			{
				Name: "public",
				Types: []Type{
					{Schema: "public", Name: "mood", Kind: "e", Display: "mood", EnumLabels: []string{"sad", "ok", "happy"}},
					{Schema: "public", Name: "address", Kind: "c", Display: "address"},
				},
				Tables: []Table{
					{
						Name: "person",
						Columns: []Column{
							{Name: "id", Type: "int4"},
							{Name: "mood", Type: "mood"},
							{Name: "mood_q", Type: "public.mood"},
							{Name: "status", Type: "enum('active','inactive')", Default: &statusDefault},
							{Name: "addr", Type: "address"},
							{Name: "name", Type: "text"},
						},
					},
				},
				Views: []Table{
					{
						Name: "person_v",
						Columns: []Column{
							{Name: "mood", Type: "mood"},
						},
					},
				},
			},
		},
	}

	EnrichEnumValues(meta)

	cols := meta.Schemas[0].Tables[0].Columns
	checks := map[string][]string{
		"id":     nil,
		"mood":   {"sad", "ok", "happy"},
		"mood_q": {"sad", "ok", "happy"},
		"status": {"active", "inactive"},
		"addr":   nil,
		"name":   nil,
	}
	for _, c := range cols {
		want := checks[c.Name]
		if !reflect.DeepEqual(c.EnumValues, want) {
			t.Fatalf("column %q EnumValues = %v, want %v", c.Name, c.EnumValues, want)
		}
	}

	if got := meta.Schemas[0].Views[0].Columns[0].EnumValues; !reflect.DeepEqual(got, []string{"sad", "ok", "happy"}) {
		t.Fatalf("view column enum not enriched: got %v", got)
	}

	EnrichEnumValues(nil) // must not panic

	// MetaToEnumDict projects only enum columns, omitting the rest.
	dict := MetaToEnumDict(*meta)
	person := dict["public"]["person"]
	if !reflect.DeepEqual(person["mood"], []string{"sad", "ok", "happy"}) ||
		!reflect.DeepEqual(person["status"], []string{"active", "inactive"}) {
		t.Fatalf("MetaToEnumDict enum projection wrong: %v", person)
	}
	if _, ok := person["id"]; ok {
		t.Fatalf("MetaToEnumDict should omit non-enum columns, got %v", person)
	}
}
