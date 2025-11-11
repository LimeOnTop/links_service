package filerepository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"links_service/internal/entity"
)

func TestFileRepositoryCreateAndComplete(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "store.json")

	repo, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	record, err := repo.CreateRecord(context.Background(), []string{"example.com", "https://golang.org"})
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if record.ID != 1 {
		t.Fatalf("expected ID 1, got %d", record.ID)
	}

	pending, err := repo.PendingRecords(context.Background())
	if err != nil {
		t.Fatalf("PendingRecords: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending record, got %d", len(pending))
	}

	statuses := map[string]entity.LinkStatus{
		"example.com":        entity.StatusAvailable,
		"https://golang.org": entity.StatusNotAvailable,
	}

	if err := repo.CompleteRecord(context.Background(), record.ID, statuses); err != nil {
		t.Fatalf("CompleteRecord: %v", err)
	}

	reloaded, err := New(path)
	if err != nil {
		t.Fatalf("reload New: %v", err)
	}

	results, err := reloaded.GetRecords(context.Background(), []int{record.ID})
	if err != nil {
		t.Fatalf("GetRecords: %v", err)
	}

	got := results[0]
	for link, status := range statuses {
		if got.Statuses[link] != status {
			t.Errorf("status for %s = %s, want %s", link, got.Statuses[link], status)
		}
	}

	if got.State != entity.StateCompleted {
		t.Errorf("state = %s, want %s", got.State, entity.StateCompleted)
	}
	if got.CompletedAt == nil {
		t.Errorf("CompletedAt must be set")
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("storage file missing: %v", err)
	}
}
