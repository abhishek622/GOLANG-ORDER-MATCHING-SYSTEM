package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/config"
)

func main() {
	// load config
	cfg := config.MustLoad()

	// Initialize database connection
	dsn := cfg.DatabaseURL()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		slog.Error("Failed to connect to database", slog.String("error", err.Error()))
		return
	}

	// Test the connection
	err = db.Ping()
	if err != nil {
		slog.Error("Failed to ping database", slog.String("error", err.Error()))
		return
	}

	// Configure connection pool
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)

	slog.Info("Storage initialized", slog.String("env", cfg.Env), slog.String("version", "1.0.0"))
	defer db.Close()
	// setup router
	router := http.NewServeMux()

	// setup server
	server := http.Server{
		Addr:    cfg.Addr,
		Handler: router,
	}

	slog.Info("Server started 🚀", slog.String("address", cfg.Addr))

	// Graceful shutdown of server
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTTIN, syscall.SIGTERM)
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			slog.Error("Failed to start server", slog.String("error", err.Error()))
		}
	}()

	<-done

	slog.Info("Shutting down the server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Failed to shutdown server", slog.String("error", err.Error()))
	}

	slog.Info("Server shutdown successfully")
}
