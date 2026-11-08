package domain

import "time"

type URL struct {
	OriginalURL string
	ExpiresAt   *time.Time
}
