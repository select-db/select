package datasource

import (
	"errors"
	"os"
)

const secretKeyEnv = "SELECTDB_SECRET_KEY"

func secretKey() (string, error) {
	key := os.Getenv(secretKeyEnv)
	if key == "" {
		return "", errors.New(secretKeyEnv + " is not set")
	}
	return key, nil
}
