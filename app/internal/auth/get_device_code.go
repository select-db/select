package auth

import (
	"context"
	"selectDb/internal/api"
)

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func (ga *GithubAuth) GetDeviceCode() (*DeviceCodeResponse, error) {
	var deviceCodeResponse DeviceCodeResponse
	err := api.Fetch(
		context.Background(),
		"GET",
		"auth/get-device-code",
		nil,
		nil,
		&deviceCodeResponse,
	)

	if err != nil {
		return nil, err
	}

	return &deviceCodeResponse, nil
}
