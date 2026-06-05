package mcp

import "errors"

// toolError is the structured envelope returned inside an MCP tool
// response when something went wrong. Stable shape so models learn it.
type toolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
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

func errUpstream(msg string) *toolError {
	return &toolError{Code: "upstream", Message: msg}
}

// asToolError coerces any error to the wire shape. Wrapped *toolError
// passes through unchanged; everything else is reported as internal.
func asToolError(err error) *toolError {
	var te *toolError
	if errors.As(err, &te) {
		return te
	}
	return &toolError{Code: "internal", Message: err.Error()}
}
