package mcp

import (
	"errors"
	"fmt"
	"net/http"

	"backend/internal/datasource/managed"
	"backend/internal/utils"

	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/selectDb/dialect/engine/connect"
)

// toolError is the structured envelope returned inside an MCP tool
// response when something went wrong. Stable shape so models learn it.
type toolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

// Error implements error so handlers can return *toolError directly.
func (e *toolError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func errBadArgument(msg string) *toolError {
	return &toolError{Code: "bad_argument", Message: msg}
}

func errNotFound(msg string) *toolError {
	return &toolError{Code: "not_found", Message: msg}
}

func errExecution(msg string, hint string) *toolError {
	return &toolError{Code: "execution_failed", Message: msg, Hint: hint}
}

// asToolError coerces any error to the wire shape. A *toolError or an engine
// ConfigError is written for the caller; anything else may carry driver or
// network detail, so the caller gets a ref and the detail goes to the log.
func asToolError(err error) *toolError {
	var te *toolError
	if errors.As(err, &te) {
		return te
	}
	var refused *managed.Refusal
	if errors.As(err, &refused) {
		switch refused.Status {
		case http.StatusForbidden:
			return &toolError{Code: "forbidden", Message: refused.Message}
		case http.StatusNotFound:
			return errNotFound(refused.Message)
		}
		return errBadArgument(refused.Message)
	}
	var cfgErr *connect.ConfigError
	if errors.As(err, &cfgErr) {
		return &toolError{Code: "upstream", Message: cfgErr.Msg}
	}
	// A managed database's failure is already classified and safe to show.
	var coded *arrowstream.Error
	if errors.As(err, &coded) && coded.Code != "" {
		return &toolError{Code: coded.Code, Message: coded.Message}
	}
	ref := utils.LogWithRef(fmt.Sprintf("mcp: internal error: %v", err))
	return &toolError{Code: "internal", Message: "internal error", Ref: ref}
}
