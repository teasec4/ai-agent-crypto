package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-agent/internal/config"
	"ai-agent/internal/handler"
	"ai-agent/internal/harness"
	"ai-agent/internal/session"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("error loading config", "error", err)
		os.Exit(1)
	}

	h := harness.New(cfg)
	sessionStore, err := session.NewStoreWithStorage(session.NewJSONStorage(cfg.SessionStoragePath))
	if err != nil {
		slog.Error("error loading sessions", "error", err)
		os.Exit(1)
	}
	agentHandler := handler.NewAgentHandler(h, sessionStore)

	if cfg.SessionTTLSeconds > 0 {
		stopCleanup := sessionStore.StartCleanup(10*time.Minute, time.Duration(cfg.SessionTTLSeconds)*time.Second)
		defer stopCleanup()
	}

	mux := http.NewServeMux()
	agentHandler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:           ":8080",
		Handler:        withRequestLogging(withRecovery(withCORS(mux))),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   5 * time.Minute, // LLM calls can be slow
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	}()

	slog.Info("API server starting", "addr", ":8080")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

// withRequestLogging logs every incoming HTTP request with method, path, status, and duration.
func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wr := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wr, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wr.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush delegates to the underlying ResponseWriter if it supports flushing (SSE).
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("http handler panic recovered", "panic", rec, "path", r.URL.Path)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
