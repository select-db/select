// Package openapi projects the apigen entity model into an OpenAPI 3.0 document.
// Like the capabilities projection it is pure — no SQL, no sqlc, no runtime — so
// the spec is derived entirely from the schema IR (@app.api ops, field kinds,
// @app.values enums, and the convention columns).
//
// Endpoints follow the codebase's RPC-over-POST convention (POST /<resource>/<op>,
// matching /workspace/create, /apikey/list, …) rather than REST, so the generated
// API is one consistent surface with the hand-written endpoints. Auth is the
// shared bearer token (a user JWT or an slct_ API key in the Authorization
// header); per-op required workspace actions are surfaced in the operation
// description and as an x-required-actions extension.
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
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Servers    []Server            `json:"servers,omitempty"`
	Tags       []Tag               `json:"tags,omitempty"`
	Security   []SecurityReq       `json:"security,omitempty"`
	Paths      map[string]PathItem `json:"paths"`
	Components Components          `json:"components"`
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

type PathItem struct {
	Post *Operation `json:"post,omitempty"`
}

type Operation struct {
	Tags            []string            `json:"tags,omitempty"`
	Summary         string              `json:"summary,omitempty"`
	Description     string              `json:"description,omitempty"`
	OperationID     string              `json:"operationId,omitempty"`
	RequestBody     *RequestBody        `json:"requestBody,omitempty"`
	Responses       map[string]Response `json:"responses"`
	RequiredActions []string            `json:"x-required-actions,omitempty"`
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
			Description: "Workspace-scoped REST-over-POST API generated from the Select schema. Authenticate with a user access token or an `slct_` API key in the `Authorization: Bearer <token>` header.",
		},
		Servers:  []Server{{URL: apiServerURL}},
		Security: []SecurityReq{{bearerScheme: {}}},
		Paths:    map[string]PathItem{},
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
			path := "/" + e.Name + "/" + op.Op
			doc.Paths[path] = PathItem{Post: operationFor(e, model, op)}
		}
	}
	return json.MarshalIndent(doc, "", "  ")
}

// operationFor builds the POST operation for one op on one entity.
func operationFor(e schema.Entity, model string, op schema.APIOp) *Operation {
	o := &Operation{
		Tags:            []string{e.Name},
		Summary:         summary(op.Op, e.Name),
		OperationID:     op.Op + model,
		RequiredActions: op.Requires,
		Responses:       responsesFor(model, op.Op),
	}
	if len(op.Requires) > 0 {
		o.Description = "Requires workspace action(s): " + strings.Join(op.Requires, ", ") + "."
	}
	if body := requestBodyFor(model, op.Op); body != nil {
		o.RequestBody = body
	}
	return o
}

// requestBodyFor is the request schema for an op: an id selector for get/delete,
// the create/update request component for those, and a list-query body for list.
func requestBodyFor(model, op string) *RequestBody {
	ref := func(name string) *RequestBody {
		return &RequestBody{Required: true, Content: map[string]MediaType{
			"application/json": {Schema: &Schema{Ref: "#/components/schemas/" + name}},
		}}
	}
	inline := func(s *Schema) *RequestBody {
		return &RequestBody{Required: true, Content: map[string]MediaType{"application/json": {Schema: s}}}
	}
	switch op {
	case "get", "delete":
		return inline(idSelector())
	case "create":
		return ref(model + "CreateRequest")
	case "update":
		return ref(model + "UpdateRequest")
	case "list":
		return inline(listRequestSchema())
	default:
		return nil
	}
}

// responsesFor is the response set for an op: the resource (list -> array),
// plus the shared auth/error responses.
func responsesFor(model, op string) map[string]Response {
	res := map[string]Response{
		"401": errorResponse("Missing or invalid authentication."),
		"403": errorResponse("Authenticated but lacks the required workspace action."),
	}
	entityRef := &Schema{Ref: "#/components/schemas/" + model}
	switch op {
	case "list":
		res["200"] = jsonResponse("A page of "+plural(strings.ToLower(model))+".", &Schema{Type: "array", Items: entityRef})
	case "delete":
		res["204"] = Response{Description: "Deleted."}
	default: // get, create, update
		res["200"] = jsonResponse("The "+strings.ToLower(model)+".", entityRef)
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
		if f.IsPK || f.Column == schema.CursorColumn {
			fs.ReadOnly = true
		}
		s.Properties[f.Name] = fs
	}
	return s
}

// writeSchema is the create/update request body. create requires the client to
// supply id (offline-first client-generated UUIDs) plus every NOT NULL writable
// column without a default; update patches, so only id is required.
func writeSchema(e schema.Entity, create bool) *Schema {
	s := &Schema{Type: "object", Properties: map[string]*Schema{"id": {Type: "string", Format: "uuid"}}, Required: []string{"id"}}
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

// fieldSchema maps one field's kind (+ nullability, enum) to a JSON schema.
func fieldSchema(f schema.Field) *Schema {
	s := &Schema{Nullable: f.Nullable}
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

func idSelector() *Schema {
	return &Schema{
		Type:       "object",
		Properties: map[string]*Schema{"id": {Type: "string", Format: "uuid"}},
		Required:   []string{"id"},
	}
}

// listRequestSchema is a provisional filter/sort/pagination body; it firms up
// once the list handler's query grammar lands.
func listRequestSchema() *Schema {
	return &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"filter": {Type: "object", Description: "Field filters keyed by column, each an operator->value object."},
			"sort":   {Type: "string", Description: "Sort expression, e.g. \"-updated_at\"."},
			"limit":  {Type: "integer", Description: "Maximum rows to return."},
			"cursor": {Type: "string", Description: "Opaque pagination cursor from a previous page."},
		},
	}
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
