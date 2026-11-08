package main

import (
	"context"
	"errors"
	"leetx/url/shortener/cache"
	"leetx/url/shortener/database"
	"leetx/url/shortener/events"
	"leetx/url/shortener/handlers"
	"leetx/url/shortener/repository"
	"leetx/url/shortener/service"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	workerCtx, cancelWorker := context.WithCancel(context.Background())

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file: ", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL env variable is not set")
	}
	db, err := database.Connect(signalCtx, databaseURL)
	if err != nil {
		log.Fatal("Failed to connect to PostgreSQL: ", err)
	}
	defer db.Close()

	repo := repository.NewURLRepository(db)
	redisClient, err := cache.NewRedisClient()
	if err != nil {
		log.Fatal("Failed to connect to redis client: ", err)
	}
	defer redisClient.Close()

	urlCache := cache.NewURLCache(redisClient)

	publisher := events.NewMemoryClickPublisher(100)

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()
		publisher.StartWorker(workerCtx, repo)
	}()

	urlService := service.NewURLService(repo, urlCache, publisher)
	handler := handlers.NewURLHandler(urlService)

	http.HandleFunc("POST /urls", handler.ShortenHandler)
	http.HandleFunc("GET /{code}", handler.ExtractHandler)
	http.HandleFunc("DELETE /urls/{code}", handler.DeleteHandler)
	http.HandleFunc("GET /urls/{code}/stats", handler.StatsHandler)

	log.Println("Server is running on http://localhost:8080")
	server := &http.Server{
		Addr:    ":8080",
		Handler: http.DefaultServeMux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-signalCtx.Done()

	log.Println("Shutdown signal received")

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancelShutdown()
	
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown failed: %v", err)
	}

	// Stop worker after HTTP shutdown
	cancelWorker()

	// Wait for worker to drain and flush clicks
	wg.Wait()

	log.Println("Application shutdown complete")
}
