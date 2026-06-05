// Package mcp exposes tools over streamable HTTP (JSON-RPC 2.0).
// API-key only; the key's workspace and role are the sole authority.
// See README.md for auth model, tool catalog, and design decisions.
package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"backend/internal/api/middlewares"
)

// Bump when tool schemas change incompatibly; clients pin this in `initialize`.
const protocolVersion = "2024-11-05"

const serverName = "selectdb-mcp"
const serverVersion = "0.1.0"

// jsonrpcRequest is the on-the-wire request envelope.
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonrpcResponse is the response envelope; exactly one of Result/Error is set.
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// Handler runs behind Authenticated middleware; rejects non-API-key principals.
func Handler() http.HandlerFunc {
	registry := defaultTools()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !middlewares.IsAPIKeyPrincipal(r) {
			http.Error(w, "MCP requires an API key", http.StatusForbidden)
			return
		}
		workspaceID := middlewares.MemberWorkspaceID(r)

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeRPCError(w, nil, codeParseError, "could not read request body")
			return
		}

		var req jsonrpcRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeRPCError(w, nil, codeParseError, "invalid JSON")
			return
		}
		if req.JSONRPC != "2.0" {
			writeRPCError(w, req.ID, codeInvalidRequest, `jsonrpc must be "2.0"`)
			return
		}

		switch req.Method {
		case "initialize":
			handleInitialize(w, req)
		case "notifications/initialized":
			// MCP clients send this after initialize; no response expected.
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			handleToolsList(w, req, registry)
		case "tools/call":
			handleToolsCall(w, r, req, registry, workspaceID)
		default:
			writeRPCError(w, req.ID, codeMethodNotFound, "method not found: "+req.Method)
		}
	}
}

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      map[string]any `json:"serverInfo"`
}

func handleInitialize(w http.ResponseWriter, req jsonrpcRequest) {
	writeRPC(w, req.ID, initializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities: map[string]any{
			"tools": map[string]any{},
		},
		ServerInfo: map[string]any{
			"name":    serverName,
			"version": serverVersion,
		},
	})
}

func handleToolsList(w http.ResponseWriter, req jsonrpcRequest, registry []Tool) {
	defs := make([]toolDef, 0, len(registry))
	for _, t := range registry {
		defs = append(defs, toolDef{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
			Annotations: t.Annotations,
		})
	}
	writeRPC(w, req.ID, map[string]any{"tools": defs})
}

func handleToolsCall(w http.ResponseWriter, r *http.Request, req jsonrpcRequest, registry []Tool, workspaceID string) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		writeRPCError(w, req.ID, codeInvalidParams, "invalid params")
		return
	}
	for _, t := range registry {
		if t.Name != p.Name {
			continue
		}
		result, err := t.Run(r.Context(), r, workspaceID, p.Arguments)
		if err != nil {
			// MCP convention: tool errors are surfaced inside the result
			// envelope (isError: true) so the model can react, not as
			// JSON-RPC errors (which clients tend to treat as fatal).
			writeRPC(w, req.ID, toolErrorResult(err))
			return
		}
		writeRPC(w, req.ID, toolOKResult(result))
		return
	}
	writeRPCError(w, req.ID, codeMethodNotFound, "no such tool: "+p.Name)
}

// MCP wraps tool output in a content-blocks array. We only emit one
// JSON text block per call; the model handles the JSON itself.
func toolOKResult(payload any) map[string]any {
	body, err := json.Marshal(payload)
	if err != nil {
		body = []byte(fmt.Sprintf(`{"error":{"code":"marshal","message":%q}}`, err.Error()))
	}
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(body)},
		},
		"isError": false,
	}
}

func toolErrorResult(err error) map[string]any {
	te := asToolError(err)
	body, _ := json.Marshal(map[string]any{"error": te})
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(body)},
		},
		"isError": true,
	}
}

func writeRPC(w http.ResponseWriter, id json.RawMessage, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func writeRPCError(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &jsonrpcError{
			Code:    code,
			Message: msg,
		},
	})
}

type toolDef struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	InputSchema any              `json:"inputSchema"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}
