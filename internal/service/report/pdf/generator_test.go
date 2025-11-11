package pdf

import (
	"testing"
	"time"

	"links_service/internal/entity"
)

func TestGenerator_Generate(t *testing.T) {
	generator := NewGenerator()

	records := []*entity.LinkRecord{
		{
			ID:    1,
			Links: []string{"google.com", "example.org"},
			Statuses: map[string]entity.LinkStatus{
				"google.com":  entity.StatusAvailable,
				"example.org": entity.StatusNotAvailable,
			},
			State:       entity.StateCompleted,
			RequestedAt: time.Now().UTC(),
			CompletedAt: func() *time.Time { t := time.Now().UTC(); return &t }(),
		},
		{
			ID:    2,
			Links: []string{"github.com"},
			Statuses: map[string]entity.LinkStatus{
				"github.com": entity.StatusAvailable,
			},
			State:       entity.StateCompleted,
			RequestedAt: time.Now().UTC(),
		},
	}

	pdfBytes, err := generator.Generate(records)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Error("Generate() returned empty PDF")
	}

	// PDF files start with %PDF
	if len(pdfBytes) < 4 || string(pdfBytes[0:4]) != "%PDF" {
		t.Error("Generate() did not return valid PDF format")
	}
}

func TestGenerator_Generate_EmptyRecords(t *testing.T) {
	generator := NewGenerator()

	records := []*entity.LinkRecord{}

	pdfBytes, err := generator.Generate(records)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Error("Generate() returned empty PDF even for empty records")
	}
}

func TestGenerator_Generate_SingleRecord(t *testing.T) {
	generator := NewGenerator()

	records := []*entity.LinkRecord{
		{
			ID:    1,
			Links: []string{"example.com"},
			Statuses: map[string]entity.LinkStatus{
				"example.com": entity.StatusAvailable,
			},
			State:       entity.StateCompleted,
			RequestedAt: time.Now().UTC(),
		},
	}

	pdfBytes, err := generator.Generate(records)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Error("Generate() returned empty PDF")
	}
}

func TestGenerator_Generate_MultipleRecords(t *testing.T) {
	generator := NewGenerator()

	records := []*entity.LinkRecord{
		{
			ID:    1,
			Links: []string{"google.com"},
			Statuses: map[string]entity.LinkStatus{
				"google.com": entity.StatusAvailable,
			},
			State:       entity.StateCompleted,
			RequestedAt: time.Now().UTC(),
		},
		{
			ID:    2,
			Links: []string{"example.org"},
			Statuses: map[string]entity.LinkStatus{
				"example.org": entity.StatusNotAvailable,
			},
			State:       entity.StateCompleted,
			RequestedAt: time.Now().UTC(),
		},
		{
			ID:    3,
			Links: []string{"github.com", "stackoverflow.com"},
			Statuses: map[string]entity.LinkStatus{
				"github.com":        entity.StatusAvailable,
				"stackoverflow.com": entity.StatusAvailable,
			},
			State:       entity.StateCompleted,
			RequestedAt: time.Now().UTC(),
		},
	}

	pdfBytes, err := generator.Generate(records)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Error("Generate() returned empty PDF")
	}
}

func TestNewGenerator(t *testing.T) {
	generator := NewGenerator()
	if generator == nil {
		t.Fatal("NewGenerator() returned nil")
	}
}
