package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"backend/internal/utils"
)

func GetDeviceCodeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		deviceCodeResp, err := GetDeviceCode()
		if err != nil {
			log.Printf("auth: get device code: %v", err)
			http.Error(w, "authentication is temporarily unavailable", http.StatusInternalServerError)
			return
		}

		// Remember the code so only ours can be polled
		rememberDeviceCode(deviceCodeResp.DeviceCode)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deviceCodeResp)
	}
}

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func GetDeviceCode() (*DeviceCodeResponse, error) {
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	if clientID == "" {
		return nil, fmt.Errorf("GITHUB_CLIENT_ID is not set")
	}

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", "read:user user:email")

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Accept":       "application/json",
	}

	var resp DeviceCodeResponse
	if err := utils.Fetch("POST", "https://github.com/login/device/code", strings.NewReader(data.Encode()), headers, &resp); err != nil {
		return nil, fmt.Errorf("failed to get device code: %w", err)
	}

	return &resp, nil
}
