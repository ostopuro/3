package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/athom/hotel-merge/internal/api"
	"github.com/athom/hotel-merge/internal/app"
	"github.com/athom/hotel-merge/internal/store"
	"github.com/athom/hotel-merge/internal/suppliers"
)

const (
	acmeEndpoint       = "https://5f2be0b4ffc88500167b85a0.mockapi.io/suppliers/acme"
	patagoniaEndpoint  = "https://5f2be0b4ffc88500167b85a0.mockapi.io/suppliers/patagonia"
	paperfliesEndpoint = "https://5f2be0b4ffc88500167b85a0.mockapi.io/suppliers/paperflies"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpClient := &http.Client{Timeout: 20 * time.Second}

	supplierCfg := suppliers.Config{Client: httpClient, Timeout: 10 * time.Second}
	supplierList := []suppliers.Supplier{
		suppliers.NewAcmeSupplier(suppliers.Config{Client: supplierCfg.Client, Timeout: supplierCfg.Timeout, Endpoint: acmeEndpoint}),
		suppliers.NewPatagoniaSupplier(suppliers.Config{Client: supplierCfg.Client, Timeout: supplierCfg.Timeout, Endpoint: patagoniaEndpoint}),
		suppliers.NewPaperfliesSupplier(suppliers.Config{Client: supplierCfg.Client, Timeout: supplierCfg.Timeout, Endpoint: paperfliesEndpoint}),
	}

	dataStore := store.NewMemoryStore()
	auditStore := store.NewMemoryAuditStore()

	logger := log.New(os.Stdout, "hotel-merge ", log.LstdFlags)

	service := app.NewService(app.Config{
		Store:         dataStore,
		Audit:         auditStore,
		Suppliers:     supplierList,
		FetchInterval: 5 * time.Minute,
		MergeInterval: time.Minute,
		Logger:        logger,
	})
	service.Start(ctx)
	defer service.Stop()

	router := gin.Default()
	handler := api.NewHandler(service)
	handler.Register(router)

	port := envOrDefault("PORT", "8080")
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Printf("API listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("server error: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
