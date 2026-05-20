package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ningen/embed"
	"ningen/internal/handlers"
	"ningen/internal/llm"
	"ningen/internal/rag"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func run(ctx context.Context) error {
	dbURL := mustEnv("DB_URL")
	port := envOr("PORT", "8080")

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return err
	}
	log.Println("Connected to Postgres")

	providers, err := llm.Build()
	if err != nil {
		return err
	}
	log.Printf("LLM providers available: %v", keys(providers))

	embedderURL := envOr("EMBEDDER_URL", "http://embedder:8000")
	log.Printf("Connecting to embedder sidecar at %s...", embedderURL)

	deps := &handlers.Deps{
		LLM:    providers,
		Vector: rag.New(pool),
		Embed:  embed.NewSidecarEmbedder(embedderURL),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /recommend", handlers.RecommendHandler(deps))
	mux.HandleFunc("POST /generate-review", handlers.GenerateReviewHandler(deps))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /docs", handlers.DocsHandler())
	mux.HandleFunc("GET /openapi.yaml", handlers.OpenAPIHandler())

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      requestLogger(cors(mux)),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("API server listening on :%s", port)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown error: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func cors(next http.Handler) http.Handler {
	allowed := map[string]bool{
		"https://ningen.vercel.app": true,
		"http://localhost:3000":     true,
		"http://localhost:8080":     true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %q is not set", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func keys(r llm.Registry) []string {
	ks := make([]string, 0, len(r))
	for k := range r {
		ks = append(ks, k)
	}
	return ks
}
