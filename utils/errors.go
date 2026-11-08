package utils

import "errors"

var (
	ErrURLExpired = errors.New("URL has expired")
	ErrURLNotFound = errors.New("URL not found")
	ErrCodeExists = errors.New("short code already exists")
)
