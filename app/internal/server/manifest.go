package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const manifestPath = "/version"

var manifestClient = &http.Client{Timeout: 8 * time.Second}

// Manifest holds the result of fetching a backend server's version endpoint.
// Returned by GetServerManifest for the frontend to show server status and version warnings.
type Manifest struct {
	Live           bool   `json:"live"`
	BackendVersion string `json:"backend_version"`
	MinAppVersion  string `json:"min_app_version"`
	LatestAppVersion string `json:"latest_app_version"`
	Warning        string `json:"warning"`
}

type versionResponse struct {
	Backend          string `json:"backend"`
	MinAppVersion    string `json:"min_app_version"`
	LatestAppVersion string `json:"latest_app_version"`
}

// GetServerManifest fetches the manifest (version) from the given server domain.
// If the server is unreachable, Live is false and other fields are empty.
// If the server's backend version is older than the desktop app version, Warning is set.
func (s *Server) GetServerManifest(domain string) (*Manifest, error) {
	domain = trimDomain(domain)
	if domain == "" {
		return &Manifest{Live: false}, nil
	}
	baseURL := DomainToBaseURL(domain)
	url := baseURL + manifestPath
	resp, err := manifestClient.Get(url)
	if err != nil {
		return &Manifest{Live: false}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return &Manifest{Live: false}, nil
	}
	var ver versionResponse
	if err := json.NewDecoder(resp.Body).Decode(&ver); err != nil {
		return &Manifest{Live: false}, nil
	}
	m := &Manifest{
		Live:             true,
		BackendVersion:  ver.Backend,
		MinAppVersion:   ver.MinAppVersion,
		LatestAppVersion: ver.LatestAppVersion,
	}
	appVersion := os.Getenv("APP_VERSION")
	if appVersion != "" && appVersion != "dev" && ver.Backend != "" && semverLess(ver.Backend, appVersion) {
		m.Warning = "Server version is older than this app. Some features may not work correctly."
	}
	return m, nil
}

func semverLess(a, b string) bool {
	return parseSemver(a) < parseSemver(b)
}

func parseSemver(v string) int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])
	return major*1_000_000 + minor*1_000 + patch
}
