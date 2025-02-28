package main

import (
	"context"
	"log/slog"
	"os"

	"go-data-distributor-notification/internal/api"
	"go-data-distributor-notification/internal/config"
	db "go-data-distributor-notification/internal/db/sqlc"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := config.Load(logger)

	conn, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("Cannot connect to database", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	store := db.New(conn)
	server := api.NewServer(cfg, logger, store)

	logger.Info("Starting server", "port", cfg.Port)
	if err := server.Start(":" + cfg.Port); err != nil {
		logger.Error("Cannot start server", "error", err)
		os.Exit(1)
	}
}
