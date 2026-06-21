package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/internal/middlewares"
)

// apiKeyRequest builds an httptest request whose context looks like the
// Authenticated() middleware ran with an API key principal bound to
// `workspaceID`. Skips the actual auth layer; protocol-level tests only.
func apiKeyRequest(t *testing.T, body string, workspaceID string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	ctx := middlewares.ContextWithAPIKeyPrincipal(r.Context(), "principal-1", "Principal One", workspaceID, nil, nil)
	return r.WithContext(ctx)
}

// jwtRequest pretends a JWT user authenticated: IsAPIKeyPrincipal returns false.
func jwtRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestHandler_RejectsNonPost(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	Handler()(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", w.Code)
	}
}

func TestHandler_RejectsNonAPIKeyPrincipal(t *testing.T) {
	w := httptest.NewRecorder()
	r := jwtRequest(t, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	Handler()(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 for non-API-key, got %d", w.Code)
	}
}

func TestHandler_Initialize(t *testing.T) {
	w := httptest.NewRecorder()
	r := apiKeyRequest(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		"ws-1")
	Handler()(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp jsonrpcResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	res, _ := resp.Result.(map[string]any)
	if res["protocolVersion"] != protocolVersion {
		t.Fatalf("want protocolVersion=%q got %v", protocolVersion, res["protocolVersion"])
	}
}

func TestHandler_ToolsList(t *testing.T) {
	w := httptest.NewRecorder()
	r := apiKeyRequest(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, "ws-1")
	Handler()(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp jsonrpcResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	res, _ := resp.Result.(map[string]any)
	tools, _ := res["tools"].([]any)
	if len(tools) != 7 {
		t.Fatalf("want 7 tools, got %d", len(tools))
	}
	wantSet := map[string]bool{
		"list_datasources":     true,
		"get_database_schemas": true, "get_database_table_detail": true,
		"execute_query": true, "execute_statement": true,
		"explain_query": true, "plan_query": true,
	}
	seen := map[string]bool{}
	for _, ti := range tools {
		td, _ := ti.(map[string]any)
		name, _ := td["name"].(string)
		if seen[name] {
			t.Fatalf("duplicate tool %q", name)
		}
		seen[name] = true
		if !wantSet[name] {
			t.Fatalf("unexpected tool %q", name)
		}
	}
}

func TestHandler_ToolsListEmitsAnnotations(t *testing.T) {
	w := httptest.NewRecorder()
	r := apiKeyRequest(t, `{"jsonrpc":"2.0","id":7,"method":"tools/list"}`, "ws-1")
	Handler()(w, r)

	var resp jsonrpcResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	res, _ := resp.Result.(map[string]any)
	tools, _ := res["tools"].([]any)

	byName := map[string]map[string]any{}
	for _, ti := range tools {
		td, _ := ti.(map[string]any)
		name, _ := td["name"].(string)
		byName[name] = td
	}

	cases := []struct {
		name                string
		wantReadOnly        bool
		wantDestructive     bool
		annotationsExpected bool
	}{
		{"list_datasources", true, false, true},
		{"get_database_schemas", true, false, true},
		{"get_database_table_detail", true, false, true},
		{"execute_query", true, false, true},
		{"plan_query", true, false, true},
		{"explain_query", true, false, true},
		{"execute_statement", false, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td, ok := byName[tc.name]
			if !ok {
				t.Fatalf("tool %q missing from tools/list", tc.name)
			}
			ann, _ := td["annotations"].(map[string]any)
			if tc.annotationsExpected && ann == nil {
				t.Fatalf("tool %q has no annotations", tc.name)
			}
			if got, ok := ann["readOnlyHint"].(bool); ok && got != tc.wantReadOnly {
				t.Fatalf("tool %q readOnlyHint: want %v got %v", tc.name, tc.wantReadOnly, got)
			}
			if got, ok := ann["destructiveHint"].(bool); ok && got != tc.wantDestructive {
				t.Fatalf("tool %q destructiveHint: want %v got %v", tc.name, tc.wantDestructive, got)
			}
		})
	}
}

func TestHandler_RejectsBadJSON(t *testing.T) {
	w := httptest.NewRecorder()
	r := apiKeyRequest(t, `{not json`, "ws-1")
	Handler()(w, r)
	var resp jsonrpcResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error == nil || resp.Error.Code != codeParseError {
		t.Fatalf("want parse error, got %+v", resp.Error)
	}
}

func TestHandler_UnknownMethod(t *testing.T) {
	w := httptest.NewRecorder()
	r := apiKeyRequest(t, `{"jsonrpc":"2.0","id":3,"method":"resources/list"}`, "ws-1")
	Handler()(w, r)
	var resp jsonrpcResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error == nil || resp.Error.Code != codeMethodNotFound {
		t.Fatalf("want method-not-found, got %+v", resp.Error)
	}
}
