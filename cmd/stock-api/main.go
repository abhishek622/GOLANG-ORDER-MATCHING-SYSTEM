package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/config"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/http/handlers/order"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/http/handlers/trade"
	"github.com/abhishek622/GOLANG-ORDER-MATCHING-SYSTEM/internal/storage/mysql"
)

func main() {
	// load config
	cfg := config.MustLoad()

	// db setup
	storage, err := mysql.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	slog.Info("Storage initialized", slog.String("env", cfg.Env), slog.String("version", "1.0.0"))

	// setup router
	router := http.NewServeMux()

	orderHandler := order.NewOrderHandler(storage)
	tradeHandler := trade.ListTrades(storage)

	router.HandleFunc("POST /api/orders", orderHandler.PlaceOrder)
	router.HandleFunc("GET /api/orders/{orderId}", orderHandler.GetOrderStatus)
	router.HandleFunc("DELETE /api/orders/{orderId}", orderHandler.CancelOrder)
	router.HandleFunc("GET /api/orderbook", orderHandler.GetOrderBook)
	router.HandleFunc("GET /api/trades", tradeHandler)

	// setup server
	server := http.Server{
		Addr:    cfg.Addr,
		Handler: router,
	}

	slog.Info("Server started ", slog.String("address", cfg.Addr))

	// Graceful shutdown of server
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
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

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Failed to shutdown server", slog.String("error", err.Error()))
	}

	// Close database connection
	if err := storage.DB.Close(); err != nil {
		slog.Error("Failed to close database connection", slog.String("error", err.Error()))
	}

	slog.Info("Server shutdown successfully")
}
