package api

import (
	"log/slog"
	"net/http"

	"go-data-distributor-notification/internal/config"
	db "go-data-distributor-notification/internal/db/sqlc"
	"go-data-distributor-notification/internal/middlewares"

	"github.com/gin-gonic/gin"
)

type Server struct {
	store  db.Querier
	router *gin.Engine
	logger *slog.Logger
}

func NewServer(cfg *config.Config, logger *slog.Logger,) *Server {
	server := &Server{
		logger: logger,
	}

	server.setupRouter()
	return server
}

func (s *Server) setupRouter() {
	s.router = gin.Default()

	s.setupMiddleware()

	// Public route
	s.router.GET("/health", s.healthCheck)

	// Private routes
	s.router.Use(middlewares.Authorization())
	setupRoutes(s)
}

func (s *Server) setupMiddleware() {
	s.router.Use(middlewares.Recovery(s.logger))
	s.router.Use(middlewares.Helmet())
	s.router.Use(middlewares.RequestID())
	// s.router.Use(middlewares.Logger(s.logger))
}

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) Start(address string) error {
	return s.router.Run(address)
}
