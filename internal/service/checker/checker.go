package checker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"links_service/internal/entity"
)

// Checker validates availability of remote HTTP resources.
type Checker struct {
	client     *http.Client
	workerPool int
}

// Config describes runtime parameters for Checker.
type Config struct {
	Timeout    time.Duration
	WorkerPool int
}

// New creates a checker with sensible defaults.
func New(cfg Config) *Checker {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	workers := cfg.WorkerPool
	if workers <= 0 {
		workers = 5
	}

	return &Checker{
		client: &http.Client{
			Timeout: timeout,
		},
		workerPool: workers,
	}
}

// Check returns availability statuses for provided links.
func (c *Checker) Check(ctx context.Context, links []string) map[string]entity.LinkStatus {
	results := make(map[string]entity.LinkStatus, len(links))
	var mu sync.Mutex

	wg := sync.WaitGroup{}
	sem := make(chan struct{}, c.workerPool)

	for _, raw := range links {
		raw := raw
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			status := c.checkSingle(ctx, raw)

			mu.Lock()
			results[raw] = status
			mu.Unlock()
		}()
	}

	wg.Wait()
	return results
}

func (c *Checker) checkSingle(ctx context.Context, raw string) entity.LinkStatus {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return entity.StatusNotAvailable
	}

	candidates := candidateURLs(raw)

	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return entity.StatusNotAvailable
		}

		if c.tryURL(ctx, candidate) {
			return entity.StatusAvailable
		}
	}

	return entity.StatusNotAvailable
}

func (c *Checker) tryURL(ctx context.Context, urlStr string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, http.NoBody)
	if err != nil {
		return false
	}

	resp, err := c.client.Do(req)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			return false
		}
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			return false
		}
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

func candidateURLs(link string) []string {
	link = strings.TrimSpace(link)
	if link == "" {
		return []string{}
	}

	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		if _, err := url.Parse(link); err == nil {
			return []string{link}
		}
	}

	escaped := strings.TrimPrefix(link, "//")
	if escaped == "" {
		return []string{}
	}

	candidates := []string{
		fmt.Sprintf("https://%s", escaped),
		fmt.Sprintf("http://%s", escaped),
	}

	return candidates
}
