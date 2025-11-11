package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"links_service/internal/entity"
)

func TestChecker_Check(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := New(Config{
		Timeout:    1 * time.Second,
		WorkerPool: 2,
	})

	ctx := context.Background()
	links := []string{server.URL}

	statuses := checker.Check(ctx, links)

	if len(statuses) != 1 {
		t.Fatalf("Check() returned %d statuses, want 1", len(statuses))
	}

	if statuses[server.URL] != entity.StatusAvailable {
		t.Errorf("Check() status = %v, want %v", statuses[server.URL], entity.StatusAvailable)
	}
}

func TestChecker_Check_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	checker := New(Config{
		Timeout:    1 * time.Second,
		WorkerPool: 2,
	})

	ctx := context.Background()
	links := []string{server.URL}

	statuses := checker.Check(ctx, links)

	if statuses[server.URL] != entity.StatusNotAvailable {
		t.Errorf("Check() status = %v, want %v", statuses[server.URL], entity.StatusNotAvailable)
	}
}

func TestChecker_Check_InvalidDomain(t *testing.T) {
	checker := New(Config{
		Timeout:    1 * time.Second,
		WorkerPool: 2,
	})

	ctx := context.Background()
	links := []string{"invalid-domain-that-does-not-exist-12345.com"}

	statuses := checker.Check(ctx, links)

	if statuses[links[0]] != entity.StatusNotAvailable {
		t.Errorf("Check() status = %v, want %v", statuses[links[0]], entity.StatusNotAvailable)
	}
}

func TestChecker_Check_EmptyString(t *testing.T) {
	checker := New(Config{
		Timeout:    1 * time.Second,
		WorkerPool: 2,
	})

	ctx := context.Background()
	links := []string{""}

	statuses := checker.Check(ctx, links)

	if statuses[""] != entity.StatusNotAvailable {
		t.Errorf("Check() status = %v, want %v", statuses[""], entity.StatusNotAvailable)
	}
}

func TestChecker_Check_MultipleLinks(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()

	checker := New(Config{
		Timeout:    1 * time.Second,
		WorkerPool: 2,
	})

	ctx := context.Background()
	links := []string{server1.URL, server2.URL}

	statuses := checker.Check(ctx, links)

	if len(statuses) != 2 {
		t.Fatalf("Check() returned %d statuses, want 2", len(statuses))
	}

	if statuses[server1.URL] != entity.StatusAvailable {
		t.Errorf("Check() status for server1 = %v, want %v", statuses[server1.URL], entity.StatusAvailable)
	}

	if statuses[server2.URL] != entity.StatusAvailable {
		t.Errorf("Check() status for server2 = %v, want %v", statuses[server2.URL], entity.StatusAvailable)
	}
}

func TestCandidateURLs(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantHTTPS bool
		wantHTTP  bool
	}{
		{
			name:      "plain domain",
			input:     "example.com",
			wantHTTPS: true,
			wantHTTP:  true,
		},
		{
			name:      "with https prefix",
			input:     "https://example.com",
			wantHTTPS: true,
			wantHTTP:  false,
		},
		{
			name:      "with http prefix",
			input:     "http://example.com",
			wantHTTPS: false,
			wantHTTP:  true,
		},
		{
			name:      "empty string",
			input:     "",
			wantHTTPS: false,
			wantHTTP:  false,
		},
		{
			name:      "whitespace only",
			input:     "   ",
			wantHTTPS: false,
			wantHTTP:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := candidateURLs(tt.input)

			hasHTTPS := false
			hasHTTP := false
			for _, candidate := range candidates {
				if strings.HasPrefix(candidate, "https://") {
					hasHTTPS = true
				}
				if strings.HasPrefix(candidate, "http://") {
					hasHTTP = true
				}
			}

			if tt.wantHTTPS && !hasHTTPS {
				t.Errorf("candidateURLs() should include HTTPS variant, got %v", candidates)
			}
			if tt.wantHTTP && !hasHTTP {
				t.Errorf("candidateURLs() should include HTTP variant, got %v", candidates)
			}
			if !tt.wantHTTPS && hasHTTPS {
				t.Errorf("candidateURLs() should not include HTTPS variant, got %v", candidates)
			}
			if !tt.wantHTTP && hasHTTP {
				t.Errorf("candidateURLs() should not include HTTP variant, got %v", candidates)
			}
		})
	}
}

func TestNew_Defaults(t *testing.T) {
	checker := New(Config{})

	if checker.client.Timeout != 5*time.Second {
		t.Errorf("New() default timeout = %v, want 5s", checker.client.Timeout)
	}

	if checker.workerPool != 5 {
		t.Errorf("New() default workerPool = %d, want 5", checker.workerPool)
	}
}

func TestNew_CustomConfig(t *testing.T) {
	checker := New(Config{
		Timeout:    10 * time.Second,
		WorkerPool: 10,
	})

	if checker.client.Timeout != 10*time.Second {
		t.Errorf("New() timeout = %v, want 10s", checker.client.Timeout)
	}

	if checker.workerPool != 10 {
		t.Errorf("New() workerPool = %d, want 10", checker.workerPool)
	}
}
