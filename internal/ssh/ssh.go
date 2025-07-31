package ssh

import (
	"context"
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"

	"station/internal/config"
	"station/internal/db"
	"station/internal/services"
	"station/internal/tui"
)

type Server struct {
	cfg            *config.Config
	db             *db.DB
	executionQueue *services.ExecutionQueueService
	srv            *ssh.Server
}

func New(cfg *config.Config, database *db.DB, executionQueue *services.ExecutionQueueService) *Server {
	s := &Server{
		cfg:            cfg,
		db:             database,
		executionQueue: executionQueue,
	}

	s.srv = s.createSSHServer()
	return s
}

func (s *Server) createSSHServer() *ssh.Server {
	srv, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf(":%d", s.cfg.SSHPort)),
		wish.WithHostKeyPath(s.cfg.SSHHostKeyPath),
		wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
			// Allow any public key for now (development only)
			return true
		}),
		wish.WithPasswordAuth(func(ctx ssh.Context, password string) bool {
			// Allow any password for now (development only)
			return true
		}),
		wish.WithMiddleware(
			bubbletea.Middleware(s.teaHandler),
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Fatal("Failed to create SSH server:", err)
	}
	return srv
}

func (s *Server) teaHandler(session ssh.Session) (tea.Model, []tea.ProgramOption) {
	// Create the new TUI model with database access and execution queue
	tuiModel := tui.NewModel(s.db, s.executionQueue)
	
	return tuiModel, []tea.ProgramOption{
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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return s.srv.Shutdown(shutdownCtx)
	}
}