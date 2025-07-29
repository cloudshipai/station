package api

import (
	"context"
	"fmt"
	"net/http"
	"station/internal/config"
	"station/internal/db"

	"github.com/gin-gonic/gin"
)

type Server struct {
	cfg        *config.Config
	db         db.Database
	httpServer *http.Server
}

func New(cfg *config.Config, database db.Database) *Server {
	return &Server{
		cfg: cfg,
		db:  database,
	}
}

func (s *Server) Start(ctx context.Context) error {
	// Set Gin to release mode for production
	gin.SetMode(gin.ReleaseMode)
	
	// Create Gin router with minimal middleware
	router := gin.New()
	router.Use(gin.Recovery())
	
	// Health check endpoint only
	router.GET("/health", s.healthCheck)
	
	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.APIPort),
		Handler: router,
	}
	
	// Start server in goroutine
	go func() {
		fmt.Printf("🚀 API server starting on port %d\n", s.cfg.APIPort)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("API server error: %v\n", err)
		}
	}()
	
	// Wait for context cancellation
	<-ctx.Done()
	
	// Graceful shutdown
	fmt.Println("🛑 Shutting down API server...")
	return s.httpServer.Shutdown(context.Background())
}

// Health check endpoint
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "station-api",
		"version": "1.0.0",
	})
}