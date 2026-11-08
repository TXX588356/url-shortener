package service

import (
	"context"
	"errors"
	"leetx/url/shortener/domain"
	"leetx/url/shortener/utils"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeURLRepository struct {
	urls         map[string]*domain.URL
	clicks       map[string]int64
	createErrors []error
	createCalls  int
	incrementErr error
}

var _ URLRepository = (*fakeURLRepository)(nil)

func newFakeURLRepository() *fakeURLRepository {
	return &fakeURLRepository{urls: make(map[string]*domain.URL), clicks: make(map[string]int64)}
}
func (r *fakeURLRepository) Create(ctx context.Context, code, originalURL string, expiresAt *time.Time) error {
	r.createCalls++
	if r.createCalls <= len(r.createErrors) && r.createErrors[r.createCalls-1] != nil {
		return r.createErrors[r.createCalls-1]
	}
	r.urls[code] = &domain.URL{OriginalURL: originalURL, ExpiresAt: expiresAt}
	return nil
}
func (r *fakeURLRepository) GetByShortCode(ctx context.Context, code string) (*domain.URL, error) {
	u, ok := r.urls[code]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return u, nil
}
func (r *fakeURLRepository) GetClickCount(ctx context.Context, code string) (int64, error) {
	if _, ok := r.urls[code]; !ok {
		return 0, pgx.ErrNoRows
	}
	return r.clicks[code], nil
}
func (r *fakeURLRepository) IncrementClickCount(ctx context.Context, code string) error {
	if r.incrementErr != nil {
		return r.incrementErr
	}
	if _, ok := r.urls[code]; !ok {
		return pgx.ErrNoRows
	}
	r.clicks[code]++
	return nil
}
func (r *fakeURLRepository) Delete(ctx context.Context, code string) error {
	if _, ok := r.urls[code]; !ok {
		return pgx.ErrNoRows
	}
	delete(r.urls, code)
	delete(r.clicks, code)
	return nil
}

func TestURLServiceLifecycle(t *testing.T) {
	expiration := time.Now().Add(time.Hour)
	for _, tt := range []struct {
		name      string
		expiresAt *time.Time
	}{
		{"without expiration", nil}, {"with expiration", &expiration},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeURLRepository()
			svc := NewURLService(repo)
			ctx := context.Background()
			code, err := svc.CreateURL(ctx, "https://example.com", tt.expiresAt)
			if err != nil {
				t.Fatal(err)
			}
			if len(code) != 8 {
				t.Fatalf("code length = %d, want 8", len(code))
			}
			stored := repo.urls[code]
			if stored == nil || stored.OriginalURL != "https://example.com" {
				t.Fatalf("stored URL = %+v", stored)
			}
			if tt.expiresAt == nil {
				if stored.ExpiresAt != nil {
					t.Errorf("expiration = %v, want nil", stored.ExpiresAt)
				}
			} else if stored.ExpiresAt == nil || !stored.ExpiresAt.Equal(*tt.expiresAt) {
				t.Errorf("expiration = %v, want %v", stored.ExpiresAt, tt.expiresAt)
			}
			for want := int64(0); want <= 2; want++ {
				count, err := svc.GetClickCount(ctx, code)
				if err != nil || count != want {
					t.Fatalf("count = (%d, %v), want (%d, nil)", count, err, want)
				}
				if want < 2 {
					original, err := svc.ResolveURL(ctx, code)
					if err != nil || original != "https://example.com" {
						t.Fatalf("resolve = (%q, %v)", original, err)
					}
				}
			}
			if err := svc.Delete(ctx, code); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.ResolveURL(ctx, code); !errors.Is(err, pgx.ErrNoRows) {
				t.Errorf("resolve after delete = %v", err)
			}
			if _, err := svc.GetClickCount(ctx, code); !errors.Is(err, pgx.ErrNoRows) {
				t.Errorf("stats after delete = %v", err)
			}
			if err := svc.Delete(ctx, code); !errors.Is(err, pgx.ErrNoRows) {
				t.Errorf("repeat delete = %v", err)
			}
		})
	}
}

func TestResolveURLFailures(t *testing.T) {
	expired := time.Now().Add(-time.Hour)
	unavailable := errors.New("increment unavailable")
	for _, tt := range []struct {
		name                  string
		url                   *domain.URL
		incrementErr, wantErr error
	}{
		{name: "missing", wantErr: pgx.ErrNoRows},
		{name: "expired", url: &domain.URL{OriginalURL: "https://example.com", ExpiresAt: &expired}, wantErr: utils.ErrURLExpired},
		{name: "increment error", url: &domain.URL{OriginalURL: "https://example.com"}, incrementErr: unavailable, wantErr: unavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeURLRepository()
			if tt.url != nil {
				repo.urls["abc12345"] = tt.url
			}
			repo.incrementErr = tt.incrementErr
			original, err := NewURLService(repo).ResolveURL(context.Background(), "abc12345")
			if original != "" || !errors.Is(err, tt.wantErr) {
				t.Errorf("resolve = (%q, %v), want empty URL and %v", original, err, tt.wantErr)
			}
			if repo.clicks["abc12345"] != 0 {
				t.Error("failed resolve incremented click count")
			}
		})
	}
}

func TestCreateURLRetries(t *testing.T) {
	conflict := &pgconn.PgError{Code: "23505"}
	unavailable := errors.New("database unavailable")
	for _, tt := range []struct {
		name      string
		errs      []error
		calls     int
		wantErr   error
		exhausted bool
	}{
		{name: "retry conflict", errs: []error{conflict}, calls: 2},
		{name: "stop on other error", errs: []error{unavailable}, calls: 1, wantErr: unavailable},
		{name: "retry limit", errs: []error{conflict, conflict, conflict, conflict, conflict}, calls: 5, exhausted: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeURLRepository()
			repo.createErrors = tt.errs
			code, err := NewURLService(repo).CreateURL(context.Background(), "https://example.com", nil)
			if repo.createCalls != tt.calls {
				t.Errorf("Create calls = %d, want %d", repo.createCalls, tt.calls)
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
			} else if tt.exhausted {
				if err == nil {
					t.Error("expected retry exhaustion error")
				}
			} else if err != nil || len(code) != 8 || repo.urls[code] == nil {
				t.Errorf("create = (%q, %v), want stored short code", code, err)
			}
			if (tt.wantErr != nil || tt.exhausted) && code != "" {
				t.Errorf("code on failure = %q, want empty", code)
			}
		})
	}
}
