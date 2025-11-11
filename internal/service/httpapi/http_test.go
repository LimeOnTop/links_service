package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"links_service/internal/entity"
	"links_service/internal/usecase"
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

func TestService_handleCheck(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		body             string
		wantStatus       int
		validateResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:       "method not allowed",
			method:     http.MethodGet,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "invalid JSON",
			method:     http.MethodPost,
			body:       "invalid json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty links",
			method:     http.MethodPost,
			body:       `{"links": []}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "success",
			method:     http.MethodPost,
			body:       `{"links": ["google.com"]}`,
			wantStatus: http.StatusOK,
			validateResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp checkResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp.LinksNum == 0 {
					t.Error("LinksNum is zero")
				}
				if status, ok := resp.Links["google.com"]; !ok {
					t.Error("Links[google.com] not found")
				} else if status != "available" && status != "not available" {
					t.Errorf("Links[google.com] = %q, want 'available' or 'not available'", status)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			checker := newMockChecker()
			reporter := newMockReporter()

			checker.statuses["google.com"] = entity.StatusAvailable

			linkService := usecase.NewLinkService(repo, checker, reporter)
			httpService := New(linkService, nil)

			req := httptest.NewRequest(tt.method, "/api/v1/links/check", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			httpService.handleCheck(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("handleCheck() status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}
		})
	}
}

func TestService_handleReport(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		body             string
		wantStatus       int
		validateResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:       "method not allowed",
			method:     http.MethodGet,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "invalid JSON",
			method:     http.MethodPost,
			body:       "invalid json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty links_list",
			method:     http.MethodPost,
			body:       `{"links_list": []}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "record not found",
			method:     http.MethodPost,
			body:       `{"links_list": [999]}`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "success",
			method:     http.MethodPost,
			body:       `{"links_list": [1]}`,
			wantStatus: http.StatusOK,
			validateResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Header().Get("Content-Type") != "application/pdf" {
					t.Errorf("Content-Type = %q, want 'application/pdf'", w.Header().Get("Content-Type"))
				}
				if len(w.Body.Bytes()) == 0 {
					t.Error("response body is empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			checker := newMockChecker()
			reporter := newMockReporter()

			// Setup for success case
			if tt.name == "success" {
				repo.CreateRecord(context.Background(), []string{"test.com"})
				repo.CompleteRecord(context.Background(), 1, map[string]entity.LinkStatus{
					"test.com": entity.StatusAvailable,
				})
			}

			linkService := usecase.NewLinkService(repo, checker, reporter)
			httpService := New(linkService, nil)

			req := httptest.NewRequest(tt.method, "/api/v1/links/report", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			httpService.handleReport(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("handleReport() status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}
		})
	}
}

func TestService_handleHealth(t *testing.T) {
	httpService := &Service{}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	httpService.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleHealth() status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("status = %q, want 'ok'", resp["status"])
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("writeError() status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error != "test error" {
		t.Errorf("Error = %q, want 'test error'", resp.Error)
	}

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Code = %d, want %d", resp.Code, http.StatusBadRequest)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	payload := map[string]string{"key": "value"}

	writeJSON(w, http.StatusOK, payload)

	if w.Code != http.StatusOK {
		t.Errorf("writeJSON() status = %d, want %d", w.Code, http.StatusOK)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want 'application/json'", w.Header().Get("Content-Type"))
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("result[key] = %q, want 'value'", result["key"])
	}
}
