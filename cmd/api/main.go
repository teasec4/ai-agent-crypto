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
		Handler: mux,
	}

	log.Println("API server listening on :8080")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
