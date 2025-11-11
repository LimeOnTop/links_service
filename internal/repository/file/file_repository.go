package filerepository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"links_service/internal/entity"
)

type persistedStore struct {
	NextID  int                  `json:"next_id"`
	Records []*entity.LinkRecord `json:"records"`
}

type FileRepository struct {
	mu     sync.RWMutex
	path   string
	nextID int
	data   map[int]*entity.LinkRecord
}

func New(path string) (*FileRepository, error) {
	repo := &FileRepository{
		path:   path,
		nextID: 1,
		data:   make(map[int]*entity.LinkRecord),
	}
	if err := repo.load(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *FileRepository) CreateRecord(_ context.Context, links []string) (*entity.LinkRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := r.nextID
	r.nextID++

	record := &entity.LinkRecord{
		ID:          id,
		Links:       append([]string(nil), links...),
		Statuses:    make(map[string]entity.LinkStatus, len(links)),
		State:       entity.StatePending,
		RequestedAt: time.Now().UTC(),
	}

	for _, link := range record.Links {
		record.Statuses[link] = entity.StatusChecking
	}

	r.data[id] = record

	if err := r.persistLocked(); err != nil {
		delete(r.data, id)
		r.nextID--
		return nil, err
	}

	return cloneRecord(record), nil
}

func (r *FileRepository) CompleteRecord(_ context.Context, id int, statuses map[string]entity.LinkStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.data[id]
	if !ok {
		return entity.NotFoundError{ID: id}
	}

	for _, link := range record.Links {
		if status, ok := statuses[link]; ok {
			record.Statuses[link] = status
			continue
		}
		if existing, ok := record.Statuses[link]; !ok || existing == entity.StatusChecking {
			record.Statuses[link] = entity.StatusNotAvailable
		}
	}

	now := time.Now().UTC()
	record.CompletedAt = &now
	record.State = entity.StateCompleted

	return r.persistLocked()
}

func (r *FileRepository) GetRecords(_ context.Context, ids []int) ([]*entity.LinkRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	records := make([]*entity.LinkRecord, 0, len(ids))
	for _, id := range ids {
		record, ok := r.data[id]
		if !ok {
			return nil, entity.NotFoundError{ID: id}
		}
		records = append(records, cloneRecord(record))
	}
	return records, nil
}

func (r *FileRepository) PendingRecords(_ context.Context) ([]*entity.LinkRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var pending []*entity.LinkRecord
	for _, record := range r.data {
		if record.State != entity.StateCompleted {
			pending = append(pending, cloneRecord(record))
		}
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i].ID < pending[j].ID
	})

	return pending, nil
}

func (r *FileRepository) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("creating storage dir: %w", err)
	}

	payload := persistedStore{
		NextID:  r.nextID,
		Records: make([]*entity.LinkRecord, 0, len(r.data)),
	}

	for _, record := range r.data {
		payload.Records = append(payload.Records, record)
	}

	sort.Slice(payload.Records, func(i, j int) bool {
		return payload.Records[i].ID < payload.Records[j].ID
	})

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal storage: %w", err)
	}

	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp storage: %w", err)
	}

	if err := os.Rename(tmp, r.path); err != nil {
		return fmt.Errorf("rename storage file: %w", err)
	}

	return nil
}

func (r *FileRepository) load() error {
	info, err := os.Stat(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat storage: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("storage path %s is a directory", r.path)
	}

	data, err := os.ReadFile(r.path)
	if err != nil {
		return fmt.Errorf("read storage: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	var payload persistedStore
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("unmarshal storage: %w", err)
	}

	r.nextID = payload.NextID
	if r.nextID <= 0 {
		r.nextID = 1
	}

	for _, record := range payload.Records {
		r.data[record.ID] = record
	}

	return nil
}

func cloneRecord(in *entity.LinkRecord) *entity.LinkRecord {
	if in == nil {
		return nil
	}

	out := &entity.LinkRecord{
		ID:          in.ID,
		Links:       append([]string(nil), in.Links...),
		Statuses:    make(map[string]entity.LinkStatus, len(in.Statuses)),
		State:       in.State,
		RequestedAt: in.RequestedAt,
	}

	if in.CompletedAt != nil {
		t := *in.CompletedAt
		out.CompletedAt = &t
	}

	for k, v := range in.Statuses {
		out.Statuses[k] = v
	}

	return out
}
