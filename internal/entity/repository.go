package entity

import "context"

type LinkRepository interface {
	CreateRecord(ctx context.Context, links []string) (*LinkRecord, error)
	CompleteRecord(ctx context.Context, id int, statuses map[string]LinkStatus) error
	GetRecords(ctx context.Context, ids []int) ([]*LinkRecord, error)
	PendingRecords(ctx context.Context) ([]*LinkRecord, error)
}
