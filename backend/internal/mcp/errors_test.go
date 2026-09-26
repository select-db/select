package mcp

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/selectDb/dialect/engine/connect"
)

func TestAsToolError_HidesUnknownErrors(t *testing.T) {
	te := asToolError(errors.New("dial tcp 10.0.0.5:5432: connection refused"))
	if te.Code != "internal" {
		t.Fatalf("code = %q, want internal", te.Code)
	}
	if strings.Contains(te.Message, "10.0.0.5") {
		t.Fatalf("message leaks the cause: %q", te.Message)
	}
	if te.Ref == "" {
		t.Fatal("want a ref to find the logged cause")
	}
}

func TestAsToolError_PassesToolErrorsThrough(t *testing.T) {
	want := errBadArgument("sql is required")
	if got := asToolError(fmt.Errorf("run: %w", want)); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAsToolError_ShowsConfigErrors(t *testing.T) {
	err := fmt.Errorf("open datasource: %w", &connect.ConfigError{Msg: "SSH tunneling is not supported for sqlite"})
	te := asToolError(err)
	if te.Code != "upstream" || te.Message != "SSH tunneling is not supported for sqlite" {
		t.Fatalf("got %+v", te)
	}
}
