package service

import (
	"context"
	"errors"
	"fmt"
	"leetx/url/shortener/domain"
	"leetx/url/shortener/utils"
	"log"
	"time"
)

type URLRepository interface {
	GetByShortCode(ctx context.Context, shortCode string) (*domain.URL, error)
	GetClickCount(ctx context.Context, shortCode string) (int64, error)
	IncrementClickCount(ctx context.Context, shortCode string) error
	Delete(ctx context.Context, shortCode string) error
	Create(ctx context.Context, shortCode, originalURL string, expiresAt *time.Time) error
}

type URLService struct {
	repo      URLRepository
	cache     URLCache
	publisher ClickPublisher
}

type URLCache interface {
	Get(ctx context.Context, shortCode string) (string, error)
	Set(ctx context.Context, shortCode, originalURL string, ttl time.Duration) error
	Delete(ctx context.Context, shortCode string) error
}

func NewURLService(repo URLRepository, cache URLCache, publisher ClickPublisher) *URLService {
	return &URLService{
		repo:      repo,
		cache:     cache,
		publisher: publisher,
	}
}

func (s *URLService) GetClickCount(ctx context.Context, shortCode string) (int64, error) {
	clickCount, err := s.repo.GetClickCount(ctx, shortCode)
	if err != nil {
		return 0, fmt.Errorf("failed to get click count: %w", err)
	}

	return clickCount, nil
}

func (s *URLService) ResolveURL(ctx context.Context, shortCode string) (string, error) {
	originalURL, err := s.cache.Get(ctx, shortCode)
	if err == nil {
		// Cache hit
		err = s.publisher.PublishClick(ctx, shortCode)
		if err != nil {
			log.Printf("failed to publish click: %v", err)
		}
		return originalURL, nil
	}

	// Cache miss
	url, err := s.repo.GetByShortCode(ctx, shortCode)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve original URL: %w", err)
	}

	const defaultCacheTTL = 5 * time.Minute
	ttl := defaultCacheTTL

	if url.ExpiresAt != nil {
		remaining := time.Until(*url.ExpiresAt)
		if remaining <= 0 {
			return "", utils.ErrURLExpired
		}

		if remaining < ttl {
			ttl = remaining
		}
	}

	err = s.publisher.PublishClick(ctx, shortCode)
	if err != nil {
		log.Printf("failed to publish click: %v", err)
	}

	err = s.cache.Set(ctx, shortCode, url.OriginalURL, ttl)
	if err != nil {
		log.Printf("failed to populate Redis cache: %v", err)
	}

	return url.OriginalURL, nil
}

func (s *URLService) Delete(ctx context.Context, shortCode string) error {
	err := s.repo.Delete(ctx, shortCode)
	if err != nil {
		return fmt.Errorf("failed to delete short code: %w", err)
	}

	err = s.cache.Delete(ctx, shortCode)
	if err != nil {
		return fmt.Errorf("failed to delete redis cache: %w", err)
	}

	return nil
}

func (s *URLService) CreateURL(ctx context.Context, originalURL string, expiresAt *time.Time) (string, error) {
	const maxRetries = 5

	for attempt := 0; attempt < maxRetries; attempt++ {
		shortCode, err := utils.GenerateShortCode(8)
		if err != nil {
			return "", fmt.Errorf("failed to generate short code: %w", err)
		}

		err = s.repo.Create(ctx, shortCode, originalURL, expiresAt)
		if err == nil {
			return shortCode, nil
		}

		if errors.Is(err, utils.ErrCodeExists) {
			continue
		}

		return "", fmt.Errorf("failed to create short URL: %w", err)
	}

	return "", errors.New("failed to generate unique short code after 5 attempts")

}
