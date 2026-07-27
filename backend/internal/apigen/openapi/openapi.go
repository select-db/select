// Package openapi projects the apigen entity model into an OpenAPI 3.0 document.
// Like the capabilities projection it is pure - no SQL, no sqlc, no runtime - so
// the spec is derived entirely from the schema IR (@app.api ops, field kinds,
// @app.values enums, and the convention columns).
//
// Endpoints are REST: a plural collection path (/roles) and an item path
// (/roles/{id}), each op mapped to its proper HTTP method - list=GET,
// create=POST, get=GET, update=PATCH (a partial merge), delete=DELETE. Auth is
// the shared bearer token (a user JWT or an slct_ API key in the Authorization
// header); per-op required workspace actions are surfaced in the operation
// description and as an x-required-actions extension. List filtering uses an
// OData $filter expression (and/or/not + grouping over the exposed fields).
package openapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"backend/internal/apigen/codegen"
	"backend/internal/apigen/schema"
)

// Version and base URL of the documented API. Both are assumptions a reviewer
// can adjust in one place; they do not affect any generated Go.
const (
	apiVersion   = "v1"
	apiServerURL = "https://api.select-db.com"
)

// --- OpenAPI 3.0 document (the minimal subset this projection emits) ---

type Document struct {
	OpenAPI    string               `json:"openapi"`
	Info       Info                 `json:"info"`
	Servers    []Server             `json:"servers,omitempty"`
	Tags       []Tag                `json:"tags,omitempty"`
	Security   []SecurityReq        `json:"security,omitempty"`
	Paths      map[string]*PathItem `json:"paths"`
	Components Components           `json:"components"`
}

type Info struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SecurityReq is one entry in a security list: scheme name -> required scopes
// (empty for our bearer token).
type SecurityReq map[string][]string

// PathItem holds the operations defined on one path, keyed by HTTP method.
type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
}

type Operation struct {
	Tags            []string            `json:"tags,omitempty"`
	Summary         string              `json:"summary,omitempty"`
	Description     string              `json:"description,omitempty"`
	OperationID     string              `json:"operationId,omitempty"`
	Parameters      []Parameter         `json:"parameters,omitempty"`
	RequestBody     *RequestBody        `json:"requestBody,omitempty"`
	Responses       map[string]Response `json:"responses"`
	RequiredActions []string            `json:"x-required-actions,omitempty"`
}

type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"` // "path" or "query"
	Required    bool    `json:"required,omitempty"`
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

type RequestBody struct {
	Required bool                 `json:"required,omitempty"`
	Content  map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

type Components struct {
	Schemas         map[string]*Schema        `json:"schemas,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Description  string `json:"description,omitempty"`
}

// Schema is the subset of JSON Schema OpenAPI 3.0 needs for this projection.
type Schema struct {
	Ref         string             `json:"$ref,omitempty"`
	Type        string             `json:"type,omitempty"`
	Format      string             `json:"format,omitempty"`
	Nullable    bool               `json:"nullable,omitempty"`
	ReadOnly    bool               `json:"readOnly,omitempty"`
	Enum        []string           `json:"enum,omitempty"`
	Description string             `json:"description,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
}

const bearerScheme = "bearerAuth"

// EmitOpenAPI renders the OpenAPI document for every API-exposed entity (those
// with at least one @app.api op). Returns indented JSON.
func EmitOpenAPI(entities []schema.Entity) ([]byte, error) {
	doc := Document{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:       "Select API",
			Version:     apiVersion,
			Description: "Workspace-scoped REST API generated from the Select schema. Authenticate with a user access token or an `slct_` API key in the `Authorization: Bearer <token>` header.",
		},
		Servers:  []Server{{URL: apiServerURL}},
		Security: []SecurityReq{{bearerScheme: {}}},
		Paths:    map[string]*PathItem{},
		Components: Components{
			Schemas: map[string]*Schema{"Error": errorSchema()},
			SecuritySchemes: map[string]SecurityScheme{
				bearerScheme: {
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "JWT or slct_ API key",
					Description:  "A workspace user access token or an `slct_`-prefixed API key.",
				},
			},
		},
	}

	for _, e := range entities {
		if len(e.API) == 0 {
			continue // not exposed over HTTP
		}
		model := codegen.Pascal(e.Name)
		coll := "/" + plural(e.Name)
		item := coll + "/{id}"
		doc.Tags = append(doc.Tags, Tag{Name: e.Name, Description: fmt.Sprintf("Operations on %s.", plural(e.Name))})

		// Component schemas: the response object plus create/update request bodies.
		doc.Components.Schemas[model] = responseSchema(e)
		if hasOp(e, "create") {
			doc.Components.Schemas[model+"CreateRequest"] = writeSchema(e, true)
		}
		if hasOp(e, "update") {
			doc.Components.Schemas[model+"UpdateRequest"] = writeSchema(e, false)
		}

		for _, op := range e.API {
			path, method := route(coll, item, op.Op)
			pi := doc.Paths[path]
			if pi == nil {
				pi = &PathItem{}
				doc.Paths[path] = pi
			}
			o := operationFor(e, model, op)
			switch method {
			case "get":
				pi.Get = o
			case "post":
				pi.Post = o
			case "patch":
				pi.Patch = o
			case "delete":
				pi.Delete = o
			}
		}
	}
	return json.MarshalIndent(doc, "", "  ")
}

// route maps an op to its (path, HTTP method): collection ops (list, create) on
// the plural path, item ops (get, update, delete) on the /{id} path.
func route(coll, item, op string) (path, method string) {
	switch op {
	case "list":
		return coll, "get"
	case "create":
		return coll, "post"
	case "get":
		return item, "get"
	case "update":
		return item, "patch"
	case "delete":
		return item, "delete"
	default:
		return coll, "post"
	}
}

// operationFor builds the operation for one op on one entity: its parameters
// (path id / list query), request body (create/update), and responses.
func operationFor(e schema.Entity, model string, op schema.APIOp) *Operation {
	o := &Operation{
		Tags:            []string{e.Name},
		Summary:         summary(op.Op, e.Name),
		OperationID:     op.Op + model,
		RequiredActions: op.Requires,
		Responses:       responsesFor(model, op.Op),
	}
	var desc []string
	if len(op.Requires) > 0 {
		desc = append(desc, "Requires workspace action(s): "+strings.Join(op.Requires, ", ")+".")
	}
	if op.Op == "list" {
		desc = append(desc, filterFieldsDoc(e))
	}
	if len(desc) > 0 {
		o.Description = strings.Join(desc, "\n\n")
	}
	switch op.Op {
	case "get", "update", "delete":
		o.Parameters = []Parameter{pathIDParam()}
	case "list":
		o.Parameters = listParams(e)
	}
	switch op.Op {
	case "create":
		o.RequestBody = jsonBodyRef(model + "CreateRequest")
	case "update":
		o.RequestBody = jsonBodyRef(model + "UpdateRequest")
	}
	return o
}

// responsesFor is the response set for an op: the resource (list -> array,
// create -> 201, delete -> 204), a 404 for item ops, plus shared auth errors.
func responsesFor(model, op string) map[string]Response {
	res := map[string]Response{
		"401": errorResponse("Missing or invalid authentication."),
		"403": errorResponse("Authenticated but lacks the required workspace action."),
	}
	entityRef := &Schema{Ref: "#/components/schemas/" + model}
	switch op {
	case "list":
		res["200"] = jsonResponse("A page of "+plural(strings.ToLower(model))+".", &Schema{Type: "array", Items: entityRef})
	case "create":
		res["201"] = jsonResponse("The created "+strings.ToLower(model)+".", entityRef)
	case "delete":
		res["204"] = Response{Description: "Deleted."}
		res["404"] = errorResponse("No such " + strings.ToLower(model) + " in this workspace.")
	default: // get, update
		res["200"] = jsonResponse("The "+strings.ToLower(model)+".", entityRef)
		res["404"] = errorResponse("No such " + strings.ToLower(model) + " in this workspace.")
	}
	return res
}

// responseSchema is the object returned for an entity: every exposed field, with
// the system-managed ones (id, cursor) marked read-only.
func responseSchema(e schema.Entity) *Schema {
	s := &Schema{Type: "object", Properties: map[string]*Schema{}}
	for _, f := range e.Fields {
		if !f.Exposed {
			continue
		}
		fs := fieldSchema(f)
		fs.Description = f.Description
		if f.IsPK || f.Column == schema.CursorColumn {
			fs.ReadOnly = true
		}
		s.Properties[f.Name] = fs
	}
	return s
}

// writeSchema is the create/update request body. create requires the client to
// supply id (offline-first client-generated UUIDs) plus every NOT NULL writable
// column without a default; update patches by id-in-path, so its body carries
// only the (optional) writable columns.
func writeSchema(e schema.Entity, create bool) *Schema {
	s := &Schema{Type: "object", Properties: map[string]*Schema{}}
	if create {
		s.Properties["id"] = &Schema{Type: "string", Format: "uuid"}
		s.Required = []string{"id"}
	}
	for _, f := range e.Fields {
		if !isWritable(f) {
			continue
		}
		s.Properties[f.Name] = fieldSchema(f)
		if create && !f.Nullable && f.Default == "" {
			s.Required = append(s.Required, f.Name)
		}
	}
	return s
}

// isWritable reports whether a field is client-settable: a patchable value column
// or a non-tenant foreign key (relationship identity set on write).
func isWritable(f schema.Field) bool {
	if f.Hidden || f.IsPK {
		return false
	}
	return f.Patchable || (f.FK != nil && f.Column != schema.TenantColumn)
}

// scalarSchema is a field's bare JSON type/format/enum, without nullability -
// the value shape used for filter operands.
func scalarSchema(f schema.Field) *Schema {
	s := &Schema{}
	switch f.Kind {
	case schema.KindUUID:
		s.Type, s.Format = "string", "uuid"
	case schema.KindTime:
		s.Type, s.Format = "string", "date-time"
	case schema.KindInt:
		s.Type = "integer"
	case schema.KindBool:
		s.Type = "boolean"
	case schema.KindJSON:
		s.Type = "object"
	case schema.KindInet:
		s.Type = "string"
	default: // text
		s.Type = "string"
	}
	if len(f.Values) > 0 {
		s.Enum = f.Values
	}
	return s
}

// fieldSchema is the scalar schema plus nullability, used for body/response
// properties.
func fieldSchema(f schema.Field) *Schema {
	s := scalarSchema(f)
	s.Nullable = f.Nullable
	return s
}

func pathIDParam() Parameter {
	return Parameter{
		Name:        "id",
		In:          "path",
		Required:    true,
		Description: "Resource id (UUID).",
		Schema:      &Schema{Type: "string", Format: "uuid"},
	}
}

// listParams is the query parameters for a list op: a single OData $filter
// expression (AND/OR/NOT + grouping over the filterable fields), plus sort and
// cursor pagination.
func listParams(e schema.Entity) []Parameter {
	return []Parameter{
		{Name: "$filter", In: "query", Description: filterDoc(e), Schema: &Schema{Type: "string"}},
		{Name: "sort", In: "query", Description: "Sort expression, e.g. \"-updated_at\".", Schema: &Schema{Type: "string"}},
		{Name: "limit", In: "query", Description: "Maximum rows to return.", Schema: &Schema{Type: "integer"}},
		{Name: "cursor", In: "query", Description: "Opaque pagination cursor from a previous page.", Schema: &Schema{Type: "string"}},
	}
}

// filterDoc documents the OData $filter grammar for one entity: the logical
// operators, the comparison/string/null operators, the filterable fields, and
// worked examples built from the entity's own columns.
func filterDoc(e schema.Entity) string {
	var b strings.Builder
	b.WriteString("OData $filter expression. Combine conditions with `and`, `or`, `not`, and parentheses. ")
	b.WriteString("Comparison: `eq`, `ne`, `gt`, `ge`, `lt`, `le`, `in`; string match: `contains(field,'x')`, `startswith(field,'x')`, `endswith(field,'x')`; null: `field eq null` / `field ne null`. ")
	b.WriteString("See the endpoint description for the filterable fields and their types.")
	if ex := filterExamples(e); len(ex) > 0 {
		b.WriteString(" Examples: " + strings.Join(ex, "; "))
	}
	return b.String()
}

// filterFieldsDoc is the markdown table of an entity's filterable fields - name,
// type (with enum values), and the OData operators that apply - rendered in the
// list endpoint description so a caller knows exactly what is queryable.
func filterFieldsDoc(e schema.Entity) string {
	var b strings.Builder
	b.WriteString("**Filterable fields**\n\n")
	b.WriteString("| Field | Type | Description | Operators |\n|---|---|---|---|\n")
	for _, f := range e.Fields {
		if !f.Exposed {
			continue
		}
		b.WriteString("| `" + f.Name + "` | " + typeLabel(f) + " | " + mdCell(f.Description) + " | " + strings.Join(odataOps(f), ", ") + " |\n")
	}
	return b.String()
}

// mdCell makes a string safe inside a one-line markdown table cell.
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "|", "\\|")
}

// typeLabel is a human type name for a field, with any @app.values enum inline.
func typeLabel(f schema.Field) string {
	var t string
	switch f.Kind {
	case schema.KindUUID:
		t = "uuid"
	case schema.KindTime:
		t = "date-time"
	case schema.KindInt:
		t = "integer"
	case schema.KindBool:
		t = "boolean"
	case schema.KindJSON:
		t = "json"
	case schema.KindInet:
		t = "ip"
	default:
		t = "string"
	}
	if len(f.Values) > 0 {
		t += " (" + strings.Join(f.Values, ", ") + ")"
	}
	return t
}

// odataOps maps a field's operator set (the shared FilterOperators taxonomy) to
// the OData tokens a caller writes. Null checks are omitted - they're universal
// and covered by the grammar legend (`field eq null` / `field ne null`).
func odataOps(f schema.Field) []string {
	var out []string
	seen := map[string]bool{}
	add := func(ss ...string) {
		for _, s := range ss {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	for _, op := range schema.FilterOperators(f) {
		switch op {
		case "eq":
			add("eq")
		case "ne":
			add("ne")
		case "lt":
			add("lt")
		case "lte":
			add("le")
		case "gt":
			add("gt")
		case "gte":
			add("ge")
		case "in":
			add("in")
		case "nin":
			add("not in")
		case "like", "ilike":
			add("contains", "startswith", "endswith")
		case "contains":
			add("contains")
		case "is_null", "not_null":
			// universal; covered by the grammar legend
		}
	}
	return out
}

// filterExamples builds one AND and one OR/NOT example from the entity's first
// couple of business (non-PK) filterable fields.
func filterExamples(e schema.Entity) []string {
	var fs []schema.Field
	for _, f := range e.Fields {
		if f.Exposed && !f.IsPK {
			fs = append(fs, f)
			if len(fs) == 2 {
				break
			}
		}
	}
	switch len(fs) {
	case 0:
		return nil
	case 1:
		lit := exampleLit(fs[0])
		return []string{
			fs[0].Name + " eq " + lit,
			"not (" + fs[0].Name + " eq " + lit + ")",
		}
	default:
		a, b := fs[0].Name+" eq "+exampleLit(fs[0]), fs[1].Name+" eq "+exampleLit(fs[1])
		return []string{
			a + " and " + b,
			"(" + a + " or " + b + ") and not (" + a + ")",
		}
	}
}

// exampleLit is a literal of the right OData shape for a field's kind: unquoted
// numbers/booleans/timestamps, quoted strings otherwise.
func exampleLit(f schema.Field) string {
	switch f.Kind {
	case schema.KindInt:
		return "100"
	case schema.KindBool:
		return "true"
	case schema.KindTime:
		return "2024-01-01T00:00:00Z"
	default: // text, uuid, inet, json
		return "'value'"
	}
}

func jsonBodyRef(name string) *RequestBody {
	return &RequestBody{Required: true, Content: map[string]MediaType{
		"application/json": {Schema: &Schema{Ref: "#/components/schemas/" + name}},
	}}
}

func errorSchema() *Schema {
	return &Schema{
		Type:       "object",
		Properties: map[string]*Schema{"error": {Type: "string", Description: "Human-readable error message."}},
		Required:   []string{"error"},
	}
}

func errorResponse(desc string) Response {
	return jsonResponse(desc, &Schema{Ref: "#/components/schemas/Error"})
}

func jsonResponse(desc string, s *Schema) Response {
	return Response{Description: desc, Content: map[string]MediaType{"application/json": {Schema: s}}}
}

func hasOp(e schema.Entity, op string) bool {
	for _, o := range e.API {
		if o.Op == op {
			return true
		}
	}
	return false
}

// summary is a human title for an op on a resource, e.g. list+role -> "List roles".
func summary(op, name string) string {
	switch op {
	case "list":
		return "List " + plural(name)
	case "get":
		return "Get " + name
	case "create":
		return "Create " + name
	case "update":
		return "Update " + name
	case "delete":
		return "Delete " + name
	default:
		return codegen.Pascal(op) + " " + name
	}
}

// plural is a naive pluralizer good enough for resource names (role -> roles,
// permission -> permissions, user_to_role -> user_to_roles).
func plural(name string) string {
	if strings.HasSuffix(name, "s") {
		return name
	}
	return name + "s"
}
