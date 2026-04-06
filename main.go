package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"linkvault/cache"
	"linkvault/license"
	"linkvault/sdk"
	"linkvault/store"
	"linkvault/web"
)

func main() {
	cfg := loadConfig()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, err := store.New(cfg.pgConnStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	redisCache := cache.New(cfg.redisAddr, cfg.redisPassword)
	defer redisCache.Close()

	lic := license.NewChecker(cfg.sdkAddr)
	sdkClient := sdk.NewClient(cfg.sdkAddr)

	go lic.RefreshLoop(ctx, 5*time.Minute)
	go sdkClient.ReportLoop(ctx, db, 60*time.Second)
	go sdkClient.UpdateCheckLoop(ctx, 5*time.Minute)

	handler := web.NewRouter(db, redisCache, lic, sdkClient)
	srv := &http.Server{Addr: cfg.listenAddr, Handler: handler}

	go func() {
		log.Printf("LinkVault starting on %s", cfg.listenAddr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}

type config struct {
	pgConnStr     string
	redisAddr     string
	redisPassword string
	sdkAddr       string
	listenAddr    string
}

func loadConfig() config {
	host := envOr("PGHOST", "localhost")
	port := envOr("PGPORT", "5432")
	user := envOr("PGUSER", "postgres")
	pass := envOr("PGPASSWORD", "linkvault")
	dbname := envOr("PGDATABASE", "linkvault")

	connStr := "host=" + host + " port=" + port + " user=" + user +
		" password=" + pass + " dbname=" + dbname + " sslmode=disable"

	return config{
		pgConnStr:     connStr,
		redisAddr:     envOr("REDIS_ADDR", ""),
		redisPassword: envOr("REDIS_PASSWORD", ""),
		sdkAddr:       envOr("REPLICATED_SDK_ADDRESS", "http://replicated:3000"),
		listenAddr:    envOr("LISTEN_ADDR", ":8080"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
