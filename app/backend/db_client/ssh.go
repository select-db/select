package db_client

import (
	"fmt"

	"selectDb/backend/graph"
	"selectDb/backend/sqllang"

	"github.com/selectDb/dialect/engine"
)

// resolveSSHConfig substitutes .env variables in the SSH config fields.
func (dbc *DbClient) resolveSSHConfig(sshCfg *graph.DBInstanceSSHConfig, folderID string) (*engine.ResolvedSSHConfig, error) {
	if sshCfg == nil || !sshCfg.Enabled {
		return nil, nil
	}

	resolve := func(s string) (string, error) {
		if s == "" {
			return "", nil
		}
		return sqllang.SubstituteVariables(dbc.Graph, s, folderID)
	}

	host, err := resolve(sshCfg.Host)
	if err != nil {
		return nil, fmt.Errorf("resolve SSH host: %w", err)
	}
	user, err := resolve(sshCfg.User)
	if err != nil {
		return nil, fmt.Errorf("resolve SSH user: %w", err)
	}
	password, err := resolve(sshCfg.Password)
	if err != nil {
		return nil, fmt.Errorf("resolve SSH password: %w", err)
	}
	privateKey, err := resolve(sshCfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("resolve SSH private key: %w", err)
	}

	port := sshCfg.Port
	if port == 0 {
		port = 22
	}

	return &engine.ResolvedSSHConfig{
		Host:       host,
		Port:       port,
		User:       user,
		AuthMethod: sshCfg.AuthMethod,
		Password:   password,
		PrivateKey: privateKey,
		HostKey:    sshCfg.HostKey, // public key; no $var indirection
	}, nil
}
