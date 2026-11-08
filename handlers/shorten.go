package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

type URLService interface {
	CreateURL(ctx context.Context, originalURL string, expiresAt *time.Time) (string, error)
	GetClickCount(ctx context.Context, shortCode string) (int64, error)
	ResolveURL(ctx context.Context, shortCode string) (string, error)
	Delete(ctx context.Context, shortCode string) error
}

type URLHandler struct {
	service URLService
}

func NewURLHandler(service URLService) *URLHandler {
	return &URLHandler{service: service}
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type ShortenRequest struct {
	URL       string `json:"url"`
	ExpiresIn *int64 `json:"expires_in,omitempty"`
}

func (h *URLHandler) ShortenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ShortenRequest

	err := json.NewDecoder(r.Body).Decode(&req) // use & because it needs to modify req (initially empty)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "Empty URL not accepted", http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(req.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	if parsedURL.Hostname() == "" {
		http.Error(w, "URL must contain a hostname", http.StatusBadRequest)
		return
	}

	var expiresAt *time.Time
	const maxExpirationSeconds int64 = 30 * 24 * 60 * 60
	if req.ExpiresIn != nil {
		seconds := *req.ExpiresIn
		if seconds <= 0 || seconds > maxExpirationSeconds {
			http.Error(w, "expires_in must be between 1 and 2592000 seconds", http.StatusBadRequest)
			return
		}
		duration := time.Duration(seconds) * time.Second
		expiration := time.Now().Add(duration)

		expiresAt = &expiration
	}

	shortCode, err := h.service.CreateURL(r.Context(), req.URL, expiresAt)
	if err != nil {
		http.Error(w, "Failed to generate short URL", http.StatusInternalServerError)
		return
	}

	shortenURL := "http://localhost:8080/" + shortCode

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(
		map[string]any{
			"url": shortenURL,
		},
	)
}
