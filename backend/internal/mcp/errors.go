package mcp

import (
	"errors"
	"log"

	"backend/internal/datasource/cellar"
	"backend/internal/utils"

	"github.com/selectDb/dialect/engine"
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
	var cfgErr *engine.ConfigError
	if errors.As(err, &cfgErr) {
		return &toolError{Code: "upstream", Message: cfgErr.Msg}
	}
	if errors.Is(err, cellar.ErrOff) {
		return &toolError{Code: "disabled", Message: cellar.ErrOff.Error()}
	}
	if errors.Is(err, cellar.ErrUnavailable) {
		return &toolError{Code: "unavailable", Message: cellar.ErrUnavailable.Error()}
	}
	ref := utils.GenerateRequestID()
	log.Printf("mcp: internal error ref=%s: %v", ref, err)
	return &toolError{Code: "internal", Message: "internal error", Ref: ref}
}
