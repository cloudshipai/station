package ssh

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/tui"
)

type Server struct {
	cfg           *config.Config
	db            *db.DB
	repos         *repositories.Repositories
	genkitService services.AgentServiceInterface
	localMode     bool
	srv           *ssh.Server
}

func New(cfg *config.Config, database *db.DB, repos *repositories.Repositories, genkitService services.AgentServiceInterface, localMode bool) *Server {
	s := &Server{
		cfg:           cfg,
		db:            database,
		repos:         repos,
		genkitService: genkitService,
		localMode:     localMode,
	}

	s.srv = s.createSSHServer()
	return s
}

func (s *Server) createSSHServer() *ssh.Server {
	var options []ssh.Option

	// Basic server options
	options = append(options,
		wish.WithAddress(fmt.Sprintf(":%d", s.cfg.SSHPort)),
		wish.WithHostKeyPath(s.cfg.SSHHostKeyPath),
	)

	// Authentication - different for local vs remote mode
	if s.localMode {
		// Local mode: Allow any authentication (single-user development)
		options = append(options,
			wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
				log.Printf("SSH: Local mode - accepting any public key for user: %s", ctx.User())
				return true
			}),
			wish.WithPasswordAuth(func(ctx ssh.Context, password string) bool {
				log.Printf("SSH: Local mode - accepting any password for user: %s", ctx.User())
				return true
			}),
		)
	} else {
		// Remote mode: System user authentication
		options = append(options,
			wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
				return s.authenticateSystemUserKey(ctx.User(), key)
			}),
			wish.WithPasswordAuth(func(ctx ssh.Context, password string) bool {
				return s.authenticateSystemUserPassword(ctx.User(), password)
			}),
		)
	}

	// Middleware
	options = append(options,
		wish.WithMiddleware(
			bubbletea.Middleware(s.teaHandler),
			logging.Middleware(),
		),
	)

	srv, err := wish.NewServer(options...)
	if err != nil {
		log.Fatal("Failed to create SSH server:", err)
	}
	return srv
}

func (s *Server) teaHandler(session ssh.Session) (tea.Model, []tea.ProgramOption) {
	// Check if chat mode is requested via environment variable or user command
	// For now, we'll default to the new chat interface
	// TODO: Add configuration option to choose between chat and traditional TUI

	// Create the new chat TUI model
	chatModel := tui.NewChatModel(s.db, s.repos, s.genkitService)

	return chatModel, []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	log.Printf("Starting SSH server on port %d", s.cfg.SSHPort)

	done := make(chan error, 1)
	go func() {
		done <- s.srv.ListenAndServe()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		log.Println("Shutting down SSH server...")
		// Very aggressive timeout - 1s for SSH shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// Start shutdown immediately
		done := make(chan error, 1)
		go func() {
			done <- s.srv.Shutdown(shutdownCtx)
		}()

		// Wait for shutdown or force close
		select {
		case err := <-done:
			log.Println("SSH server stopped gracefully")
			return err
		case <-shutdownCtx.Done():
			log.Println("SSH server shutdown timeout - forcing close")
			// Force immediate close
			return s.srv.Close()
		}
	}
}

// authenticateSystemUserKey validates SSH public key against system authorized_keys
func (s *Server) authenticateSystemUserKey(username string, key ssh.PublicKey) bool {
	log.Printf("SSH: Remote mode - validating public key for system user: %s", username)

	// Get system user
	systemUser, err := user.Lookup(username)
	if err != nil {
		log.Printf("SSH: System user %s not found: %v", username, err)
		return false
	}

	// Read user's authorized_keys file
	authorizedKeysPath := fmt.Sprintf("%s/.ssh/authorized_keys", systemUser.HomeDir)
	authorizedKeysData, err := os.ReadFile(authorizedKeysPath)
	if err != nil {
		log.Printf("SSH: Could not read authorized_keys for %s: %v", username, err)
		return false
	}

	// Parse and check each key
	for _, line := range strings.Split(string(authorizedKeysData), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		authorizedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			continue // Skip invalid keys
		}

		if ssh.KeysEqual(key, authorizedKey) {
			log.Printf("SSH: Public key authenticated for system user: %s", username)
			return true
		}
	}

	log.Printf("SSH: Public key not found in authorized_keys for user: %s", username)
	return false
}

// authenticateSystemUserPassword validates password against system authentication
func (s *Server) authenticateSystemUserPassword(username string, password string) bool {
	log.Printf("SSH: Remote mode - password authentication not supported for security reasons")
	log.Printf("SSH: Use public key authentication for system user: %s", username)

	// For security, we don't support password authentication in remote mode
	// System users should use SSH key authentication only
	return false
}
