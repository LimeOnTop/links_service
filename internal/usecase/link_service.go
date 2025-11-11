package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"

	"links_service/internal/entity"
)

var (
	ErrEmptyLinks   = errors.New("links list must not be empty")
	ErrNoValidLinks = errors.New("no valid links supplied")
)

type LinkChecker interface {
	Check(ctx context.Context, links []string) map[string]entity.LinkStatus
}

type ReportGenerator interface {
	Generate(records []*entity.LinkRecord) ([]byte, error)
}

type LinkService struct {
	repo     entity.LinkRepository
	checker  LinkChecker
	reporter ReportGenerator
}

func NewLinkService(repo entity.LinkRepository, checker LinkChecker, reporter ReportGenerator) *LinkService {
	return &LinkService{
		repo:     repo,
		checker:  checker,
		reporter: reporter,
	}
}

type SubmitLinksResult struct {
	Links    map[string]entity.LinkStatus
	RecordID int
}

func (s *LinkService) SubmitLinks(ctx context.Context, links []string) (*SubmitLinksResult, error) {
	if len(links) == 0 {
		return nil, ErrEmptyLinks
	}
	filtered := normalizeLinks(links)
	if len(filtered) == 0 {
		return nil, ErrNoValidLinks
	}

	record, err := s.repo.CreateRecord(ctx, filtered)
	if err != nil {
		return nil, err
	}

	statuses := s.checker.Check(ctx, filtered)

	if err := s.repo.CompleteRecord(ctx, record.ID, statuses); err != nil {
		return nil, err
	}

	return &SubmitLinksResult{
		Links:    statuses,
		RecordID: record.ID,
	}, nil
}

func (s *LinkService) GenerateReport(ctx context.Context, ids []int) ([]byte, error) {
	if len(ids) == 0 {
		return nil, ErrEmptyLinks
	}

	deduped := dedupeInts(ids)

	records, err := s.repo.GetRecords(ctx, deduped)
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		if record.State == entity.StateCompleted {
			continue
		}
		statuses := s.checker.Check(ctx, record.Links)
		if err := s.repo.CompleteRecord(ctx, record.ID, statuses); err != nil && !errors.Is(err, entity.ErrRecordNotFound) {
			return nil, err
		}
	}

	records, err = s.repo.GetRecords(ctx, deduped)
	if err != nil {
		return nil, err
	}

	return s.reporter.Generate(records)
}

func (s *LinkService) RecoverPending(ctx context.Context) error {
	records, err := s.repo.PendingRecords(ctx)
	if err != nil {
		return err
	}

	for _, record := range records {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		statuses := s.checker.Check(ctx, record.Links)
		if err := s.repo.CompleteRecord(ctx, record.ID, statuses); err != nil && !errors.Is(err, entity.ErrRecordNotFound) {
			return err
		}
	}
	return nil
}

func normalizeLinks(links []string) []string {
	result := make([]string, 0, len(links))
	seen := make(map[string]struct{}, len(links))
	for _, raw := range links {
		link := strings.TrimSpace(raw)
		if link == "" {
			continue
		}
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}
		result = append(result, link)
	}
	return result
}

func dedupeInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	sort.Ints(result)
	return result
}
