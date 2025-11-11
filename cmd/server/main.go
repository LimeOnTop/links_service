package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"links_service/internal/app"
)

const (
	defaultPort        = 8080
	defaultStorageFile = "data/links_store.json"
)

func main() {
	var (
		addrFlag        = flag.String("addr", "", "HTTP listen address, e.g. :8080")
		storagePathFlag = flag.String("storage", defaultStorageFile, "Path to persistent storage file")
		timeoutFlag     = flag.Duration("timeout", 5*time.Second, "Timeout for link checks")
		workersFlag     = flag.Int("workers", 5, "Number of concurrent link checks")
	)
	flag.Parse()

	logger := log.New(os.Stdout, "[links-service] ", log.LstdFlags|log.Lmsgprefix)

	addr := resolveAddr(*addrFlag)
	storagePath := resolveStoragePath(*storagePathFlag)

	cfg := app.Config{
		Addr:        addr,
		StoragePath: storagePath,
		Timeout:     *timeoutFlag,
		Workers:     *workersFlag,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, cfg, logger); err != nil {
		logger.Fatalf("application error: %v", err)
	}
}

func resolveAddr(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if port := os.Getenv("PORT"); port != "" {
		return fmt.Sprintf(":%s", port)
	}
	return fmt.Sprintf(":%d", defaultPort)
}

func resolveStoragePath(flagValue string) string {
	if filepath.IsAbs(flagValue) {
		return flagValue
	}
	if flagValue == "" {
		flagValue = defaultStorageFile
	}
	return filepath.Join(projectRoot(), flagValue)
}

func projectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
