package entity

import (
	"errors"
	"fmt"
	"time"
)

const (
	StatePending   = "pending"
	StateCompleted = "completed"
)

type LinkStatus string

const (
	StatusAvailable    LinkStatus = "available"
	StatusNotAvailable LinkStatus = "not available"
	StatusChecking     LinkStatus = "checking"
)

type LinkRecord struct {
	ID          int                   `json:"id"`
	Links       []string              `json:"links"`
	Statuses    map[string]LinkStatus `json:"statuses"`
	State       string                `json:"state"`
	RequestedAt time.Time             `json:"requested_at"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
}

var ErrRecordNotFound = errors.New("record not found")

type NotFoundError struct {
	ID int
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("record %d not found", e.ID)
}

func (e NotFoundError) Is(target error) bool {
	if errors.Is(target, ErrRecordNotFound) {
		return true
	}
	var other NotFoundError
	return errors.As(target, &other) && other.ID == e.ID
}
