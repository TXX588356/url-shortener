package events

import (
	"context"
	"errors"
	"log"
	"time"
)

type ClickCounter interface {
	IncrementClickCountBy(ctx context.Context, shortCode string, count int64) error
}

type MemoryClickPublisher struct {
	clicks chan string
}

func NewMemoryClickPublisher(bufferSize int) *MemoryClickPublisher {
	return &MemoryClickPublisher{
		clicks: make(chan string, bufferSize),
	}
}

func (p *MemoryClickPublisher) PublishClick(ctx context.Context, shortCode string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Non-blocking send
	select {
	case p.clicks <- shortCode:
		return nil
	default:
		// if the sent cannot proceed, log that error and still do redirect; prevent
		// full event buffer from blocking the request, at the cost of potentially
		// dropping click events
		return errors.New("click event buffer is full")
	}
}

func (p *MemoryClickPublisher) StartWorker(ctx context.Context, counter ClickCounter) {
	clickCounts := make(map[string]int64)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case shortCode, ok := <-p.clicks:
			if !ok {
				flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				flushClicks(flushCtx, counter, clickCounts)
				cancel()
				return
			}
			clickCounts[shortCode]++

		case <-ticker.C:
			flushClicks(ctx, counter, clickCounts)

		case <-ctx.Done():
		drainLoop:
			for {
				select {
				case shortCode, ok := <-p.clicks:
					if !ok {
						break drainLoop
					}
					clickCounts[shortCode]++
				default:
					break drainLoop
				}
			}

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			flushClicks(shutdownCtx, counter, clickCounts)
			cancel()

			return

		}
	}
}

func flushClicks(ctx context.Context, counter ClickCounter, clickCounts map[string]int64) {
	for shortCode, click := range clickCounts {
		err := counter.IncrementClickCountBy(ctx, shortCode, click)
		if err != nil {
			log.Printf("worker failed to increment click count: %v", err)
		} else {
			delete(clickCounts, shortCode)
		}
	}
}
