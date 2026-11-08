package service

import "context"

type ClickPublisher interface {
	PublishClick(ctx context.Context, shortCode string) error
}