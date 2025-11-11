package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	filerepository "links_service/internal/repository/file"
	"links_service/internal/service/checker"
	"links_service/internal/service/httpapi"
	"links_service/internal/service/report/pdf"
	"links_service/internal/usecase"
)

const (
	defaultTimeout = 5 * time.Second
	defaultWorkers = 5
)

type Config struct {
	Addr        string
	StoragePath string
	Timeout     time.Duration
	Workers     int
}

func Run(ctx context.Context, cfg Config, logger *log.Logger) error {
	if logger == nil {
		return fmt.Errorf("logger must not be nil")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	workers := cfg.Workers
	if workers <= 0 {
		workers = defaultWorkers
	}

	repo, err := filerepository.New(cfg.StoragePath)
	if err != nil {
		return fmt.Errorf("init repository: %w", err)
	}

	checkerSvc := checker.New(checker.Config{
		Timeout:    timeout,
		WorkerPool: workers,
	})

	pdfGenerator := pdf.NewGenerator()
	linkService := usecase.NewLinkService(repo, checkerSvc, pdfGenerator)
	httpService := httpapi.New(linkService, logger)

	handler := loggingMiddleware(logger, httpService.Handler())

	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	recoverCtx, recoverCancel := context.WithCancel(context.Background())
	defer recoverCancel()

	go func() {
		if err := linkService.RecoverPending(recoverCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Printf("recover pending: %v", err)
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("listening on %s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		logger.Println("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
	} else {
		logger.Println("http server stopped gracefully")
	}

	recoverCancel()
	time.Sleep(500 * time.Millisecond)
	logger.Println("shutdown complete")
	return nil
}

func loggingMiddleware(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
