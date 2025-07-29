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
	"station/internal/db/repositories"
	"station/internal/ssh/apps"
)

type Server struct {
	cfg   *config.Config
	db    *db.DB
	repos *repositories.Repositories
	srv   *ssh.Server
}

func New(cfg *config.Config, database *db.DB) *Server {
	repos := repositories.New(database)
	
	s := &Server{
		cfg:   cfg,
		db:    database,
		repos: repos,
	}

	s.srv = s.createSSHServer()
	return s
}

func (s *Server) createSSHServer() *ssh.Server {
	srv, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf(":%d", s.cfg.SSHPort)),
		wish.WithHostKeyPath(s.cfg.SSHHostKeyPath),
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
	username := session.User()
	
	// Create the main dashboard app with access to repositories
	dashboardApp := apps.NewDashboard(s.repos, username)
	
	return dashboardApp, []tea.ProgramOption{
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