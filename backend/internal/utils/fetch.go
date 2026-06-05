package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	green = "\033[32m"
	red   = "\033[31m"
	cyan  = "\033[36m"
	gray  = "\033[90m"

	httpClient = &http.Client{}
)

func Fetch(method, url string, body io.Reader, headers map[string]string, response interface{}) error {
	logEnabled := os.Getenv("LOG_HTTP") == "true" && os.Getenv("APP_ENV") == "dev"

	requestID := GenerateRequestID()
	timestamp := time.Now().Format("15:04:05")

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %w", err)
		}
	}

	if logEnabled {
		log.Printf("%s %-8s [%s] ──▶ %-6s %-30s", Colorize("[BACKEND] ▶▶▶ HTTP", cyan), timestamp, requestID, method, url)
		if len(bodyBytes) > 0 {
			LogIndentedJSONOrRaw(bodyBytes)
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		if logEnabled {
			log.Printf("%s %-8s [%s] ◀── %-6s %-30s [%sERROR%s] (%s)",
				Colorize("[BACKEND] ◀◀◀ HTTP", red), timestamp, requestID, method, url, red, gray, duration.Round(time.Millisecond))
		}
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	statusColor := green
	if resp.StatusCode >= 400 {
		statusColor = red
	}

	if logEnabled {
		log.Printf("%s %-8s [%s] ◀── %-6s %-30s [%s%d%s] (%s)",
			Colorize("[BACKEND] ◀◀◀ HTTP", cyan), timestamp, requestID, method, url, statusColor, resp.StatusCode, gray, duration.Round(time.Millisecond))
		if len(respBody) > 0 {
			LogIndentedJSONOrRaw(respBody)
		}
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if err := json.Unmarshal(respBody, response); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}

// Colorize adds ANSI color codes to a string
func Colorize(text, colorCode string) string {
	reset := "\033[0m"
	return fmt.Sprintf("%s%s%s", colorCode, text, reset)
}

// GenerateRequestID returns a random 8-character hex string
func GenerateRequestID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(b[:])
}

// LogIndentedJSONOrRaw prints JSON pretty or raw with indentation
func LogIndentedJSONOrRaw(data []byte) {
	var out bytes.Buffer
	if err := json.Indent(&out, data, "    ", "  "); err == nil {
		log.Println(out.String())
	} else {
		log.Println(indent(string(data)))
	}
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = "    " + lines[i]
	}
	return strings.Join(lines, "\n")
}
