package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"links_service/internal/entity"
)

type mockRepository struct {
	records     map[int]*entity.LinkRecord
	nextID      int
	createErr   error
	completeErr error
	getErr      error
	pendingErr  error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		records: make(map[int]*entity.LinkRecord),
		nextID:  1,
	}
}

func (m *mockRepository) CreateRecord(ctx context.Context, links []string) (*entity.LinkRecord, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	record := &entity.LinkRecord{
		ID:          m.nextID,
		Links:       links,
		Statuses:    make(map[string]entity.LinkStatus),
		State:       entity.StatePending,
		RequestedAt: time.Now().UTC(),
	}
	for _, link := range links {
		record.Statuses[link] = entity.StatusChecking
	}
	m.records[m.nextID] = record
	m.nextID++
	return record, nil
}

func (m *mockRepository) CompleteRecord(ctx context.Context, id int, statuses map[string]entity.LinkStatus) error {
	if m.completeErr != nil {
		return m.completeErr
	}
	record, ok := m.records[id]
	if !ok {
		return entity.NotFoundError{ID: id}
	}
	for link, status := range statuses {
		record.Statuses[link] = status
	}
	now := time.Now().UTC()
	record.CompletedAt = &now
	record.State = entity.StateCompleted
	return nil
}

func (m *mockRepository) GetRecords(ctx context.Context, ids []int) ([]*entity.LinkRecord, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	var records []*entity.LinkRecord
	for _, id := range ids {
		if record, ok := m.records[id]; ok {
			records = append(records, record)
		} else {
			return nil, entity.NotFoundError{ID: id}
		}
	}
	return records, nil
}

func (m *mockRepository) PendingRecords(ctx context.Context) ([]*entity.LinkRecord, error) {
	if m.pendingErr != nil {
		return nil, m.pendingErr
	}
	var pending []*entity.LinkRecord
	for _, record := range m.records {
		if record.State != entity.StateCompleted {
			pending = append(pending, record)
		}
	}
	return pending, nil
}

type mockChecker struct {
	statuses map[string]entity.LinkStatus
}

func newMockChecker() *mockChecker {
	return &mockChecker{
		statuses: make(map[string]entity.LinkStatus),
	}
}

func (m *mockChecker) Check(ctx context.Context, links []string) map[string]entity.LinkStatus {
	result := make(map[string]entity.LinkStatus)
	for _, link := range links {
		if status, ok := m.statuses[link]; ok {
			result[link] = status
		} else {
			result[link] = entity.StatusNotAvailable
		}
	}
	return result
}

type mockReporter struct {
	generateErr error
	bytes       []byte
}

func newMockReporter() *mockReporter {
	return &mockReporter{
		bytes: []byte("mock pdf"),
	}
}

func (m *mockReporter) Generate(records []*entity.LinkRecord) ([]byte, error) {
	if m.generateErr != nil {
		return nil, m.generateErr
	}
	return m.bytes, nil
}

func TestLinkService_SubmitLinks(t *testing.T) {
	tests := []struct {
		name    string
		links   []string
		wantErr error
		setup   func(*mockRepository, *mockChecker)
	}{
		{
			name:    "empty links",
			links:   []string{},
			wantErr: ErrEmptyLinks,
		},
		{
			name:    "only whitespace",
			links:   []string{"  ", "\t", "\n"},
			wantErr: ErrNoValidLinks,
		},
		{
			name:  "success",
			links: []string{"google.com", "example.org"},
			setup: func(repo *mockRepository, checker *mockChecker) {
				checker.statuses["google.com"] = entity.StatusAvailable
				checker.statuses["example.org"] = entity.StatusNotAvailable
			},
		},
		{
			name:  "deduplicates links",
			links: []string{"google.com", "google.com", "example.org"},
			setup: func(repo *mockRepository, checker *mockChecker) {
				checker.statuses["google.com"] = entity.StatusAvailable
				checker.statuses["example.org"] = entity.StatusNotAvailable
			},
		},
		{
			name:  "normalizes whitespace",
			links: []string{"  google.com  ", "example.org"},
			setup: func(repo *mockRepository, checker *mockChecker) {
				checker.statuses["google.com"] = entity.StatusAvailable
				checker.statuses["example.org"] = entity.StatusNotAvailable
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			checker := newMockChecker()
			reporter := newMockReporter()

			if tt.setup != nil {
				tt.setup(repo, checker)
			}

			service := NewLinkService(repo, checker, reporter)
			result, err := service.SubmitLinks(context.Background(), tt.links)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("SubmitLinks() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("SubmitLinks() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Fatal("SubmitLinks() result is nil")
			}

			if result.RecordID == 0 {
				t.Error("SubmitLinks() RecordID is zero")
			}

			if len(result.Links) == 0 {
				t.Error("SubmitLinks() Links is empty")
			}
		})
	}
}

func TestLinkService_GenerateReport(t *testing.T) {
	tests := []struct {
		name    string
		ids     []int
		wantErr error
		setup   func(*mockRepository, *mockChecker, *mockReporter)
	}{
		{
			name:    "empty ids",
			ids:     []int{},
			wantErr: ErrEmptyLinks,
		},
		{
			name: "success with completed records",
			ids:  []int{1, 2},
			setup: func(repo *mockRepository, checker *mockChecker, reporter *mockReporter) {
				repo.CreateRecord(context.Background(), []string{"google.com"})
				repo.CompleteRecord(context.Background(), 1, map[string]entity.LinkStatus{
					"google.com": entity.StatusAvailable,
				})
				repo.CreateRecord(context.Background(), []string{"example.org"})
				repo.CompleteRecord(context.Background(), 2, map[string]entity.LinkStatus{
					"example.org": entity.StatusNotAvailable,
				})
			},
		},
		{
			name: "completes pending records",
			ids:  []int{1},
			setup: func(repo *mockRepository, checker *mockChecker, reporter *mockReporter) {
				repo.CreateRecord(context.Background(), []string{"google.com"})
				checker.statuses["google.com"] = entity.StatusAvailable
			},
		},
		{
			name: "deduplicates ids",
			ids:  []int{1, 1, 2},
			setup: func(repo *mockRepository, checker *mockChecker, reporter *mockReporter) {
				repo.CreateRecord(context.Background(), []string{"google.com"})
				repo.CompleteRecord(context.Background(), 1, map[string]entity.LinkStatus{
					"google.com": entity.StatusAvailable,
				})
				repo.CreateRecord(context.Background(), []string{"example.org"})
				repo.CompleteRecord(context.Background(), 2, map[string]entity.LinkStatus{
					"example.org": entity.StatusNotAvailable,
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			checker := newMockChecker()
			reporter := newMockReporter()

			if tt.setup != nil {
				tt.setup(repo, checker, reporter)
			}

			service := NewLinkService(repo, checker, reporter)
			result, err := service.GenerateReport(context.Background(), tt.ids)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("GenerateReport() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("GenerateReport() unexpected error = %v", err)
				return
			}

			if len(result) == 0 {
				t.Error("GenerateReport() result is empty")
			}
		})
	}
}

func TestLinkService_RecoverPending(t *testing.T) {
	repo := newMockRepository()
	checker := newMockChecker()
	reporter := newMockReporter()

	repo.CreateRecord(context.Background(), []string{"google.com"})
	checker.statuses["google.com"] = entity.StatusAvailable

	service := NewLinkService(repo, checker, reporter)
	err := service.RecoverPending(context.Background())

	if err != nil {
		t.Errorf("RecoverPending() error = %v", err)
	}

	pending, _ := repo.PendingRecords(context.Background())
	if len(pending) != 0 {
		t.Errorf("RecoverPending() should complete all pending records, got %d pending", len(pending))
	}
}

func TestNormalizeLinks(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "removes empty strings",
			input:    []string{"google.com", "", "example.org"},
			expected: []string{"google.com", "example.org"},
		},
		{
			name:     "trims whitespace",
			input:    []string{"  google.com  ", "example.org"},
			expected: []string{"google.com", "example.org"},
		},
		{
			name:     "deduplicates",
			input:    []string{"google.com", "example.org", "google.com"},
			expected: []string{"google.com", "example.org"},
		},
		{
			name:     "preserves order",
			input:    []string{"a.com", "b.com", "c.com"},
			expected: []string{"a.com", "b.com", "c.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeLinks(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("normalizeLinks() length = %d, want %d", len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("normalizeLinks()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestDedupeInts(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "deduplicates",
			input:    []int{1, 2, 1, 3, 2},
			expected: []int{1, 2, 3},
		},
		{
			name:     "sorts result",
			input:    []int{3, 1, 2},
			expected: []int{1, 2, 3},
		},
		{
			name:     "empty input",
			input:    []int{},
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dedupeInts(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("dedupeInts() length = %d, want %d", len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("dedupeInts()[%d] = %d, want %d", i, v, tt.expected[i])
				}
			}
		})
	}
}
