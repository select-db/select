package api

import (
	"encoding/json"

	"backend/internal/apigen/schema"
)

// Capabilities is the machine-readable contract the frontend consumes to build
// filter/sort UIs and know which operations it may call. It is a pure
// projection of the entity model — no SQL, no sqlc, no runtime.
type Capabilities struct {
	Resources []ResourceCap `json:"resources"`
}

type ResourceCap struct {
	Name       string           `json:"name"`
	Operations map[string]OpCap `json:"operations"`
	Fields     []FieldCap       `json:"fields"`
	Relations  []RelationCap    `json:"relations,omitempty"`
}

// OpCap is one enabled endpoint op and the actions it requires (empty ⇒ any
// workspace member).
type OpCap struct {
	Requires []string `json:"requires,omitempty"`
}

type FieldCap struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Operators  []string `json:"operators"`
	Sortable   bool     `json:"sortable"`
	Selectable bool     `json:"selectable"`
	Values     []string `json:"values,omitempty"`
	Lookup     string   `json:"lookup,omitempty"`
	JSONPaths  []string `json:"jsonPaths,omitempty"`
}

type RelationCap struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

// EmitAPICapabilities renders the capabilities document for every API-exposed
// entity. Only non-hidden fields are surfaced; operators derive from the field
// kind (overridable by @app.ops).
func EmitAPICapabilities(entities []schema.Entity) ([]byte, error) {
	caps := Capabilities{Resources: []ResourceCap{}}
	for _, e := range entities {
		if len(e.API) == 0 {
			continue // not exposed over HTTP
		}
		rc := ResourceCap{Name: e.Name, Operations: map[string]OpCap{}, Fields: []FieldCap{}}
		for _, op := range e.API {
			rc.Operations[op.Op] = OpCap{Requires: op.Requires}
		}
		for _, f := range e.Fields {
			if !f.Exposed {
				continue
			}
			rc.Fields = append(rc.Fields, FieldCap{
				Name:       f.Name,
				Type:       string(f.Kind),
				Operators:  schema.FilterOperators(f),
				Sortable:   true,
				Selectable: true,
				Values:     f.Values,
				Lookup:     f.Lookup,
				JSONPaths:  f.JSONPaths,
			})
		}
		for _, r := range e.Relations {
			rc.Relations = append(rc.Relations, RelationCap{Name: r.Name, Target: r.Target})
		}
		caps.Resources = append(caps.Resources, rc)
	}
	return json.MarshalIndent(caps, "", "  ")
}

