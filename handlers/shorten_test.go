package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"leetx/url/shortener/utils"
)

type stubURLService struct {
	create  func(context.Context, string, *time.Time) (string, error)
	resolve func(context.Context, string) (string, error)
	stats   func(context.Context, string) (int64, error)
	delete  func(context.Context, string) error
}

var _ URLService = (*stubURLService)(nil)

func (s *stubURLService) CreateURL(ctx context.Context, url string, expiresAt *time.Time) (string, error) {
	return s.create(ctx, url, expiresAt)
}
func (s *stubURLService) ResolveURL(ctx context.Context, code string) (string, error) {
	return s.resolve(ctx, code)
}
func (s *stubURLService) GetClickCount(ctx context.Context, code string) (int64, error) {
	return s.stats(ctx, code)
}
func (s *stubURLService) Delete(ctx context.Context, code string) error {
	return s.delete(ctx, code)
}

func TestShortenHandlerSuccess(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		seconds    int
	}{
		{"without expiration", `{"url":"https://example.com"}`, 0},
		{"with expiration", `{"url":"https://example.com","expires_in":60}`, 60},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			calls := 0
			svc := &stubURLService{create: func(ctx context.Context, originalURL string, expiresAt *time.Time) (string, error) {
				calls++
				if originalURL != "https://example.com" {
					t.Errorf("URL = %q", originalURL)
				}
				if tt.seconds == 0 {
					if expiresAt != nil {
						t.Errorf("expiration = %v, want nil", expiresAt)
					}
				} else {
					duration := time.Duration(tt.seconds) * time.Second
					if expiresAt == nil || expiresAt.Before(before.Add(duration)) || expiresAt.After(time.Now().Add(duration)) {
						t.Errorf("expiration = %v, want request time plus %d seconds", expiresAt, tt.seconds)
					}
				}
				return "abc12345", nil
			}}
			req := httptest.NewRequest(http.MethodPost, "/urls", strings.NewReader(tt.body))
			recorder := httptest.NewRecorder()
			NewURLHandler(svc).ShortenHandler(recorder, req)
			if recorder.Code != http.StatusCreated {
				t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
			}
			if calls != 1 {
				t.Errorf("CreateURL calls = %d, want 1", calls)
			}
			if got := recorder.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q", got)
			}
			var response struct {
				URL string `json:"url"`
			}
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatal(err)
			}
			if response.URL != "http://localhost:8080/abc12345" {
				t.Errorf("short URL = %q", response.URL)
			}
		})
	}
}

func TestShortenHandlerInvalidRequest(t *testing.T) {
	for _, tt := range []struct {
		name, method, body string
		status             int
	}{
		{"wrong method", http.MethodGet, "", http.StatusMethodNotAllowed},
		{"malformed JSON", http.MethodPost, "{", http.StatusBadRequest},
		{"empty URL", http.MethodPost, `{}`, http.StatusBadRequest},
		{"invalid scheme", http.MethodPost, `{"url":"ftp://example.com"}`, http.StatusBadRequest},
		{"missing hostname", http.MethodPost, `{"url":"https:///path"}`, http.StatusBadRequest},
		{"zero expiration", http.MethodPost, `{"url":"https://example.com","expires_in":0}`, http.StatusBadRequest},
		{"negative expiration", http.MethodPost, `{"url":"https://example.com","expires_in":-1}`, http.StatusBadRequest},
		{"excessive expiration", http.MethodPost, `{"url":"https://example.com","expires_in":2592001}`, http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubURLService{create: func(context.Context, string, *time.Time) (string, error) {
				t.Fatal("invalid request called CreateURL")
				return "", nil
			}}
			recorder := httptest.NewRecorder()
			NewURLHandler(svc).ShortenHandler(recorder, httptest.NewRequest(tt.method, "/urls", strings.NewReader(tt.body)))
			if recorder.Code != tt.status {
				t.Errorf("status = %d, want %d", recorder.Code, tt.status)
			}
		})
	}
}

func TestShortenHandlerServiceError(t *testing.T) {
	svc := &stubURLService{create: func(context.Context, string, *time.Time) (string, error) {
		return "", errors.New("database unavailable")
	}}
	recorder := httptest.NewRecorder()
	NewURLHandler(svc).ShortenHandler(recorder, httptest.NewRequest(http.MethodPost, "/urls", strings.NewReader(`{"url":"https://example.com"}`)))
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

func TestHealthHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	HealthHandler(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", recorder.Code)
	}
}

func TestExtractHandler(t *testing.T) {
	for _, tt := range []struct {
		name   string
		err    error
		status int
	}{
		{"success", nil, http.StatusFound},
		{"expired", fmt.Errorf("resolve: %w", utils.ErrURLExpired), http.StatusGone},
		{"missing", fmt.Errorf("resolve: %w", pgx.ErrNoRows), http.StatusNotFound},
		{"service error", errors.New("unavailable"), http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			svc := &stubURLService{resolve: func(ctx context.Context, code string) (string, error) {
				called = true
				if code != "abc12345" {
					t.Errorf("code = %q", code)
				}
				return "https://example.com", tt.err
			}}
			req := httptest.NewRequest(http.MethodGet, "/abc12345", nil)
			req.SetPathValue("code", "abc12345")
			recorder := httptest.NewRecorder()
			NewURLHandler(svc).ExtractHandler(recorder, req)
			if !called {
				t.Error("ResolveURL was not called")
			}
			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.status)
			}
			if tt.err == nil && recorder.Header().Get("Location") != "https://example.com" {
				t.Errorf("Location = %q", recorder.Header().Get("Location"))
			}
		})
	}
}

func TestStatsAndDeleteHandlers(t *testing.T) {
	for _, operation := range []string{"stats", "delete"} {
		for _, tt := range []struct {
			name   string
			err    error
			status int
		}{
			{"success", nil, http.StatusOK},
			{"missing", fmt.Errorf("repository: %w", pgx.ErrNoRows), http.StatusNotFound},
			{"service error", errors.New("unavailable"), http.StatusInternalServerError},
		} {
			t.Run(operation+"/"+tt.name, func(t *testing.T) {
				called := false
				checkCode := func(code string) {
					called = true
					if code != "abc12345" {
						t.Errorf("code = %q", code)
					}
				}
				svc := &stubURLService{
					stats:  func(ctx context.Context, code string) (int64, error) { checkCode(code); return 2, tt.err },
					delete: func(ctx context.Context, code string) error { checkCode(code); return tt.err },
				}
				method := http.MethodGet
				if operation == "delete" {
					method = http.MethodDelete
				}
				req := httptest.NewRequest(method, "/urls/abc12345", nil)
				req.SetPathValue("code", "abc12345")
				recorder := httptest.NewRecorder()
				handler := NewURLHandler(svc)
				wantStatus := tt.status
				if operation == "stats" {
					handler.StatsHandler(recorder, req)
				} else {
					handler.DeleteHandler(recorder, req)
					if tt.err == nil {
						wantStatus = http.StatusNoContent
					}
				}
				if !called {
					t.Error("service was not called")
				}
				if recorder.Code != wantStatus {
					t.Fatalf("status = %d, want %d", recorder.Code, wantStatus)
				}
				if operation == "stats" && tt.err == nil {
					var response struct {
						Code  string `json:"short_code"`
						Count int64  `json:"click_count"`
					}
					if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
						t.Fatal(err)
					}
					if response.Code != "abc12345" || response.Count != 2 {
						t.Errorf("stats = %+v", response)
					}
				}
			})
		}
	}
}
