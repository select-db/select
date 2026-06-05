package server

import (
	"os"
	"path/filepath"
)

// Server provides server selection and management (domain-based, no meta DB).
// DB operations are injected to avoid import cycles.
type Server struct {
	setRootFunc     func(root string)
	runMigrationsAt func(dbPath string) error
	switchToServer  func(domain string) error
}

// New returns a new Server. Callbacks are invoked when the current server is set or switched.
func New(
	setRootFunc func(root string),
	runMigrationsAt func(dbPath string) error,
	switchToServer func(domain string) error,
) *Server {
	return &Server{
		setRootFunc:     setRootFunc,
		runMigrationsAt: runMigrationsAt,
		switchToServer:  switchToServer,
	}
}

// ListServers returns the list of server domains (folders that contain a db.sqlite).
func (s *Server) ListServers() ([]string, error) {
	return ListServerDomains()
}

// GetCurrentServer returns the current server domain, or empty if none.
func (s *Server) GetCurrentServer() (string, error) {
	return ReadCurrentDomain()
}

// GetDefaultServer returns the default server domain for the current APP_ENV, or empty if none.
func (s *Server) GetDefaultServer() (string, error) {
	return DefaultDomainForEnv(), nil
}

// SetCurrentServer sets the current server to the given domain, opens its DB, and updates root.
func (s *Server) SetCurrentServer(domain string) error {
	domain = trimDomain(domain)
	if domain == "" {
		return nil
	}
	serverDir, err := ServerRootPath(domain)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(serverDir, 0o700); err != nil {
		return err
	}
	dbPath := filepath.Join(serverDir, dbFilename)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) && s.runMigrationsAt != nil {
		if err := s.runMigrationsAt(dbPath); err != nil {
			return err
		}
	}
	if s.switchToServer != nil {
		if err := s.switchToServer(domain); err != nil {
			return err
		}
	}
	if err := WriteCurrentDomain(domain); err != nil {
		return err
	}
	if s.setRootFunc != nil {
		s.setRootFunc(serverDir)
	}
	return nil
}

// CreateServer creates a new server folder and its DB for the given domain.
func (s *Server) CreateServer(domain string) error {
	domain = trimDomain(domain)
	if domain == "" {
		return nil
	}
	serverDir, err := ServerRootPath(domain)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(serverDir, 0o700); err != nil {
		return err
	}
	dbPath := filepath.Join(serverDir, dbFilename)
	if s.runMigrationsAt == nil {
		return nil
	}
	return s.runMigrationsAt(dbPath)
}

// RemoveServer removes the server folder for the given domain.
// If it was the current server, current is cleared or set to another existing server.
func (s *Server) RemoveServer(domain string) error {
	domain = trimDomain(domain)
	if domain == "" {
		return nil
	}
	current, err := ReadCurrentDomain()
	if err != nil {
		return err
	}
	serverDir, err := ServerRootPath(domain)
	if err != nil {
		return err
	}
	if current == domain {
		others, err := ListServerDomains()
		if err != nil {
			return err
		}
		var next string
		for _, d := range others {
			if d != domain {
				next = d
				break
			}
		}
		if s.switchToServer != nil {
			if err := s.switchToServer(next); err != nil {
				return err
			}
		}
		if err := WriteCurrentDomain(next); err != nil {
			return err
		}
		if s.setRootFunc != nil {
			if next != "" {
				nextRoot, err := ServerRootPath(next)
				if err != nil {
					return err
				}
				s.setRootFunc(nextRoot)
			} else {
				s.setRootFunc("")
			}
		}
	}
	return os.RemoveAll(serverDir)
}

func trimDomain(domain string) string {
	const prefix = "https://"
	const prefix2 = "http://"
	if len(domain) > len(prefix) && domain[:len(prefix)] == prefix {
		domain = domain[len(prefix):]
	}
	if len(domain) > len(prefix2) && domain[:len(prefix2)] == prefix2 {
		domain = domain[len(prefix2):]
	}
	return domain
}
