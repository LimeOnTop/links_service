package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"links_service/internal/entity"
	"links_service/internal/usecase"
)

type Service struct {
	linkService *usecase.LinkService
	logger      *log.Logger
}

func New(linkService *usecase.LinkService, logger *log.Logger) *Service {
	return &Service{
		linkService: linkService,
		logger:      logger,
	}
}

// Handler builds the HTTP multiplexer for the API.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/links/check", s.handleCheck)
	mux.HandleFunc("/api/v1/links/report", s.handleReport)
	mux.HandleFunc("/healthz", s.handleHealth)
	return mux
}

type checkRequest struct {
	Links []string `json:"links"`
}

type checkResponse struct {
	Links    map[string]string `json:"links"`
	LinksNum int               `json:"links_num"`
	LinksSum int               `json:"links_sum,omitempty"`
}

func (s *Service) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()

	var req checkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	result, err := s.linkService.SubmitLinks(ctx, req.Links)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrEmptyLinks), errors.Is(err, usecase.ErrNoValidLinks):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			s.logger.Printf("submit links: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to process links")
		}
		return
	}

	resp := checkResponse{
		Links:    make(map[string]string, len(result.Links)),
		LinksNum: result.RecordID,
		LinksSum: result.RecordID,
	}

	for link, status := range result.Links {
		resp.Links[link] = string(status)
	}

	writeJSON(w, http.StatusOK, resp)
}

type reportRequest struct {
	LinksList []int `json:"links_list"`
}

func (s *Service) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()

	var req reportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	pdfBytes, err := s.linkService.GenerateReport(ctx, req.LinksList)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrEmptyLinks):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, entity.ErrRecordNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			var notFound entity.NotFoundError
			if errors.As(err, &notFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			s.logger.Printf("generate report: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to generate report")
		}
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="links-report-%s.pdf"`, time.Now().UTC().Format("20060102T150405Z")))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(pdfBytes); err != nil {
		s.logger.Printf("write pdf response: %v", err)
	}
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		log.Printf("encode json: %v", err)
	}
}

type errorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{
		Error: message,
		Code:  status,
	})
}
