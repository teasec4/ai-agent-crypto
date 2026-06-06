package main

import (
	"ai-agent/internal/config"
	"ai-agent/internal/handler"
	"ai-agent/internal/harness"
	"ai-agent/internal/session"
	"log"
	"net/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	h := harness.New(cfg)
	sessionStore := session.NewStore()
	agentHandler := handler.NewAgentHandler(h, sessionStore)

	mux := http.NewServeMux()
	agentHandler.RegisterRoutes(mux)

	srv := http.Server{
		Addr:    ":8080",
		Handler: withCORS(mux),
	}

	log.Println("API server listening on :8080")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
