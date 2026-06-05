package toolkit

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
)

const cyan = "\033[36m"
const yellow = "\033[33m"
const reset = "\033[0m"

// getCallerInfo returns the file name and line number of the caller
func getCallerInfo(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", 0
	}
	return filepath.Base(file), line
}

// PrintJson prints any value as colored JSON with file and line info
func PrintJson(v any) {
	file, line := getCallerInfo(2)
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Printf("%s[%s:%d]%s [PrintJson] Failed to MarshalIndent value: %v\n", yellow, file, line, reset, err)
		return
	}
	re := regexp.MustCompile(`"([^"]+)"\s*:`)
	colored := re.ReplaceAllStringFunc(string(b), func(match string) string {
		return cyan + match[:len(match)-1] + reset + ":"
	})
	log.Printf("%s[%s:%d]%s\n%s\n", yellow, file, line, reset, colored)
}

// Print takes any value and prints it in a readable format with file and line info
func Print(v interface{}) {
	file, line := getCallerInfo(2)
	prefix := fmt.Sprintf("%s[%s:%d]%s ", yellow, file, line, reset)

	if v == nil {
		fmt.Printf("%s<nil>\n", prefix)
		return
	}

	// Special handling for error types
	if err, ok := v.(error); ok {
		fmt.Printf("%serror: %v\n", prefix, err)
		return
	}

	// Get the type and value
	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	switch val.Kind() {
	case reflect.String:
		fmt.Printf("%s%q\n", prefix, v)
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		// For complex types, use the colored JSON function
		// Pass skip=3 since we're calling from Print
		file, line := getCallerInfo(2)
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			log.Printf("%s[PrintJson] Failed to MarshalIndent value: %v\n", prefix, err)
			return
		}
		re := regexp.MustCompile(`"([^"]+)"\s*:`)
		colored := re.ReplaceAllStringFunc(string(b), func(match string) string {
			return cyan + match[:len(match)-1] + reset + ":"
		})
		fmt.Printf("%s[%s:%d]%s\n%s\n", yellow, file, line, reset, colored)
	case reflect.Ptr:
		if val.IsNil() {
			fmt.Printf("%s%s: <nil>\n", prefix, typ.String())
		} else {
			fmt.Printf("%s%s -> ", prefix, typ.String())
			// Don't add prefix for the nested call
			printWithoutPrefix(val.Elem().Interface())
		}
	default:
		fmt.Printf("%s%s: %v\n", prefix, typ.String(), v)
	}
}

// printWithoutPrefix is a helper to print nested pointer values without duplicating the prefix
func printWithoutPrefix(v interface{}) {
	if v == nil {
		fmt.Println("<nil>")
		return
	}

	if err, ok := v.(error); ok {
		fmt.Printf("error: %v\n", err)
		return
	}

	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	switch val.Kind() {
	case reflect.String:
		fmt.Printf("%q\n", v)
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		b, _ := json.MarshalIndent(v, "", "  ")
		re := regexp.MustCompile(`"([^"]+)"\s*:`)
		colored := re.ReplaceAllStringFunc(string(b), func(match string) string {
			return cyan + match[:len(match)-1] + reset + ":"
		})
		fmt.Printf("\n%s\n", colored)
	case reflect.Ptr:
		if val.IsNil() {
			fmt.Printf("%s: <nil>\n", typ.String())
		} else {
			fmt.Printf("%s -> ", typ.String())
			printWithoutPrefix(val.Elem().Interface())
		}
	default:
		fmt.Printf("%s: %v\n", typ.String(), v)
	}
}
